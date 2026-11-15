package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gote")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func run(t *testing.T, bin, cfg, state string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+cfg, "XDG_STATE_HOME="+state)
	var so, se strings.Builder
	cmd.Stdout, cmd.Stderr = &so, &se
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return so.String(), se.String(), code
}

func TestBinaryE2E(t *testing.T) {
	bin := buildBinary(t)
	root := t.TempDir()
	cfg, state, notes := filepath.Join(root, "cfg"), filepath.Join(root, "state"), filepath.Join(root, "notes")
	os.MkdirAll(notes, 0o755)

	_, stderr, code := run(t, bin, cfg, state, "status")
	if code != 1 || !strings.Contains(stderr, filepath.Join(cfg, "gote", "config.yaml")) {
		t.Fatalf("missing config: %d %q", code, stderr)
	}

	os.MkdirAll(filepath.Join(cfg, "gote"), 0o755)
	os.WriteFile(filepath.Join(cfg, "gote", "config.yaml"), []byte("notes_dir: "+notes+"\n"), 0o644)
	os.WriteFile(filepath.Join(notes, "a.md"), []byte("[[b]] [[missing]]"), 0o644)
	os.WriteFile(filepath.Join(notes, "b.md"), []byte("# b"), 0o644)
	os.MkdirAll(filepath.Join(notes, ".trash"), 0o755)
	os.WriteFile(filepath.Join(notes, ".trash", "c.md"), []byte("x"), 0o644)

	stdout, stderr, code := run(t, bin, cfg, state, "status", "--json")
	var st struct{ Notes, Links, Unresolved int }
	if code != 0 || json.Unmarshal([]byte(stdout), &st) != nil || st.Notes != 2 || st.Links != 2 || st.Unresolved != 1 {
		t.Fatalf("status: %d %q %q", code, stdout, stderr)
	}

	stdout, stderr, code = run(t, bin, cfg, state, "reindex", "--json")
	var rs struct{ Added int }
	if code != 0 || json.Unmarshal([]byte(stdout), &rs) != nil || rs.Added != 2 {
		t.Fatalf("reindex: %d %q %q", code, stdout, stderr)
	}

	if _, _, code = run(t, bin, cfg, state, "nonsense"); code != 2 {
		t.Fatalf("unknown command exit %d", code)
	}
	if stdout, _, code = run(t, bin, cfg, state, "--version"); code != 0 || !strings.HasPrefix(stdout, "gote ") {
		t.Fatalf("version: %d %q", code, stdout)
	}
}
