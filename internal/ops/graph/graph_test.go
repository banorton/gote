package graph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

var fixture = map[string]string{
	"a.md":      "# A\n\nto [[b]] and [[nope]] and [[a]]\n",
	"b.md":      "# B\n\n![[c]]\n\nback [[a]]\n",
	"c.md":      "---\nparent: a\nrelated: [d2]\n---\n# C\n",
	"d2.md":     "# D2\n",
	"orphan.md": "# Orphan\n",
	"self.md":   "# Self\n\n[[self]]\n",
	"deep.md":   "# Deep\n\n[[d2]]\n",
}

func setup(t *testing.T) client.Client {
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
	return client.Local(&ops.Env{Store: st, Index: ix, Now: time.Now})
}

func call(t *testing.T, c client.Client, op string, req any, resp any) error {
	t.Helper()
	return c.Call(context.Background(), op, req, resp)
}

func sp(s string) *string { return &s }

func TestLinks(t *testing.T) {
	c := setup(t)
	var got []LinkOut
	if err := call(t, c, "graph.links", IDReq{ID: "a.md"}, &got); err != nil {
		t.Fatal(err)
	}
	want := []LinkOut{
		{ID: sp("b.md"), Raw: "b", Title: "b", Kind: "wiki", Line: 3},
		{ID: nil, Raw: "nope", Title: "nope", Kind: "wiki", Line: 3},
		{ID: sp("a.md"), Raw: "a", Title: "a", Kind: "wiki", Line: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}

	got = nil
	if err := call(t, c, "graph.links", IDReq{ID: "c.md"}, &got); err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, l := range got {
		kinds[l.Kind] = *l.ID
	}
	if !reflect.DeepEqual(kinds, map[string]string{"parent": "a.md", "related": "d2.md"}) {
		t.Fatalf("got %+v", got)
	}

	got = nil
	if err := call(t, c, "graph.links", IDReq{ID: "orphan.md"}, &got); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil: %v %v", got, err)
	}
	var e *ops.Error
	err := call(t, c, "graph.links", IDReq{ID: "zzz.md"}, &got)
	if !asErr(err, &e) || e.Code != ops.CodeNotFound {
		t.Fatalf("%v", err)
	}
}

func asErr(err error, target **ops.Error) bool {
	oe, ok := err.(*ops.Error)
	if ok {
		*target = oe
	}
	return ok
}

func TestBacklinks(t *testing.T) {
	c := setup(t)
	var got []BackOut
	if err := call(t, c, "graph.backlinks", IDReq{ID: "a.md"}, &got); err != nil {
		t.Fatal(err)
	}
	want := []BackOut{
		{ID: "a.md", Title: "a", Kind: "wiki", Line: 3},
		{ID: "b.md", Title: "b", Kind: "wiki", Line: 5},
		{ID: "c.md", Title: "c", Kind: "parent", Line: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestOrphans(t *testing.T) {
	c := setup(t)
	var got []NoteRef
	if err := call(t, c, "graph.orphans", nil, &got); err != nil {
		t.Fatal(err)
	}
	want := []NoteRef{
		{ID: "orphan.md", Title: "orphan", Kind: "note"},
		{ID: "self.md", Title: "self", Kind: "note"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestBroken(t *testing.T) {
	c := setup(t)
	var got []Broken
	if err := call(t, c, "graph.broken", nil, &got); err != nil {
		t.Fatal(err)
	}
	want := []Broken{{Src: "a.md", Raw: "nope", Line: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func tree(t *testing.T, c client.Client, id string, depth int) Node {
	t.Helper()
	var n Node
	if err := call(t, c, "graph.neighbors", NeighborsReq{ID: id, Depth: depth}, &n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestNeighborsDepth1(t *testing.T) {
	c := setup(t)
	got := tree(t, c, "a.md", 0)
	want := Node{ID: "a.md", Title: "a", Children: []Node{
		{ID: "b.md", Title: "b", Dir: "out", Kind: "wiki", Seen: false, Children: []Node{}},
		{ID: "c.md", Title: "c", Dir: "in", Kind: "parent", Children: []Node{}},
	}}
	if !reflect.DeepEqual(got, want) {
		b, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("got %s", b)
	}
}

func TestNeighborsDepth2CycleTerminates(t *testing.T) {
	c := setup(t)
	got := tree(t, c, "a.md", 2)
	want := Node{ID: "a.md", Title: "a", Children: []Node{
		{ID: "b.md", Title: "b", Dir: "out", Kind: "wiki", Children: []Node{
			{ID: "c.md", Title: "c", Dir: "out", Kind: "embed", Seen: true, Children: []Node{}},
		}},
		{ID: "c.md", Title: "c", Dir: "in", Kind: "parent", Children: []Node{
			{ID: "d2.md", Title: "d2", Dir: "out", Kind: "related", Children: []Node{}},
			{ID: "b.md", Title: "b", Dir: "in", Kind: "embed", Seen: true, Children: []Node{}},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		b, _ := json.MarshalIndent(got, "", " ")
		t.Fatalf("got %s", b)
	}
}

func TestNeighborsDepthBounds(t *testing.T) {
	c := setup(t)
	for _, d := range []int{-1, 6} {
		var n Node
		var e *ops.Error
		err := call(t, c, "graph.neighbors", NeighborsReq{ID: "a.md", Depth: d}, &n)
		if !asErr(err, &e) || e.Code != ops.CodeInvalid {
			t.Fatalf("depth %d: %v", d, err)
		}
	}
	var n Node
	var e *ops.Error
	err := call(t, c, "graph.neighbors", NeighborsReq{ID: "nope.md"}, &n)
	if !asErr(err, &e) || e.Code != ops.CodeNotFound {
		t.Fatalf("%v", err)
	}
}
