package server_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryServerE2E(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "ge")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	bin := filepath.Join(root, "gote")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/gote")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cfg, state, notes := filepath.Join(root, "c"), filepath.Join(root, "s"), filepath.Join(root, "n")
	os.MkdirAll(filepath.Join(cfg, "gote"), 0o755)
	os.MkdirAll(notes, 0o755)
	os.WriteFile(filepath.Join(cfg, "gote", "config.yaml"), []byte("notes_dir: "+notes+"\n"), 0o644)
	os.WriteFile(filepath.Join(notes, "a.md"), []byte("x"), 0o644)

	run := func(args ...string) (string, string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+cfg, "XDG_STATE_HOME="+state)
		var so, se strings.Builder
		cmd.Stdout, cmd.Stderr = &so, &se
		err := cmd.Run()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return so.String(), se.String(), ee.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		return so.String(), se.String(), 0
	}
	t.Cleanup(func() { run("server", "stop") })

	out, _, code := run("server", "status", "--json")
	if code != 0 || !strings.Contains(out, `"running": false`) {
		t.Fatalf("status before: %d %s", code, out)
	}

	out, errs, code := run("status", "--json")
	if code != 0 || !strings.Contains(out, `"notes": 1`) || errs != "" {
		t.Fatalf("status: %d %s %s", code, out, errs)
	}

	out, _, code = run("server", "status", "--json")
	var st struct {
		Running bool
		Version string
		PID     int
		Socket  string
	}
	if code != 0 || json.Unmarshal([]byte(out), &st) != nil || !st.Running || st.PID == 0 || st.Version == "" {
		t.Fatalf("status after: %d %s", code, out)
	}
	if fi, err := os.Stat(st.Socket); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("socket: %v %v", err, fi)
	}

	if _, _, code = run("server", "stop"); code != 0 {
		t.Fatalf("stop: %d", code)
	}
	out, _, _ = run("server", "status", "--json")
	if !strings.Contains(out, `"running": false`) {
		t.Fatalf("after stop: %s", out)
	}
	if _, err := os.Stat(filepath.Join(state, "gote", "gote.sock")); !os.IsNotExist(err) {
		t.Fatalf("socket remains: %v", err)
	}
}
