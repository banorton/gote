package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFallsBackToLocalWhenSpawnFails(t *testing.T) {
	notes := env(t)
	p := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "gote", "config.yaml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("notes_dir: "+notes+"\n"), 0o644)
	os.WriteFile(filepath.Join(notes, "a.md"), []byte("x"), 0o644)

	orig := spawnServer
	t.Cleanup(func() { spawnServer = orig })
	spawnServer = func(string) func() error { return func() error { return errors.New("no fork") } }

	code, out, errs := runWith(std, "status", "--json")
	if code != 0 || !strings.Contains(out, `"notes": 1`) {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	if want := "gote: server unavailable (no fork); running in-process\n"; errs != want {
		t.Fatalf("stderr %q want %q", errs, want)
	}

	code, _, errs = runWith(std, "status", "--json", "--no-server")
	if code != 0 || errs != "" {
		t.Fatalf("--no-server: %d %q", code, errs)
	}
}

func TestServerFalseConfigUsesLocalSilently(t *testing.T) {
	notes := env(t)
	p := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "gote", "config.yaml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("notes_dir: "+notes+"\nserver: false\n"), 0o644)
	orig := spawnServer
	t.Cleanup(func() { spawnServer = orig })
	spawnServer = func(string) func() error { return func() error { t.Error("spawned"); return nil } }
	if code, _, errs := runWith(std, "status"); code != 0 || errs != "" {
		t.Fatalf("%d %q", code, errs)
	}
}
