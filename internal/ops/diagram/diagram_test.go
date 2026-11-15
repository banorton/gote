package diagram

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

var fixedNow = time.Date(2026, 11, 14, 20, 30, 15, 0, time.UTC)

func setup(t *testing.T) (string, *ops.Env, client.Client) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	env := &ops.Env{
		Cfg:   &config.Config{NotesDir: dir, DiagramDir: "diagrams"},
		Store: st,
		Index: index.New(st, dir),
		Now:   func() time.Time { return fixedNow },
	}
	return dir, env, client.Local(env)
}

type createResp struct{ ID, Path string }

func create(t *testing.T, c client.Client, name string) createResp {
	t.Helper()
	var r struct {
		ID   string `json:"id"`
		Path string `json:"path"`
	}
	if err := c.Call(context.Background(), "diagram.create", map[string]string{"name": name}, &r); err != nil {
		t.Fatal(err)
	}
	return createResp{r.ID, r.Path}
}

func TestCreateNaming(t *testing.T) {
	dir, _, c := setup(t)
	r := create(t, c, "")
	if r.ID != "diagrams/261114203015_diagram.excalidraw" {
		t.Fatalf("id %q", r.ID)
	}
	if r.Path != filepath.Join(dir, "diagrams", "261114203015_diagram.excalidraw") {
		t.Fatalf("path %q", r.Path)
	}
	r = create(t, c, "system design v2")
	if r.ID != "diagrams/261114203015_diagram_system_design_v2.excalidraw" {
		t.Fatalf("id %q", r.ID)
	}
}

func TestCreateRoundTripsAndIndexes(t *testing.T) {
	_, env, c := setup(t)
	r := create(t, c, "x")
	src, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := excal.Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(excal.Check(d)) != 0 || len(d.Elements) != 0 {
		t.Fatalf("%+v", d)
	}
	var kind string
	if err := env.Store.DB.QueryRow("SELECT kind FROM notes WHERE id=?", r.ID).Scan(&kind); err != nil || kind != "diagram" {
		t.Fatalf("kind %q err %v", kind, err)
	}
}

func TestCreateCollisionIsConflict(t *testing.T) {
	_, _, c := setup(t)
	create(t, c, "x")
	err := c.Call(context.Background(), "diagram.create", map[string]string{"name": "x"}, nil)
	if oe, ok := err.(*ops.Error); !ok || oe.Code != ops.CodeConflict {
		t.Fatalf("%v", err)
	}
}

func TestCreateRejectsPathNames(t *testing.T) {
	_, _, c := setup(t)
	for _, n := range []string{"a/b", `a\b`, "../x"} {
		err := c.Call(context.Background(), "diagram.create", map[string]string{"name": n}, nil)
		if oe, ok := err.(*ops.Error); !ok || oe.Code != ops.CodeInvalid {
			t.Errorf("%q: %v", n, err)
		}
	}
}

func writeFixture(t *testing.T, dir, rel, fixture string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "excal", "testdata", fixture))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadAndCheckOps(t *testing.T) {
	dir, _, c := setup(t)
	writeFixture(t, dir, "diagrams/a.excalidraw", "valid.excalidraw")
	var rd struct {
		ID      string
		Path    string
		Summary excal.Summary
	}
	if err := c.Call(context.Background(), "diagram.read", map[string]string{"id": "diagrams/a.excalidraw"}, &rd); err != nil {
		t.Fatal(err)
	}
	if rd.Path != filepath.Join(dir, "diagrams", "a.excalidraw") || len(rd.Summary.Shapes) != 3 || len(rd.Summary.Edges) != 2 {
		t.Fatalf("%+v", rd)
	}
	var ck struct{ Issues []excal.Issue }
	if err := c.Call(context.Background(), "diagram.check", map[string]string{"id": "diagrams/a.excalidraw"}, &ck); err != nil || len(ck.Issues) != 0 {
		t.Fatalf("%v %v", ck, err)
	}
	broken := `{"type":"excalidraw","elements":[{"id":"a1","type":"arrow","startBinding":{"elementId":"nope"}}]}`
	os.WriteFile(filepath.Join(dir, "diagrams", "b.excalidraw"), []byte(broken), 0o644)
	if err := c.Call(context.Background(), "diagram.check", map[string]string{"id": "diagrams/b.excalidraw"}, &ck); err != nil || len(ck.Issues) != 1 {
		t.Fatalf("%v %v", ck, err)
	}
}

