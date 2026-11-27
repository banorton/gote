package ops_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
	opgraph "github.com/banorton/gote/internal/ops/graph"
	opnotes "github.com/banorton/gote/internal/ops/notes"
	opsearch "github.com/banorton/gote/internal/ops/search"
	optasks "github.com/banorton/gote/internal/ops/tasks"
	"github.com/banorton/gote/internal/store"
)

type av struct {
	c   client.Client
	dir string
}

func archiveVault(t *testing.T) *av {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"live.md":         "---\ntags: [work]\n---\nzebra live [[old]]\n- [ ] live task\n",
		"old.md":          "---\ntags: [Archive, work]\n---\nzebra old\n- [ ] old task\n",
		"lonely.md":       "---\ntags: [archive]\n---\nzebra lonely\n",
		"solo.md":         "zebra solo\n",
		"d.excalidraw":    string(excal.NewScene()),
		"arch.excalidraw": string(excal.NewScene()),
	}
	for rel, c := range files {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(c), 0o644); err != nil {
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
	env := &ops.Env{Cfg: &config.Config{NotesDir: dir, Editor: "true"}, Store: st, Index: ix, Now: time.Now}
	if _, err := st.DB.Exec("INSERT INTO tags VALUES('arch.excalidraw','archive')"); err != nil {
		t.Fatal(err)
	}
	return &av{c: client.Local(env), dir: dir}
}

func (v *av) call(t *testing.T, op string, req, resp any) {
	t.Helper()
	if err := v.c.Call(context.Background(), op, req, resp); err != nil {
		t.Fatalf("%s: %v", op, err)
	}
}

func refIDs(rs []opnotes.Ref) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.ID)
	}
	slices.Sort(out)
	return out
}

func eq(t *testing.T, what string, got, want []string) {
	t.Helper()
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("%s: got %v want %v", what, got, want)
	}
}

func TestArchiveHidesByDefault(t *testing.T) {
	v := archiveVault(t)
	var rs []opnotes.Ref

	v.call(t, "notes.list", opnotes.ListReq{}, &rs)
	eq(t, "get", refIDs(rs), []string{"live.md", "solo.md"})
	v.call(t, "notes.list", opnotes.ListReq{All: true}, &rs)
	eq(t, "get --all", refIDs(rs), []string{"live.md", "lonely.md", "old.md", "solo.md"})

	v.call(t, "tags.notes", opnotes.TagsNotesReq{Tags: []string{"work"}}, &rs)
	eq(t, "tag work", refIDs(rs), []string{"live.md"})
	v.call(t, "tags.notes", opnotes.TagsNotesReq{Tags: []string{"work"}, All: true}, &rs)
	eq(t, "tag work --all", refIDs(rs), []string{"live.md", "old.md"})
	v.call(t, "tags.notes", opnotes.TagsNotesReq{Tags: []string{"archive"}}, &rs)
	eq(t, "tag archive", refIDs(rs), []string{"arch.excalidraw", "lonely.md", "old.md"})

	for _, id := range []string{"live.md", "old.md", "solo.md"} {
		v.call(t, "notes.visit", opnotes.IDReq{ID: id}, nil)
		v.call(t, "pins.add", opnotes.IDReq{ID: id}, nil)
	}
	v.call(t, "notes.recent", opnotes.RecentReq{}, &rs)
	eq(t, "recent", refIDs(rs), []string{"live.md", "solo.md"})
	v.call(t, "notes.recent", opnotes.RecentReq{All: true}, &rs)
	eq(t, "recent --all", refIDs(rs), []string{"live.md", "old.md", "solo.md"})
	v.call(t, "pins.list", opnotes.PinsListReq{}, &rs)
	eq(t, "pinned", refIDs(rs), []string{"live.md", "solo.md"})
	v.call(t, "pins.list", opnotes.PinsListReq{All: true}, &rs)
	eq(t, "pinned --all", refIDs(rs), []string{"live.md", "old.md", "solo.md"})
}

func TestArchiveHidesFromSearch(t *testing.T) {
	v := archiveVault(t)
	q := func(req opsearch.QueryReq) []string {
		var r opsearch.QueryResp
		v.call(t, "search.query", req, &r)
		var out []string
		for _, x := range r.Results {
			out = append(out, x.ID)
		}
		return out
	}
	eq(t, "-t", q(opsearch.QueryReq{Q: "zebra"}), []string{"live.md", "solo.md"})
	eq(t, "-t --all", q(opsearch.QueryReq{Q: "zebra", All: true}), []string{"live.md", "lonely.md", "old.md", "solo.md"})
	eq(t, "-t tag:archive", q(opsearch.QueryReq{Q: "zebra tag:archive"}), []string{"lonely.md", "old.md"})
	eq(t, "-t TAG:ARCHIVE only", q(opsearch.QueryReq{Q: "tag:ARCHIVE"}), []string{"arch.excalidraw", "lonely.md", "old.md"})
	eq(t, "-t tag:work", q(opsearch.QueryReq{Q: "tag:work"}), []string{"live.md"})
	eq(t, "-t -tag:work", q(opsearch.QueryReq{Q: "-tag:work"}), []string{"d.excalidraw", "solo.md"})

	f := func(req opsearch.FuzzyReq) []string {
		var r opsearch.FuzzyResp
		v.call(t, "search.fuzzy", req, &r)
		var out []string
		for _, x := range r.Results {
			out = append(out, x.ID)
		}
		return out
	}
	eq(t, "fuzzy", f(opsearch.FuzzyReq{Q: "lonely"}), nil)
	eq(t, "fuzzy --all", f(opsearch.FuzzyReq{Q: "lonely", All: true}), []string{"lonely.md"})
	eq(t, "fuzzy tag:archive", f(opsearch.FuzzyReq{Q: "tag:archive lonely"}), []string{"lonely.md"})

	var c opsearch.CorpusResp
	v.call(t, "search.corpus", opsearch.CorpusReq{}, &c)
	var cids []string
	for _, it := range c.Items {
		cids = append(cids, it.ID)
	}
	eq(t, "corpus", cids, []string{"d.excalidraw", "live.md", "solo.md"})
	v.call(t, "search.corpus", opsearch.CorpusReq{All: true}, &c)
	if len(c.Items) != 6 {
		t.Errorf("corpus --all: %d", len(c.Items))
	}

	eq(t, "diagram -t", q(opsearch.QueryReq{Q: "kind:diagram"}), []string{"d.excalidraw"})
	eq(t, "diagram -t --all", q(opsearch.QueryReq{Q: "kind:diagram", All: true}), []string{"arch.excalidraw", "d.excalidraw"})
}

