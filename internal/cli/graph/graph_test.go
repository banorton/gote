package graph

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
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/graph"
	"github.com/banorton/gote/internal/store"
)

var fixture = map[string]string{
	"a.md":      "# A\n\nto [[b]] and [[nope]] and [[a]]\n",
	"b.md":      "# B\n\n![[c]]\n\nback [[a]]\n",
	"c.md":      "---\nparent: a\nrelated: [d2]\n---\n# C\n",
	"d2.md":     "# D2\n",
	"orphan.md": "# Orphan\n",
}

type resolveReq struct {
	Ref string `json:"ref"`
}
type resolveResp struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Path  string `json:"path"`
}

func init() {
	ops.Handle("notes.resolve", func(ctx context.Context, env *ops.Env, r resolveReq) (resolveResp, error) {
		id := r.Ref
		if id == "-" {
			id = "a.md"
		} else if !strings.HasSuffix(id, ".md") {
			id += ".md"
		}
		var out resolveResp
		err := env.Store.DB.QueryRow("SELECT id, title, kind FROM notes WHERE id=?", id).Scan(&out.ID, &out.Title, &out.Kind)
		if err != nil {
			return out, ops.Errorf(ops.CodeNotFound, "note %q not found", r.Ref)
		}
		return out, nil
	})
}

func setup(t *testing.T, jsonOut bool) (*cli.Ctx, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range fixture {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ix := index.New(st, dir)
	if _, err := ix.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	c := &cli.Ctx{Client: client.Local(&ops.Env{Store: st, Index: ix, Now: time.Now}), Out: &out, Err: &out, JSON: jsonOut}
	return c, &out
}

func TestGraphDepth2Golden(t *testing.T) {
	c, out := setup(t, false)
	if err := runGraph(c, []string{"a", "-d", "2"}); err != nil {
		t.Fatal(err)
	}
	want := `a
├── → b
│   └── → c [embed] (seen)
└── ← c [parent]
    ├── → d2 [related]
    └── ← b [embed] (seen)
`
	if out.String() != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestGraphFlagBeforeNoteAndDefault(t *testing.T) {
	c, out := setup(t, false)
	if err := runGraph(c, []string{"-d", "1", "a"}); err != nil {
		t.Fatal(err)
	}
	want := "a\n├── → b\n└── ← c [parent]\n"
	if out.String() != want {
		t.Fatalf("got:\n%s", out)
	}
	out.Reset()
	if err := runGraph(c, []string{"a"}); err != nil || out.String() != want {
		t.Fatalf("default depth: %v\n%s", err, out)
	}
}

func TestGraphUsage(t *testing.T) {
	c, _ := setup(t, false)
	for _, args := range [][]string{nil, {"a", "-d", "0"}, {"a", "-d", "6"}, {"a", "b"}} {
		var ue *cli.UsageError
		if err := runGraph(c, args); !errors.As(err, &ue) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
}

func TestLinksText(t *testing.T) {
	c, out := setup(t, false)
	if err := runLinks(c, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	want := "b.md :3\nnope (unresolved) :3\na.md :3\n"
	if out.String() != want {
		t.Fatalf("got:\n%s", out)
	}
	out.Reset()
	if err := runLinks(c, []string{"c"}); err != nil {
		t.Fatal(err)
	}
	if want := "d2.md [related]\na.md [parent]\n"; out.String() != want {
		t.Fatalf("got:\n%s", out)
	}
}

func TestBacklinksText(t *testing.T) {
	c, out := setup(t, false)
	if err := runBacklinks(c, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	want := "a.md :3\nb.md :5\nc.md [parent]\n"
	if out.String() != want {
		t.Fatalf("got:\n%s", out)
	}
}

func TestOrphansAndBrokenText(t *testing.T) {
	c, out := setup(t, false)
	if err := runOrphans(c, nil); err != nil || out.String() != "orphan.md\n" {
		t.Fatalf("%v %q", err, out)
	}
	out.Reset()
	if err := runBroken(c, nil); err != nil || out.String() != "a.md:3 nope\n" {
		t.Fatalf("%v %q", err, out)
	}
}

func TestJSON(t *testing.T) {
	c, out := setup(t, true)
	if err := runGraph(c, []string{"a", "-d", "1"}); err != nil {
		t.Fatal(err)
	}
	var n struct {
		ID       string
		Children []struct{ ID, Dir, Kind string }
	}
	if err := json.Unmarshal(out.Bytes(), &n); err != nil || n.ID != "a.md" || len(n.Children) != 2 || n.Children[0].Dir != "out" {
		t.Fatalf("%v %s", err, out)
	}
	for name, run := range map[string]func(*cli.Ctx, []string) error{"orphans": runOrphans, "broken": runBroken} {
		out.Reset()
		if err := run(c, nil); err != nil {
			t.Fatal(err)
		}
		var arr []map[string]any
		if err := json.Unmarshal(out.Bytes(), &arr); err != nil || len(arr) != 1 {
			t.Fatalf("%s: %v %s", name, err, out)
		}
	}
	out.Reset()
	if err := runLinks(c, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	var arr []map[string]any
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil || len(arr) != 3 || arr[1]["id"] != nil {
		t.Fatalf("%v %s", err, out)
	}
}

func TestUnresolvedNoteErrors(t *testing.T) {
	c, _ := setup(t, false)
	err := runLinks(c, []string{"missing"})
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeNotFound {
		t.Fatalf("%v", err)
	}
}
