package diagram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/cli/view"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
	"github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/store"
)

var lastDir string

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runWith(t, nil, args...)
}

func runWith(t *testing.T, extra map[string]string, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	lastDir = dir
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	env := &ops.Env{Cfg: &config.Config{NotesDir: dir, DiagramDir: "diagrams"}, Store: st, Index: index.New(st, dir), Now: time.Now}
	copyFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "broken.excalidraw"), []byte(`{"type":"excalidraw","elements":[{"id":"t1","type":"text","text":"lost","containerId":"zzz"}]}`), 0o644)
	os.Chtimes(filepath.Join(dir, "a.excalidraw"), time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local), time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local))
	os.Chtimes(filepath.Join(dir, "broken.excalidraw"), time.Date(2026, 2, 3, 4, 5, 6, 0, time.Local), time.Date(2026, 2, 3, 4, 5, 6, 0, time.Local))
	for name, content := range extra {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}
	if _, err := env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	c := &cli.Ctx{Client: client.Local(env), Cfg: env.Cfg, Out: &out, Err: &out}
	if len(args) > 0 && args[len(args)-1] == "--json" {
		c.JSON = true
		args = args[:len(args)-1]
	}
	err = command.Run(c, args)
	return out.String(), err
}

func copyFixture(t *testing.T, dir, name, fixture string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "excal", "testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadHuman(t *testing.T) {
	out, err := run(t, "read", "a.excalidraw")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`rect-a rectangle "Client"`, `Client -> Server "request"`, `Server -> (unbound)`, `note-1 "see [[Design Doc]]"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestReadJSON(t *testing.T) {
	out, err := run(t, "read", "a.excalidraw", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		ID      string
		Summary struct{ Shapes []any }
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.ID != "a.excalidraw" || len(r.Summary.Shapes) != 3 {
		t.Fatalf("%v %s", err, out)
	}
}

func TestCheckOKAndIssues(t *testing.T) {
	out, err := run(t, "check", "a.excalidraw")
	if err != nil || !strings.Contains(out, "ok") {
		t.Fatalf("%v %q", err, out)
	}
	out, err = run(t, "check", "broken.excalidraw")
	if err == nil {
		t.Fatal("expected error for issues")
	}
	if !strings.Contains(out, "t1") || !strings.Contains(out, "zzz") {
		t.Fatalf("%q", out)
	}
}

func TestCheckJSONStillPrintsIssues(t *testing.T) {
	out, err := run(t, "check", "broken.excalidraw", "--json")
	if err == nil {
		t.Fatal("expected error")
	}
	var r struct {
		Issues []struct{ Element, Problem string }
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil || len(r.Issues) != 1 || r.Issues[0].Element != "t1" {
		t.Fatalf("%v %s", err, out)
	}
}

func TestNonDrawingRefRejected(t *testing.T) {
	if _, err := run(t, "read", "n.md"); err == nil || !strings.Contains(err.Error(), "not a drawing") {
		t.Fatalf("%v", err)
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{{"read"}, {"check"}, {"read", "a", "b"}} {
		_, err := run(t, args...)
		var ue *cli.UsageError
		if !errors.As(err, &ue) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
}

func editorSetup(t *testing.T) *[]string {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var opened []string
	ctx, cancel := context.WithCancel(context.Background())
	origOpen, origWait := view.Opener, view.WaitContext
	t.Cleanup(func() { view.Opener, view.WaitContext = origOpen, origWait; cancel() })
	view.Opener = func(url string) error { opened = append(opened, url); cancel(); return nil }
	view.WaitContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	return &opened
}

func TestBareCreatesDrawingAndOpensEditor(t *testing.T) {
	opened := editorSetup(t)
	out, err := run(t)
	if err != nil {
		t.Fatal(err)
	}
	if len(*opened) != 1 || !strings.Contains((*opened)[0], "/d/diagrams/") || !strings.Contains((*opened)[0], "_diagram.excalidraw?t=") {
		t.Fatalf("opened %v", *opened)
	}
	files, _ := filepath.Glob(filepath.Join(lastDir, "diagrams", "*_diagram.excalidraw"))
	if len(files) != 1 {
		t.Fatalf("created %v", files)
	}
	if !strings.Contains(out, (*opened)[0]) {
		t.Fatalf("URL not printed: %q", out)
	}
}

func TestNamedCreatesDrawingWithName(t *testing.T) {
	opened := editorSetup(t)
	if _, err := run(t, "my", "sketch"); err != nil {
		t.Fatal(err)
	}
	if len(*opened) != 1 || !strings.Contains((*opened)[0], "_diagram_my_sketch.excalidraw?t=") {
		t.Fatalf("opened %v", *opened)
	}
	files, _ := filepath.Glob(filepath.Join(lastDir, "diagrams", "*_diagram_my_sketch.excalidraw"))
	if len(files) != 1 {
		t.Fatalf("created %v", files)
	}
}

func TestOpenExistingDrawing(t *testing.T) {
	opened := editorSetup(t)
	if _, err := run(t, "open", "a.excalidraw"); err != nil {
		t.Fatal(err)
	}
	if len(*opened) != 1 || !strings.Contains((*opened)[0], "/d/a.excalidraw?t=") {
		t.Fatalf("opened %v", *opened)
	}
	if files, _ := filepath.Glob(filepath.Join(lastDir, "diagrams", "*")); len(files) != 0 {
		t.Fatalf("open must not create: %v", files)
	}
}

func TestOpenRejectsNonDrawingAndMissingRef(t *testing.T) {
	opened := editorSetup(t)
	if _, err := run(t, "open", "n.md"); err == nil || !strings.Contains(err.Error(), "not a drawing") {
		t.Fatalf("%v", err)
	}
	var ue *cli.UsageError
	if _, err := run(t, "open"); !errors.As(err, &ue) {
		t.Fatalf("want usage error, got %v", err)
	}
	if len(*opened) != 0 {
		t.Fatal("opened a browser on failure")
	}
}

func noFilesCreated(t *testing.T) {
	t.Helper()
	if files, _ := filepath.Glob(filepath.Join(lastDir, "diagrams", "*")); len(files) != 0 {
		t.Fatalf("created files: %v", files)
	}
}

func TestHelpFlagsPrintUsageAndCreateNothing(t *testing.T) {
	opened := editorSetup(t)
	for _, flag := range []string{"--help", "-h"} {
		out, err := run(t, flag)
		if err != nil {
			t.Fatalf("%s: %v", flag, err)
		}
		for _, want := range []string{"[name]", "open <ref>", "list", "read <ref>", "check <ref>"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: usage missing %q: %q", flag, want, out)
			}
		}
		noFilesCreated(t)
	}
	if len(*opened) != 0 {
		t.Fatal("opened editor")
	}
}

func TestUnknownFlagIsUsageErrorAndCreatesNothing(t *testing.T) {
	opened := editorSetup(t)
	for _, args := range [][]string{{"--bogus"}, {"-x"}, {"-xyz", "name"}, {"name", "--help2"}} {
		_, err := run(t, args...)
		var ue *cli.UsageError
		if !errors.As(err, &ue) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
		noFilesCreated(t)
	}
	if len(*opened) != 0 {
		t.Fatal("opened editor")
	}
}

func TestListNonTTYLines(t *testing.T) {
	out, err := run(t, "list")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "  (broken.excalidraw)") || !strings.HasSuffix(lines[1], "  (a.excalidraw)") {
		t.Fatalf("%q", out)
	}
}

func TestListLimit(t *testing.T) {
	out, err := run(t, "list", "-n", "1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "\n") != 1 || !strings.Contains(out, "(broken.excalidraw)") {
		t.Fatalf("%q", out)
	}
}

func TestListJSON(t *testing.T) {
	out, err := run(t, "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rs []struct{ ID, Title, Path, Modified string }
	if err := json.Unmarshal([]byte(out), &rs); err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || rs[0].ID != "broken.excalidraw" || rs[0].Modified != time.Date(2026, 2, 3, 4, 5, 6, 0, time.Local).Format(time.RFC3339) || rs[1].ID != "a.excalidraw" {
		t.Fatalf("%+v", rs)
	}
	if rs[0].Path != filepath.Join(lastDir, "broken.excalidraw") || rs[0].Title == "" {
		t.Fatalf("%+v", rs[0])
	}
}

func TestListEmptyJSONIsArray(t *testing.T) {
	out, err := run(t, "list", "-n", "0", "--json")
	if err != nil || strings.TrimSpace(out) == "null" {
		t.Fatalf("%q %v", out, err)
	}
}

func TestListRejectsBadArgs(t *testing.T) {
	for _, args := range [][]string{{"list", "extra"}, {"list", "-z"}, {"list", "-n", "-1"}} {
		_, err := run(t, args...)
		if err == nil {
			t.Errorf("%v: want error", args)
		}
	}
}

func TestDescribeSetPrintClear(t *testing.T) {
	out, err := run(t, "describe", "a.excalidraw", "login", "flow", "overview")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("set should be quiet, got %q", out)
	}
	dir := lastDir
	src, _ := os.ReadFile(filepath.Join(dir, "a.excalidraw"))
	d, err := excal.Decode(src)
	if err != nil || d.Gote.Description != "login flow overview" {
		t.Fatalf("%v %q", err, d.Gote.Description)
	}
}

func TestDescribePrintsExisting(t *testing.T) {
	desc := `{"type":"excalidraw","gote":{"description":"seen before"},"elements":[]}`
	out, err := runWith(t, map[string]string{"d.excalidraw": desc}, "describe", "d.excalidraw")
	if err != nil || strings.TrimSpace(out) != "seen before" {
		t.Fatalf("%v %q", err, out)
	}
	out, err = runWith(t, map[string]string{"d.excalidraw": desc}, "describe", "d.excalidraw", "--json")
	var r struct{ ID, Description string }
	if err != nil || json.Unmarshal([]byte(out), &r) != nil || r.ID != "d.excalidraw" || r.Description != "seen before" {
		t.Fatalf("%v %q", err, out)
	}
}

func TestDescribeClear(t *testing.T) {
	desc := `{"type":"excalidraw","gote":{"description":"gone soon"},"elements":[]}`
	out, err := runWith(t, map[string]string{"d.excalidraw": desc}, "describe", "d.excalidraw", "--clear", "--json")
	var r struct{ ID, Description string }
	if err != nil || json.Unmarshal([]byte(out), &r) != nil || r.Description != "" {
		t.Fatalf("%v %q", err, out)
	}
	src, _ := os.ReadFile(filepath.Join(lastDir, "d.excalidraw"))
	if strings.Contains(string(src), "gone soon") || strings.Contains(string(src), `"gote"`) {
		t.Fatalf("%s", src)
	}
}

func TestDescribeStdin(t *testing.T) {
	old := stdin
	stdin = strings.NewReader("long text\nover two lines\n")
	t.Cleanup(func() { stdin = old })
	if _, err := run(t, "describe", "a.excalidraw", "-"); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(filepath.Join(lastDir, "a.excalidraw"))
	d, _ := excal.Decode(src)
	if d.Gote.Description != "long text\nover two lines" {
		t.Fatalf("%q", d.Gote.Description)
	}
}

func TestDescribeUsage(t *testing.T) {
	for _, args := range [][]string{{"describe"}, {"describe", "a.excalidraw", "x", "--clear"}} {
		_, err := run(t, args...)
		var ue *cli.UsageError
		if !errors.As(err, &ue) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
	if _, err := run(t, "describe", "n.md", "x"); err == nil || !strings.Contains(err.Error(), "not a drawing") {
		t.Fatalf("%v", err)
	}
}

var searchFiles = map[string]string{
	"client-flow.excalidraw": `{"type":"excalidraw","gote":{"description":"handshake between peers"},"elements":[{"id":"t","type":"text","text":"zebra label"}]}`,
	"client.md":              "client zebra handshake note\n",
}

func ids(t *testing.T, out string) []string {
	t.Helper()
	var r struct {
		Results []struct{ ID, Title, Path string }
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("%v %q", err, out)
	}
	var got []string
	for _, x := range r.Results {
		if x.Title == "" || x.Path == "" {
			t.Fatalf("incomplete result %+v", x)
		}
		got = append(got, x.ID)
	}
	return got
}

func TestSearchFuzzyOnlyDrawings(t *testing.T) {
	out, err := runWith(t, searchFiles, "search", "client")
	if err != nil || strings.TrimSpace(out) != "client-flow  (client-flow.excalidraw)" {
		t.Fatalf("%v %q", err, out)
	}
	out, err = runWith(t, searchFiles, "search", "client", "--json")
	if got := ids(t, out); err != nil || strings.Join(got, ",") != "client-flow.excalidraw" {
		t.Fatalf("%v %v", err, got)
	}
	var raw struct{ Results []map[string]any }
	json.Unmarshal([]byte(out), &raw)
	for _, k := range []string{"id", "title", "path", "score"} {
		if _, ok := raw.Results[0][k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
}

func TestSearchTextOnlyDrawingsIncludingDescription(t *testing.T) {
	for _, q := range []string{"zebra", "handshake"} {
		out, err := runWith(t, searchFiles, "search", "-t", q, "--json")
		if got := ids(t, out); err != nil || strings.Join(got, ",") != "client-flow.excalidraw" {
			t.Fatalf("%q: %v %v", q, err, got)
		}
	}
	out, err := runWith(t, searchFiles, "search", "--text", "handshake")
	if err != nil || !strings.HasPrefix(out, "client-flow  client-flow.excalidraw:1\n    zebra label handshake") || strings.Contains(out, "client.md") {
		t.Fatalf("%v %q", err, out)
	}
}

func TestSearchSemanticOnlyDrawings(t *testing.T) {
	var gotKind string
	search.SemanticHook = func(ctx context.Context, env *ops.Env, q string, limit int, kind string, all bool) ([]search.Result, error) {
		gotKind = kind
		return []search.Result{{ID: "client-flow.excalidraw", Title: "client-flow", Kind: "diagram"}}, nil
	}
	t.Cleanup(func() { search.SemanticHook = nil })
	out, err := runWith(t, searchFiles, "search", "-s", "peers talking", "--json")
	if got := ids2(out); err != nil || gotKind != "diagram" || len(got) != 1 {
		t.Fatalf("%v %q %q", err, gotKind, out)
	}
}

func ids2(out string) []string {
	var r struct{ Results []struct{ ID string } }
	json.Unmarshal([]byte(out), &r)
	var got []string
	for _, x := range r.Results {
		got = append(got, x.ID)
	}
	return got
}

func TestSearchUsageErrors(t *testing.T) {
	for _, args := range [][]string{{"search"}, {"search", "-t", "-s", "x"}, {"search", "-t"}, {"search", "-n", "0", "x"}} {
		_, err := run(t, args...)
		var ue *cli.UsageError
		if !errors.As(err, &ue) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
}

func TestSearchLimit(t *testing.T) {
	files := map[string]string{"k1.excalidraw": `{"type":"excalidraw","elements":[]}`, "k2.excalidraw": `{"type":"excalidraw","elements":[]}`}
	out, err := runWith(t, files, "search", "-n", "1", "k", "--json")
	if got := ids2(out); err != nil || len(got) != 1 {
		t.Fatalf("%v %q", err, out)
	}
}

func TestReadShowsDescription(t *testing.T) {
	desc := `{"type":"excalidraw","gote":{"description":"seen in read"},"elements":[]}`
	out, err := runWith(t, map[string]string{"d.excalidraw": desc}, "read", "d.excalidraw")
	if err != nil || !strings.Contains(out, "description\n  seen in read") {
		t.Fatalf("%v %q", err, out)
	}
	out, err = runWith(t, map[string]string{"d.excalidraw": desc}, "read", "d.excalidraw", "--json")
	if err != nil || !strings.Contains(out, `"description": "seen in read"`) {
		t.Fatalf("%v %q", err, out)
	}
}

func TestRenderJSONAndPlain(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	out, err := run(t, "render", "a.excalidraw", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		ID            string
		Path          string
		Width, Height int
		Fresh         bool
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.ID != "a.excalidraw" || !strings.HasSuffix(r.Path, ".png") || r.Width < 64 || r.Height < 64 || !r.Fresh {
		t.Fatalf("%v %s", err, out)
	}
	if _, err := os.Stat(r.Path); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"width"`) || !strings.Contains(out, `"fresh"`) {
		t.Fatalf("json keys: %s", out)
	}

	plain, err := run(t, "render", "a.excalidraw")
	if err != nil || strings.TrimSpace(plain) != r.Path {
		t.Fatalf("%v %q want %q", err, plain, r.Path)
	}
}

func TestRenderRejectsNonDrawingAndBadArgs(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if _, err := run(t, "render", "n.md"); err == nil {
		t.Fatal("note should be rejected")
	}
	if _, err := run(t, "render"); err == nil {
		t.Fatal("missing ref should be a usage error")
	}
}

func TestSearchNoArgsNeedsTerminal(t *testing.T) {
	_, err := runWith(t, searchFiles, "search")
	var ue *cli.UsageError
	if !errors.As(err, &ue) {
		t.Fatalf("%v", err)
	}
}
