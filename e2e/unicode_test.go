package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/unicode/norm"
)

var oddNames = []string{
	"two words",
	"Tom & Jerry",
	"it's mine",
	"party \U0001F389 time",
	"Café crème",
	"日本語のノート",
	"mix 日本 & café \U0001F680's",
}

func jsonIDs(t *testing.T, out string) []string {
	t.Helper()
	var wrapped struct {
		Results []struct{ ID string }
	}
	if json.Unmarshal([]byte(out), &wrapped) == nil && wrapped.Results != nil {
		var ids []string
		for _, r := range wrapped.Results {
			ids = append(ids, r.ID)
		}
		return ids
	}
	var list []struct{ ID string }
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatalf("parse %q: %v", out, err)
	}
	var ids []string
	for _, r := range list {
		ids = append(ids, r.ID)
	}
	return ids
}

func contains(ids []string, id string) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

func TestUnicodeAndOddFilenames(t *testing.T) {
	e := newEnv(t)
	hub := "hub"
	var hubBody strings.Builder
	for _, n := range oddNames {
		var r struct {
			ID      string
			Path    string
			Created bool
		}
		if err := json.Unmarshal([]byte(e.ok("new", "--json", n)), &r); err != nil || !r.Created || r.ID != n+".md" {
			t.Fatalf("new %q: %+v %v", n, r, err)
		}
		if _, err := os.Stat(filepath.Join(e.dir, n+".md")); err != nil {
			t.Fatalf("file for %q: %v", n, err)
		}
		hubBody.WriteString("- [[" + n + "]]\n")
		// distinct body token per note
		os.WriteFile(filepath.Join(e.dir, n+".md"), []byte("body marker"+strings.NewReplacer(" ", "", "&", "").Replace("x")+"\n"), 0o644)
	}
	e.write(hub+".md", hubBody.String())

	if got := noteCount(t, e); got != len(oddNames)+1 {
		t.Fatalf("notes = %d", got)
	}
	var br struct{ Unresolved int }
	json.Unmarshal([]byte(e.ok("status", "--json")), &br)
	if br.Unresolved != 0 {
		t.Fatalf("unresolved links: %d (%s)", br.Unresolved, e.ok("broken"))
	}

	for _, n := range oddNames {
		q := strings.Fields(n)[0]
		if ids := jsonIDs(t, e.ok("search", "--json", q)); !contains(ids, n+".md") {
			t.Errorf("fuzzy %q: %v", q, ids)
		}
		if ids := jsonIDs(t, e.ok("search", "--json", n)); !contains(ids, n+".md") {
			t.Errorf("fuzzy full %q: %v", n, ids)
		}
		if ids := jsonIDs(t, e.ok("backlinks", "--json", n)); !contains(ids, hub+".md") {
			t.Errorf("backlinks %q: %v", n, ids)
		}
		if out := e.ok("view", "-T", n); !strings.Contains(out, "body marker") || strings.Contains(out, "\x1b") {
			t.Errorf("view -T %q: %q", n, out)
		}
	}
	// title text search hits the title column with non-ASCII tokens
	for tok, want := range map[string]string{"日本語のノート": "日本語のノート.md", "Jerry": "Tom & Jerry.md", "café": "Café crème.md"} {
		if ids := jsonIDs(t, e.ok("search", "-t", "--json", tok)); !contains(ids, want) {
			t.Errorf("-t %q: %v", tok, ids)
		}
	}

	// rename rewrites inbound links
	newName := "Renamed & 日本 \U0001F680's"
	e.ok("rename", "Tom & Jerry", "-n", newName)
	b, _ := os.ReadFile(filepath.Join(e.dir, hub+".md"))
	if !strings.Contains(string(b), "[["+newName+"]]") || strings.Contains(string(b), "[[Tom & Jerry]]") {
		t.Fatalf("hub after rename:\n%s", b)
	}
	if ids := jsonIDs(t, e.ok("backlinks", "--json", newName)); !contains(ids, hub+".md") {
		t.Fatalf("backlinks after rename: %v", ids)
	}

	// delete / recover
	victim := "Café crème"
	e.ok("delete", victim)
	if _, err := os.Stat(filepath.Join(e.dir, victim+".md")); !os.IsNotExist(err) {
		t.Fatalf("not deleted: %v", err)
	}
	e.ok("recover", victim)
	if _, err := os.Stat(filepath.Join(e.dir, victim+".md")); err != nil {
		t.Fatalf("not recovered: %v", err)
	}
	if ids := jsonIDs(t, e.ok("backlinks", "--json", victim)); !contains(ids, hub+".md") {
		t.Fatalf("backlinks after recover: %v", ids)
	}
}

// macOS tools write NFD filenames; links typed as NFC must still resolve.
func TestUnicodeNormalizationForms(t *testing.T) {
	e := newEnv(t)
	nfc := "Café"
	nfd := norm.NFD.String(nfc)
	e.write(nfd+".md", "target\n")
	e.write("hub.md", "[["+nfc+"]]\n")
	if ids := jsonIDs(t, e.ok("backlinks", "--json", nfc)); !contains(ids, "hub.md") {
		t.Fatalf("NFC link to NFD file unresolved: %v", ids)
	}
}
