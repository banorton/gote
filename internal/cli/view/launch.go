package view

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"encoding/json"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/paths"
	"github.com/banorton/gote/internal/store"
	"github.com/banorton/gote/internal/web"
)

// Opener launches the system browser; tests replace it.
var Opener = func(url string) error {
	bin := "xdg-open"
	if runtime.GOOS == "darwin" {
		bin = "open"
	}
	cmd := exec.Command(bin, url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", bin, err)
	}
	go cmd.Wait()
	return nil
}

// WaitContext is the context a serverless view blocks on; tests replace it.
var WaitContext = func() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func Route(id, kind string) string {
	if kind == note.KindDiagram {
		return "/d/" + escape(id)
	}
	return "/n/" + escape(id)
}

// Launch opens a route in the browser. With a running server it returns once
// the browser is opened; otherwise it starts a web listener of its own and
// blocks until interrupted.
func Launch(c *cli.Ctx, id, route string) error {
	if base, token, ok := attach(c); ok {
		return announce(c, id, base+route+"?t="+token)
	}
	return serve(c, id, route)
}

type webInfo struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// attach finds the running server's web listener, verifying it is alive: a
// stale web.json from a crashed server must not be trusted.
func attach(c *cli.Ctx) (base, token string, ok bool) {
	if c.NoServer || c.Cfg == nil || !c.Cfg.Server {
		return "", "", false
	}
	dir, err := paths.StateDir()
	if err != nil {
		return "", "", false
	}
	b, err := os.ReadFile(filepath.Join(dir, "web.json"))
	if err != nil {
		return "", "", false
	}
	var w webInfo
	if json.Unmarshal(b, &w) != nil || w.Port == 0 || w.Token == "" {
		return "", "", false
	}
	base = "http://127.0.0.1:" + strconv.Itoa(w.Port)
	hc := &http.Client{Timeout: time.Second}
	resp, err := hc.Get(base + "/healthz?t=" + w.Token)
	if err != nil {
		return "", "", false
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", false
	}
	return base, w.Token, true
}

func announce(c *cli.Ctx, id, url string) error {
	if c.JSON {
		if err := c.PrintJSON(map[string]string{"id": id, "url": url}); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(c.Out, url)
	}
	if err := Opener(url); err != nil {
		fmt.Fprintf(c.Err, "gote: could not open a browser (%v); open the URL above manually\n", err)
	}
	return nil
}

func serve(c *cli.Ctx, id, route string) error {
	dbPath, err := paths.DBPath()
	if err != nil {
		return err
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	env := &ops.Env{Cfg: c.Cfg, Store: st, Index: index.New(st, c.Cfg.NotesDir), Now: time.Now}
	ctx, stop := WaitContext()
	defer stop()
	ws, err := web.Start(ctx, env, web.NoStateFile())
	if err != nil {
		return err
	}
	defer ws.Close()
	if err := announce(c, id, ws.URL(route)); err != nil {
		return err
	}
	if !c.JSON {
		fmt.Fprintln(c.Err, "serving until interrupted (ctrl-c to stop)")
	}
	<-ctx.Done()
	return nil
}
