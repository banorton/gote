package notes

import (
	"encoding/json"
	"strings"
	"testing"
)

func archiveSandbox(t *testing.T) *sandbox {
	s := newSandbox(t)
	s.write("live.md", "---\ntags: [work]\n---\nlive\n")
	s.write("old.md", "---\ntags: [work, archive]\n---\nold\n")
	return s
}

func listIDs(t *testing.T, s *sandbox, args ...string) []string {
	t.Helper()
	code, out := s.run(append([]string{"--json"}, args...)...)
	if code != 0 {
		t.Fatalf("%v: exit %d %q", args, code, out)
	}
	var hits []struct{ ID string }
	if err := json.Unmarshal([]byte(out), &hits); err != nil {
		t.Fatalf("%v: %v %q", args, err, out)
	}
	var ids []string
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	return ids
}

func TestListsHideArchivedUnlessAll(t *testing.T) {
	s := archiveSandbox(t)
	s.run("pin", "live")
	s.run("pin", "old")
	s.run("--json", "info", "live")
	s.run("live")
	s.run("old")
	for _, cmd := range [][]string{{"get"}, {"recent"}, {"pinned"}, {"tag", "work"}} {
		if got := strings.Join(listIDs(t, s, cmd...), ","); got != "live.md" {
			t.Errorf("%v: got %q", cmd, got)
		}
		all := append(append([]string{}, cmd...), "--all")
		if got := len(listIDs(t, s, all...)); got != 2 {
			t.Errorf("%v: got %d", all, got)
		}
	}
	if got := strings.Join(listIDs(t, s, "tag", "archive"), ","); got != "old.md" {
		t.Errorf("tag archive: %q", got)
	}
}

func TestArchiveUnarchiveCommands(t *testing.T) {
	s := archiveSandbox(t)
	s.write("plain.md", "plain body\n")
	code, out := s.run("--json", "archive", "live", "plain")
	if code != 0 {
		t.Fatalf("%d %q", code, out)
	}
	var changed []string
	if err := json.Unmarshal([]byte(out), &changed); err != nil || len(changed) != 2 {
		t.Fatalf("%v %q", err, out)
	}
	if got := s.read("plain.md"); got != "---\ntags:\n  - archive\n---\nplain body\n" {
		t.Fatalf("%q", got)
	}
	if got := strings.Join(listIDs(t, s, "get"), ","); got != "" {
		t.Fatalf("archived notes still listed: %q", got)
	}
	code, out = s.run("unarchive", "old", "plain")
	if code != 0 || !strings.Contains(out, "unarchived 2") {
		t.Fatalf("%d %q", code, out)
	}
	if got := strings.Join(listIDs(t, s, "get"), ","); got != "old.md,plain.md" {
		t.Fatalf("%q", got)
	}
	if code, _ := s.run("archive"); code != 2 {
		t.Fatalf("archive with no note must be a usage error, got %d", code)
	}
	if code, _ := s.run("archive", "nonexistent-zzz"); code != 1 {
		t.Fatalf("got %d", code)
	}
}