func TestArchiveHidesFromTasksOrphansDiagramList(t *testing.T) {
	v := archiveVault(t)
	texts := func(ts []optasks.Task) []string {
		var out []string
		for _, x := range ts {
			out = append(out, x.Text)
		}
		return out
	}
	var ts []optasks.Task
	v.call(t, "tasks.list", optasks.ListReq{}, &ts)
	eq(t, "tasks", texts(ts), []string{"live task"})
	v.call(t, "tasks.list", optasks.ListReq{Archived: true}, &ts)
	eq(t, "tasks archived", texts(ts), []string{"live task", "old task"})
	v.call(t, "tasks.list", optasks.ListReq{Q: "tag:archive"}, &ts)
	eq(t, "tasks tag:archive", texts(ts), []string{"old task"})

	var os_ []opgraph.NoteRef
	ids := func() []string {
		var out []string
		for _, n := range os_ {
			out = append(out, n.ID)
		}
		return out
	}
	v.call(t, "graph.orphans", opgraph.OrphansReq{}, &os_)
	eq(t, "orphans", ids(), []string{"solo.md"})
	v.call(t, "graph.orphans", opgraph.OrphansReq{All: true}, &os_)
	eq(t, "orphans --all", ids(), []string{"lonely.md", "solo.md"})

	// links/backlinks show real structure: live links to the archived note.
	var bl []opgraph.BackOut
	v.call(t, "graph.backlinks", opgraph.IDReq{ID: "old.md"}, &bl)
	if len(bl) != 1 || bl[0].ID != "live.md" {
		t.Errorf("backlinks %+v", bl)
	}
	var info opnotes.InfoResp
	v.call(t, "notes.info", opnotes.IDReq{ID: "old.md"}, &info)
	var rr opnotes.Ref
	v.call(t, "notes.resolve", opnotes.ResolveReq{Ref: "old"}, &rr)
	if rr.ID != "old.md" {
		t.Errorf("resolve %+v", rr)
	}

	var dl []struct{ ID string }
	v.call(t, "diagram.list", map[string]any{}, &dl)
	if len(dl) != 1 || dl[0].ID != "d.excalidraw" {
		t.Errorf("diagram list %+v", dl)
	}
	v.call(t, "diagram.list", map[string]any{"all": true}, &dl)
	if len(dl) != 2 {
		t.Errorf("diagram list --all %+v", dl)
	}
}

func TestArchiveUnarchiveEditOnlyTags(t *testing.T) {
	v := archiveVault(t)
	before, _ := os.ReadFile(filepath.Join(v.dir, "live.md"))
	_ = before
	var changed []string
	v.call(t, "notes.archive", opnotes.IDsReq{IDs: []string{"live.md", "old.md"}}, &changed)
	eq(t, "archive changed", changed, []string{"live.md"})
	got, _ := os.ReadFile(filepath.Join(v.dir, "live.md"))
	if string(got) != "---\ntags:\n  - work\n  - archive\n---\nzebra live [[old]]\n- [ ] live task\n" {
		t.Errorf("archived content %q (was %q)", got, before)
	}
	var rs []opnotes.Ref
	v.call(t, "notes.list", opnotes.ListReq{}, &rs)
	eq(t, "hidden after archive", refIDs(rs), []string{"solo.md"})

	v.call(t, "notes.unarchive", opnotes.IDsReq{IDs: []string{"live.md", "old.md", "solo.md"}}, &changed)
	eq(t, "unarchive changed", changed, []string{"live.md", "old.md"})
	got, _ = os.ReadFile(filepath.Join(v.dir, "live.md"))
	if string(got) != "---\ntags:\n  - work\n---\nzebra live [[old]]\n- [ ] live task\n" {
		t.Errorf("unarchive: %q", got)
	}
	old, _ := os.ReadFile(filepath.Join(v.dir, "old.md"))
	if string(old) != "---\ntags:\n  - work\n---\nzebra old\n- [ ] old task\n" {
		t.Errorf("old: %q", old)
	}
}
