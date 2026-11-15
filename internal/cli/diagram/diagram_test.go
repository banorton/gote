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
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
	"github.com/banorton/gote/internal/store"
)

// fake of the Phase 2 notes.resolve op: ref is a notes-dir-relative path.
func init() {
	ops.Handle("notes.resolve", func(ctx context.Context, env *ops.Env, req struct {
		Ref string `json:"ref"`
	}) (map[string]string, error) {
		p := filepath.Join(env.Cfg.NotesDir, req.Ref)
		if _, err := os.Stat(p); err != nil {
			return nil, ops.Errorf(ops.CodeNotFound, "no note matches %q", req.Ref)
		}
		kind := "note"
		if strings.HasSuffix(req.Ref, ".excalidraw") {
			kind = "diagram"
		}
		return map[string]string{"id": req.Ref, "title": req.Ref, "kind": kind, "path": p}, nil
	})
}

var lastDir string

func run(t *testing.T, args ...string) (string, error) {
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
