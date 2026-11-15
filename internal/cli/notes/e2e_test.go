package notes

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/banorton/gote/internal/cli"
	_ "github.com/banorton/gote/internal/ops/search"
)

type sandbox struct {
	t     *testing.T
	notes string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	root := t.TempDir()
	cfg := filepath.Join(root, "config")
	notes := filepath.Join(root, "notes")
	for _, d := range []string{filepath.Join(cfg, "gote"), notes} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfg, "gote", "config.yaml"), []byte("notes_dir: "+notes+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("EDITOR", "true")
	return &sandbox{t: t, notes: notes}
}

func (s *sandbox) run(args ...string) (int, string) {
	s.t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		s.t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() { b, _ := io.ReadAll(r); done <- string(b) }()
	code := cli.Main(args)
	w.Close()
	os.Stdout = old
	return code, <-done
}

func (s *sandbox) write(rel, content string) {
	s.t.Helper()
	p := filepath.Join(s.notes, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		s.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		s.t.Fatal(err)
	}
}

func (s *sandbox) read(rel string) string {
	b, err := os.ReadFile(filepath.Join(s.notes, filepath.FromSlash(rel)))
	if err != nil {
		s.t.Fatal(err)
	}
	return string(b)
}

func TestBareNameCreatesNote(t *testing.T) {
	s := newSandbox(t)
	if code, _ := s.run("foo"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got := s.read("foo.md"); !strings.HasPrefix(got, "---\ncreated: \"") {
		t.Fatalf("%q", got)
	}
	s.write("foo.md", "edited\n")
	if code, _ := s.run("foo"); code != 0 || s.read("foo.md") != "edited\n" {
		t.Fatal("existing note must not be recreated")
	}
	if code, _ := s.run("my", "multi word"); code != 0 {
		t.Fatal(code)
	}
	if _, err := os.Stat(filepath.Join(s.notes, "my multi word.md")); err != nil {
		t.Fatal(err)
	}
	if code, _ := s.run("-d", "dated"); code != 0 {
		t.Fatal(code)
	}
	ms, _ := filepath.Glob(filepath.Join(s.notes, "??????_dated.md"))
	if len(ms) != 1 {
		t.Fatalf("date prefix: %v", ms)
	}
	if code, _ := s.run("new", "foo"); code != 1 {
		t.Fatalf("new on existing must conflict, got %d", code)
	}
	if code, _ := s.run("new", "fresh"); code != 0 {
		t.Fatal(code)
	}
}

func TestDashOpensLastVisited(t *testing.T) {
	s := newSandbox(t)
	s.write("one.md", "x")
	s.run("one")
	code, out := s.run("--json", "info", "-")
	if code != 0 {
		t.Fatal(code)
	}
	var info struct{ ID string }
	if err := json.Unmarshal([]byte(out), &info); err != nil || info.ID != "one.md" {
		t.Fatalf("%v %q", err, out)
	}
}

func TestAliasesResolve(t *testing.T) {
	want := map[string]string{
		"q": "quick", "qs": "quick-save", "p": "pinned", "u": "unpin", "up": "unpin", "r": "recent", "t": "tag",
		"d": "delete", "del": "delete", "mv": "rename", "rn": "rename", "dup": "duplicate", "cp": "duplicate",
		"i": "info", "tmpl": "template", "c": "config", "ce": "config-edit", "g": "get", "exp": "export", "imp": "import",
	}
	for _, n := range []string{"ro", "rd", "rp", "rv", "rr", "so", "sd", "sp", "sv", "sr", "to", "td", "tp", "tv", "tr", "po", "pd", "pv", "pu", "pr"} {
		want[n] = n
	}
	for alias, name := range want {
		cmd, ok := cli.Lookup(alias)
		if !ok || cmd.Name != name {
			t.Errorf("alias %q -> %v (ok=%v), want %q", alias, cmd, ok, name)
		}
	}
	for _, n := range []string{"open", "new", "quick", "today", "recent", "pinned", "pin", "unpin", "tag", "delete", "trash", "recover", "rename", "duplicate", "info", "template", "config", "get", "export", "import"} {
		if _, ok := cli.Lookup(n); !ok {
			t.Errorf("command %q missing", n)
		}
	}
}

func TestJSONOutputs(t *testing.T) {
	s := newSandbox(t)
	s.write("a.md", "---\ntags: [x]\n---\nbody\n")
	s.write("b.md", "---\ntags: [x, y]\n---\n[[a]]\n")
	s.run("a")

	_, out := s.run("--json", "info", "a")
	var info struct {
		ID, Title string
		Tags      []string
		Backlinks int
	}
	if err := json.Unmarshal([]byte(out), &info); err != nil || info.ID != "a.md" || len(info.Tags) != 1 || info.Backlinks != 1 {
		t.Fatalf("%v %q", err, out)
	}
	_, out = s.run("--json", "recent")
	var rec []struct{ ID string }
	if err := json.Unmarshal([]byte(out), &rec); err != nil || len(rec) != 1 || rec[0].ID != "a.md" {
		t.Fatalf("%v %q", err, out)
	}
	_, out = s.run("--json", "tag")
	var tags []struct {
		Tag   string
		Count int
	}
	if err := json.Unmarshal([]byte(out), &tags); err != nil || len(tags) != 2 || tags[0].Tag != "x" || tags[0].Count != 2 {
		t.Fatalf("%v %q", err, out)
	}
	_, out = s.run("--json", "tag", "x", "y")
	var nl []struct{ ID string }
	if err := json.Unmarshal([]byte(out), &nl); err != nil || len(nl) != 1 || nl[0].ID != "b.md" {
		t.Fatalf("%v %q", err, out)
	}
}

func TestManagementCommands(t *testing.T) {
	s := newSandbox(t)
	s.write("a.md", "---\ntags: [x]\n---\nbody\n")
	s.write("o.md", "see [[a]]\n")
	if code, _ := s.run("rename", "a", "-n", "b"); code != 0 {
		t.Fatal(code)
	}
	if s.read("o.md") != "see [[b]]\n" {
		t.Fatalf("%q", s.read("o.md"))
	}
	if code, _ := s.run("mv", "b", "c"); code != 0 {
		t.Fatal(code)
	}
	if code, _ := s.run("dup", "c", "d"); code != 0 {
		t.Fatal(code)
	}
	if code, _ := s.run("pin", "d"); code != 0 {
		t.Fatal(code)
	}
	_, out := s.run("--json", "pinned")
	if !strings.Contains(out, `"d.md"`) {
		t.Fatal(out)
	}
	if code, _ := s.run("unpin", "d"); code != 0 {
		t.Fatal(code)
	}
	if code, _ := s.run("tag", "add", "z", "c", "d"); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(s.read("c.md"), "- z") {
		t.Fatal(s.read("c.md"))
	}
	if code, _ := s.run("tag", "rm", "z", "c"); code != 0 || strings.Contains(s.read("c.md"), "- z") {
		t.Fatal("tag rm")
	}
	if code, _ := s.run("delete", "d"); code != 0 {
		t.Fatal(code)
	}
	if _, err := os.Stat(filepath.Join(s.notes, ".trash", "d.md")); err != nil {
		t.Fatal(err)
	}
	if code, _ := s.run("recover", "d"); code != 0 {
		t.Fatal(code)
	}
	if code, _ := s.run("delete", "nonexistent-zzz"); code != 1 {
		t.Fatal("delete missing must fail")
	}
	arc := filepath.Join(t.TempDir(), "e.tar.gz")
	if code, _ := s.run("export", arc); code != 0 {
		t.Fatal(code)
	}
	if _, err := os.Stat(arc); err != nil {
		t.Fatal(err)
	}
	if code, _ := s.run("quick"); code != 0 {
		t.Fatal(code)
	}
	s.write(".quick.md", "idea\n")
	if code, _ := s.run("qs", "idea note"); code != 0 {
		t.Fatal(code)
	}
	if !strings.Contains(s.read("idea note.md"), "idea") {
		t.Fatal("quick save")
	}
	if code, _ := s.run("today", "-1"); code != 0 {
		t.Fatal(code)
	}
	ms, _ := filepath.Glob(filepath.Join(s.notes, "daily", "*_daily.md"))
	if len(ms) != 1 {
		t.Fatal(ms)
	}
	if code, _ := s.run("template", "meeting"); code != 0 {
		t.Fatal(code)
	}
	if code, out := s.run("--json", "template", "list"); code != 0 || !strings.Contains(out, "meeting") {
		t.Fatal(out)
	}
	if code, _ := s.run("-t", "meeting", "withtpl"); code != 0 {
		t.Fatal(code)
	}
}

func TestListersNonInteractivePrintList(t *testing.T) {
	s := newSandbox(t)
	s.write("a.md", "x")
	s.run("a")
	code, out := s.run("ro")
	if code != 0 || !strings.Contains(out, "a.md") {
		t.Fatalf("%d %q", code, out)
	}
	if code, _ := s.run("so"); code != 2 {
		t.Fatal("search lister without query is a usage error")
	}
}

func TestSearchListerUsesFuzzyTitles(t *testing.T) {
	s := newSandbox(t)
	s.write("k8s.md", "---\ntitle: Kubernetes notes\n---\npods\n")
	s.write("body.md", "mentions kubernetes in the body\n")
	code, out := s.run("so", "kube")
	if code != 0 || !strings.Contains(out, "k8s.md") || strings.Contains(out, "body.md") {
		t.Fatalf("%d %q", code, out)
	}
	if code, out = s.run("so", "kube", "tag:x"); code != 1 || out != "" {
		t.Fatalf("filter tokens in fuzzy lister must error: %d %q", code, out)
	}
}