func TestReadErrors(t *testing.T) {
	dir, _, c := setup(t)
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.excalidraw"), []byte("{oops"), 0o644)
	for id, code := range map[string]string{
		"missing.excalidraw":   ops.CodeNotFound,
		"n.md":                 ops.CodeInvalid,
		"bad.excalidraw":       ops.CodeInvalid,
		"../escape.excalidraw": ops.CodeInvalid,
	} {
		err := c.Call(context.Background(), "diagram.read", map[string]string{"id": id}, nil)
		if oe, ok := err.(*ops.Error); !ok || oe.Code != code {
			t.Errorf("%s: want %s got %v", id, code, err)
		}
	}
}

func TestVaultIndexingFTSAndEmbedResolution(t *testing.T) {
	dir, env, _ := setup(t)
	writeFixture(t, dir, "diagrams/261114203015_diagram_x.excalidraw", "valid.excalidraw")
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("embed ![[261114203015_diagram_x]]\nplain [[261114203015_diagram_x.excalidraw]]\npath [[diagrams/261114203015_diagram_x.excalidraw]]\nmd [d](diagrams/261114203015_diagram_x.excalidraw)\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "design doc.md"), []byte("d"), 0o644)
	if _, err := env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	db := env.Store.DB
	var id string
	if err := db.QueryRow("SELECT note_id FROM notes_fts WHERE notes_fts MATCH 'cache'").Scan(&id); err != nil || id != "diagrams/261114203015_diagram_x.excalidraw" {
		t.Fatalf("fts %q %v", id, err)
	}
	rows, err := db.Query("SELECT dst_raw, dst_id FROM links WHERE src='a.md'")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for rows.Next() {
		var raw string
		var dst *string
		rows.Scan(&raw, &dst)
		n++
		if dst == nil || *dst != "diagrams/261114203015_diagram_x.excalidraw" {
			t.Errorf("%s unresolved: %v", raw, dst)
		}
	}
	if n != 4 {
		t.Fatalf("links %d", n)
	}
	var dd, sn *string
	db.QueryRow("SELECT dst_id FROM links WHERE src='diagrams/261114203015_diagram_x.excalidraw' AND dst_raw='Design Doc'").Scan(&dd)
	db.QueryRow("SELECT dst_id FROM links WHERE src='diagrams/261114203015_diagram_x.excalidraw' AND dst_raw='server notes'").Scan(&sn)
	if dd == nil || *dd != "design doc.md" || sn != nil {
		t.Fatalf("diagram outbound links: %v %v", dd, sn)
	}
}

func TestListNewestFirstDrawingsOnly(t *testing.T) {
	dir, env, c := setup(t)
	write := func(rel string, mt time.Time) {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, excal.NewScene(), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, mt, mt)
	}
	write("old.excalidraw", time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local))
	write("diagrams/new.excalidraw", time.Date(2026, 3, 4, 5, 6, 7, 0, time.Local))
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("x"), 0o644)
	if _, err := env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rs []ListItem
	if err := c.Call(context.Background(), "diagram.list", ListReq{}, &rs); err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || rs[0].ID != "diagrams/new.excalidraw" || rs[1].ID != "old.excalidraw" {
		t.Fatalf("%+v", rs)
	}
	if rs[0].Modified != "260304050607" || rs[0].Path != filepath.Join(dir, "diagrams", "new.excalidraw") || rs[0].Title == "" {
		t.Fatalf("%+v", rs[0])
	}
	if err := c.Call(context.Background(), "diagram.list", ListReq{Limit: 1}, &rs); err != nil || len(rs) != 1 {
		t.Fatalf("%v %+v", err, rs)
	}
	err := c.Call(context.Background(), "diagram.list", ListReq{Limit: -1}, &rs)
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeInvalid {
		t.Fatalf("%v", err)
	}
}

func TestListEmptyIsNonNil(t *testing.T) {
	_, _, c := setup(t)
	var rs []ListItem
	if err := c.Call(context.Background(), "diagram.list", ListReq{}, &rs); err != nil || rs == nil || len(rs) != 0 {
		t.Fatalf("%v %#v", err, rs)
	}
}
