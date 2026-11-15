// Package e2e drives the real gote binary against throwaway state dirs.
package e2e

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

const ldPkg = "github.com/banorton/gote/internal/cli.Version="

var binDir string

func TestMain(m *testing.M) {
	code := func() int {
		var err error
		binDir, err = os.MkdirTemp("/tmp", "gb")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(binDir)
		for _, v := range []string{"vA", "vB"} {
			cmd := exec.Command("go", "build", "-ldflags", "-X "+ldPkg+v, "-o", filepath.Join(binDir, "gote-"+v), "../cmd/gote")
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Fprintf(os.Stderr, "build %s: %v\n%s", v, err, out)
				return 1
			}
		}
		return m.Run()
	}()
	os.Exit(code)
}

type env struct {
	t                  *testing.T
	root, cfg, st, dir string
	extra              []string
}

// newEnv makes a short-pathed sandbox (unix socket paths are limited to ~104 bytes).
func newEnv(t *testing.T) *env {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "g")
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, root: root, cfg: filepath.Join(root, "c"), st: filepath.Join(root, "s"), dir: filepath.Join(root, "n")}
	os.MkdirAll(filepath.Join(e.cfg, "gote"), 0o755)
	os.MkdirAll(e.dir, 0o755)
	e.writeConfig("")
	t.Cleanup(func() {
		e.stopServer()
		os.RemoveAll(root)
	})
	return e
}

func (e *env) writeConfig(extra string) {
	e.t.Helper()
	body := "notes_dir: " + e.dir + "\n" + extra
	if err := os.WriteFile(filepath.Join(e.cfg, "gote", "config.yaml"), []byte(body), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) write(name, body string) {
	e.t.Helper()
	p := filepath.Join(e.dir, name)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

type result struct {
	out, err string
	code     int
}

func bin(v string) string { return filepath.Join(binDir, "gote-"+v) }

func (e *env) cmd(version string, args ...string) *exec.Cmd {
	c := exec.Command(bin(version), args...)
	c.Env = append(os.Environ(), "XDG_CONFIG_HOME="+e.cfg, "XDG_STATE_HOME="+e.st, "EDITOR=true",
		"HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "http_proxy=", "https_proxy=", "all_proxy=")
	c.Env = append(c.Env, e.extra...)
	return c
}

func (e *env) runV(version string, args ...string) result {
	e.t.Helper()
	c := e.cmd(version, args...)
	var so, se strings.Builder
	c.Stdout, c.Stderr = &so, &se
	err := c.Run()
	r := result{out: so.String(), err: se.String()}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		r.code = ee.ExitCode()
	} else if err != nil {
		e.t.Fatalf("run %v: %v", args, err)
	}
	return r
}

func (e *env) run(args ...string) result { e.t.Helper(); return e.runV("vA", args...) }

// ok runs and fails on non-zero exit or any stderr output (incl. fallback warnings).
func (e *env) ok(args ...string) string {
	e.t.Helper()
	r := e.run(args...)
	if r.code != 0 || r.err != "" {
		e.t.Fatalf("gote %v: code %d\nstdout: %s\nstderr: %s", args, r.code, r.out, r.err)
	}
	return r.out
}

type srvStatus struct {
	Running bool
	Version string
	PID     int
	Socket  string
}

func (e *env) serverStatus() srvStatus {
	e.t.Helper()
	var s srvStatus
	if err := json.Unmarshal([]byte(e.ok("server", "status", "--json")), &s); err != nil {
		e.t.Fatal(err)
	}
	return s
}

func (e *env) stopServer() {
	var st srvStatus
	var out strings.Builder
	c := e.cmd("vA", "server", "status", "--json")
	c.Stdout = &out
	if c.Run() == nil && json.Unmarshal([]byte(out.String()), &st) == nil && st.PID > 0 {
		e.cmd("vA", "server", "stop").Run()
		// a server that ignores stop must not outlive the test
		time.Sleep(100 * time.Millisecond)
		if alive(st.PID) {
			syscall.Kill(st.PID, syscall.SIGKILL)
		}
	}
}

func waitDead(t *testing.T, pid int) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if !alive(pid) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("pid %d still alive", pid)
}

// alive reports whether pid is a live process. A killed server is orphaned
// (spawned detached), and in containers or CI the reaper may be slow or absent,
// so a zombie counts as dead.
func alive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		i := strings.LastIndexByte(string(b), ')')
		return i < 0 || i+2 >= len(b) || b[i+2] != 'Z'
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", fmt.Sprint(pid)).Output()
	if err != nil {
		return true
	}
	return !strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

// budget scales a timing limit on shared CI runners, which are several times
// slower than a dev machine; the scaled limit still catches the regressions
// these budgets exist for (e.g. quadratic indexing took >8 s locally).
func budget(d time.Duration) time.Duration {
	if os.Getenv("CI") != "" {
		return 3 * d
	}
	return d
}
