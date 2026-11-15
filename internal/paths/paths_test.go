package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if got := ConfigDir(); got != "/cfg/gote" {
		t.Fatal(got)
	}
	sd, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(sd)
	if err != nil || st.Mode().Perm() != 0o700 {
		t.Fatalf("%v %v", err, st)
	}
	db, _ := DBPath()
	sock, _ := SocketPath()
	if db != filepath.Join(sd, "gote.db") || sock != filepath.Join(sd, "gote.sock") {
		t.Fatal(db, sock)
	}
}

func TestDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got := ConfigDir(); got != "/home/u/.config/gote" {
		t.Fatal(got)
	}
	t.Setenv("XDG_STATE_HOME", "")
	if got := stateBase(); got != "/home/u/.local/state/gote" {
		t.Fatal(got)
	}
}
