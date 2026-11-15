package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func noteCount(t *testing.T, e *env, args ...string) int {
	t.Helper()
	var s struct{ Notes int }
	if err := json.Unmarshal([]byte(e.ok(append(args, "status", "--json")...)), &s); err != nil {
		t.Fatal(err)
	}
	return s.Notes
}

func TestCrashRecovery(t *testing.T) {
	e := newEnv(t)
	e.write("a.md", "alpha\n")
	if n := noteCount(t, e); n != 1 {
		t.Fatalf("notes = %d", n)
	}
	st := e.serverStatus()
	if !st.Running || st.PID == 0 {
		t.Fatalf("server not running: %+v", st)
	}
	if err := syscall.Kill(st.PID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitDead(t, st.PID)

	e.write("b.md", "bravo crashword\n")
	if n := noteCount(t, e); n != 2 {
		t.Fatalf("notes after crash = %d, want 2", n)
	}
	st2 := e.serverStatus()
	if !st2.Running || st2.PID == st.PID {
		t.Fatalf("want fresh server, old %d new %+v", st.PID, st2)
	}
	if out := e.ok("search", "-t", "--json", "crashword"); !strings.Contains(out, "b.md") {
		t.Fatalf("search: %s", out)
	}

	e.ok("server", "stop")
	waitDead(t, st2.PID)
	// server.lock is a persistent flock file by design; a held lock would have
	// blocked the fresh server above, so only the socket and pid files matter.
	entries, _ := os.ReadDir(e.st + "/gote")
	for _, en := range entries {
		if strings.HasSuffix(en.Name(), ".sock") || strings.HasSuffix(en.Name(), ".pid") {
			t.Errorf("stale file after stop: %s", en.Name())
		}
	}
}

func TestDBDeletionRebuilds(t *testing.T) {
	e := newEnv(t)
	e.write("a.md", "alpha [[b]]\n")
	e.write("b.md", "bravo\n")
	e.ok("pin", "a")
	e.ok("view", "-T", "b")
	if out := e.ok("pinned", "--json"); !strings.Contains(out, "a.md") {
		t.Fatalf("pin not recorded: %s", out)
	}
	e.ok("server", "stop")
	db := filepath.Join(e.st, "gote", "gote.db")
	for _, suf := range []string{"", "-wal", "-shm"} {
		os.Remove(db + suf)
	}
	var s struct{ Notes, Links int }
	if err := json.Unmarshal([]byte(e.ok("status", "--json")), &s); err != nil {
		t.Fatal(err)
	}
	if s.Notes != 2 || s.Links != 1 {
		t.Fatalf("rebuild counts: %+v", s)
	}
	// Expected: pins and visits live only in the db, so they are gone after deletion.
	out := e.ok("pinned", "--json")
	if strings.Contains(out, "a.md") {
		t.Fatalf("pin survived db deletion: %s", out)
	}
}

func TestUpgradeRestartsServer(t *testing.T) {
	e := newEnv(t)
	e.write("a.md", "alpha\n")
	if r := e.runV("vA", "status"); r.code != 0 || r.err != "" {
		t.Fatalf("A: %+v", r)
	}
	a := e.serverStatus()
	if a.Version != "vA" {
		t.Fatalf("server version %q", a.Version)
	}
	r := e.runV("vB", "status", "--json")
	if r.code != 0 || r.err != "" {
		t.Fatalf("B: %+v", r)
	}
	b := e.serverStatus()
	if b.Version != "vB" || !b.Running {
		t.Fatalf("after upgrade: %+v", b)
	}
	if b.PID == a.PID {
		t.Fatalf("server pid unchanged: %d", a.PID)
	}
}
