package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/banorton/gote/internal/ops"
)

type remote struct{ hc *http.Client }

func unixHTTP(socket string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
			DisableKeepAlives: true,
		},
	}
}

// Remote talks to a gote server over its Unix socket.
func Remote(socketPath string) Client { return &remote{unixHTTP(socketPath, 0)} }

func (r *remote) Call(ctx context.Context, name string, req, resp any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return ops.Errorf(ops.CodeInvalid, "%s: encode request: %v", name, err)
	}
	hr, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://gote/v1/op/"+name, bytes.NewReader(body))
	if err != nil {
		return ops.Errorf(ops.CodeInvalid, "%s: %v", name, err)
	}
	hr.Header.Set("Content-Type", "application/json")
	res, err := r.hc.Do(hr)
	if err != nil {
		return ops.Errorf(ops.CodeInternal, "%s: server request: %v", name, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return ops.Errorf(ops.CodeInternal, "%s: read response: %v", name, err)
	}
	if res.StatusCode != http.StatusOK {
		var env struct {
			Error *ops.Error `json:"error"`
		}
		if json.Unmarshal(data, &env) == nil && env.Error != nil {
			return env.Error
		}
		return ops.Errorf(ops.CodeInternal, "%s: server returned %s", name, res.Status)
	}
	if resp == nil {
		return nil
	}
	if err := json.Unmarshal(data, resp); err != nil {
		return ops.Errorf(ops.CodeInternal, "%s: decode response: %v", name, err)
	}
	return nil
}

type ServerInfo struct {
	Version string `json:"version"`
	PID     int    `json:"pid"`
	Started string `json:"started"`
}

// Probe returns the version info of the server on socket, or an error if none answers.
func Probe(socket string) (ServerInfo, error) {
	var info ServerInfo
	res, err := unixHTTP(socket, time.Second).Get("http://gote/v1/version")
	if err != nil {
		return info, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return info, fmt.Errorf("version: %s", res.Status)
	}
	return info, json.NewDecoder(res.Body).Decode(&info)
}

func Shutdown(socket string) error {
	res, err := unixHTTP(socket, 2*time.Second).Post("http://gote/v1/shutdown", "application/json", nil)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("shutdown: %s", res.Status)
	}
	return nil
}

// WaitGone waits for the socket file to disappear.
func WaitGone(socket string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for {
		if _, err := os.Stat(socket); os.IsNotExist(err) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("server did not stop within %s", d)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type ConnectOpts struct {
	Socket, Version string
	// Spawn starts a detached server; it need not wait for readiness.
	Spawn func() error
	// Zero values mean the defaults (2s stop, 3s start).
	StopWait, StartWait time.Duration
}

// Connect returns a Remote client for a server whose version matches, replacing
// a mismatched server or spawning one if none runs.
func Connect(o ConnectOpts) (Client, error) {
	if o.StopWait == 0 {
		o.StopWait = 2 * time.Second
	}
	if o.StartWait == 0 {
		o.StartWait = 3 * time.Second
	}
	if info, err := Probe(o.Socket); err == nil {
		if info.Version == o.Version {
			return Remote(o.Socket), nil
		}
		if err := Shutdown(o.Socket); err != nil {
			return nil, fmt.Errorf("stop server %s: %w", info.Version, err)
		}
		if err := WaitGone(o.Socket, o.StopWait); err != nil {
			return nil, err
		}
	}
	if err := o.Spawn(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(o.StartWait)
	for {
		info, err := Probe(o.Socket)
		if err == nil {
			if info.Version != o.Version {
				return nil, fmt.Errorf("spawned server reports version %s, want %s", info.Version, o.Version)
			}
			return Remote(o.Socket), nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("server did not start within %s", o.StartWait)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// SpawnServer starts `<this binary> serve` detached, with stdio on logPath.
func SpawnServer(logPath string) func() error {
	return func() error {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		// A test binary re-run with "serve" would execute the whole suite again.
		if strings.HasSuffix(filepath.Base(exe), ".test") {
			return errors.New("refusing to spawn a server from a test binary")
		}
		log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer log.Close()
		cmd := exec.Command(exe, "serve")
		cmd.Stdout, cmd.Stderr = log, log
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return err
		}
		return cmd.Process.Release()
	}
}
