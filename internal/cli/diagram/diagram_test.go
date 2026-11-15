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

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	dir := t.TempDir()
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

func TestBareIsNotAvailable(t *testing.T) {
	for _, args := range [][]string{{}, {"my sketch"}} {
		if _, err := run(t, args...); err == nil || err.Error() != "diagram editor not available yet" {
			t.Errorf("%v: %v", args, err)
		}
	}
}
