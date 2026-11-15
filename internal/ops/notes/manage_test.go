package notes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func renameFixture(h *harness) {
	h.write("dir/a.md", "---\ntags: [t]\n---\nself [[a]]\n")
	h.write("one.md", "[[a]] and [[a|shown]] and [[a#head]] and ![[a]] and [[dir/a]] and [t](dir/a.md) and [u](dir/a.md#frag)\n")
	h.write("two.md", "alias [[Ay]]\n```\n[[a]]\n```\nand `[[a]]` inline\n")
	h.write("dir/three.md", "[rel](a.md) [[A]]\n")
	h.write("keep.md", "nothing here\n")
	h.write("aa.md", "---\naliases: [Ay]\n---\n")
	h.reconcile()
}

func TestRenameRewritesInboundLinks(t *testing.T) {
	h := newHarness(t)
	renameFixture(h)
	h.must("pins.add", IDReq{ID: "dir/a.md"}, nil)
	h.must("notes.visit", IDReq{ID: "dir/a.md"}, nil)
	var r RenameResp
	h.must("notes.rename", RenameReq{ID: "dir/a.md", New: "b"}, &r)
	if r.ID != "dir/b.md" || r.Path != filepath.Join(h.dir, "dir/b.md") || h.exists("dir/a.md") || !h.exists("dir/b.md") {
		t.Fatalf("%+v", r)
	}
	want := "[[b]] and [[b|shown]] and [[b#head]] and ![[b]] and [[dir/b]] and [t](dir/b.md) and [u](dir/b.md#frag)\n"
	if got := h.read("one.md"); got != want {
		t.Fatalf("one.md:\n%q\nwant\n%q", got, want)
	}
	if got := h.read("two.md"); got != "alias [[Ay]]\n```\n[[a]]\n```\nand `[[a]]` inline\n" {
		t.Fatalf("two.md must keep alias links and code untouched: %q", got)
	}
	if got := h.read("dir/three.md"); got != "[rel](b.md) [[b]]\n" {
		t.Fatalf("three.md: %q", got)
	}
	if got := h.read("dir/b.md"); got != "---\ntags: [t]\n---\nself [[b]]\n" {
		t.Fatalf("self: %q", got)
	}
	sort.Strings(r.Rewritten)
	if strings.Join(r.Rewritten, ",") != "dir/b.md,dir/three.md,one.md" {
		t.Fatalf("rewritten %v", r.Rewritten)
	}
	if h.count("SELECT count(*) FROM pins WHERE note_id='dir/b.md'") != 1 || h.count("SELECT count(*) FROM pins WHERE note_id='dir/a.md'") != 0 {
		t.Fatal("pins not moved")
	}
	if h.count("SELECT count(*) FROM visits WHERE note_id='dir/b.md'") != 1 {
		t.Fatal("visits not moved")
	}
	if h.count("SELECT count(*) FROM links WHERE dst_id='dir/b.md'") < 6 || h.count("SELECT count(*) FROM notes WHERE id='dir/a.md'") != 0 {
		t.Fatal("index not updated")
	}
}

func TestRenameMovesDirectoryWithSlash(t *testing.T) {
	h := newHarness(t)
	h.write("a.md", "x\n")
	h.write("o.md", "[[a]] [t](a.md)\n")
	h.reconcile()
	h.must("notes.rename", RenameReq{ID: "a.md", New: "sub/b"}, nil)
	if !h.exists("sub/b.md") || h.read("o.md") != "[[b]] [t](sub/b.md)\n" {
		t.Fatalf("%q", h.read("o.md"))
	}
	h.wantCode("notes.rename", RenameReq{ID: "ghost.md", New: "o"}, "not_found")
	h.write("sub/o.md", "x")
	h.reconcile()
	h.wantCode("notes.rename", RenameReq{ID: "sub/b.md", New: "o"}, "conflict")
}

func TestRenameConflictChangesNothing(t *testing.T) {
	h := newHarness(t)
	renameFixture(h)
	before := map[string]string{}
	for _, f := range []string{"one.md", "dir/three.md", "dir/a.md"} {
		before[f] = h.read(f)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(h.dir, "dir/three.md"), future, future); err != nil {
		t.Fatal(err)
	}
	h.wantCode("notes.rename", RenameReq{ID: "dir/a.md", New: "b"}, "conflict")
	for f, c := range before {
		if h.read(f) != c {
			t.Fatalf("%s changed", f)
		}
	}
	if !h.exists("dir/a.md") || h.exists("dir/b.md") {
		t.Fatal("file moved")
	}
}

func TestDeleteTrashRecoverRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.write("d/x.md", "one\n")
	h.reconcile()
	h.must("pins.add", IDReq{ID: "d/x.md"}, nil)
	h.must("notes.visit", IDReq{ID: "d/x.md"}, nil)
	var del DeleteResp
	h.must("notes.delete", IDReq{ID: "d/x.md"}, &del)
	if del.TrashPath != filepath.Join(h.dir, ".trash/d/x.md") || h.exists("d/x.md") || !h.exists(".trash/d/x.md") {
		t.Fatalf("%+v", del)
	}
	if h.count("SELECT count(*) FROM notes WHERE id='d/x.md'") != 0 || h.count("SELECT count(*) FROM pins")+h.count("SELECT count(*) FROM visits") != 0 {
		t.Fatal("rows not removed")
	}
	h.write("d/x.md", "two\n")
	h.reconcile()
	h.must("notes.delete", IDReq{ID: "d/x.md"}, &del)
	if !strings.HasPrefix(filepath.Base(del.TrashPath), "x_260305140709") || filepath.Ext(del.TrashPath) != ".md" {
		t.Fatalf("collision name: %s", del.TrashPath)
	}
	var tr []TrashEntry
	h.must("notes.trash", nil, &tr)
	if len(tr) != 2 || tr[0].OriginalID != "d/x.md" || tr[1].OriginalID != "d/x.md" {
		t.Fatalf("%+v", tr)
	}
	var rec RecoverResp
	h.must("notes.recover", RecoverReq{OriginalID: "d/x.md"}, &rec)
	if rec.ID != "d/x.md" || h.read("d/x.md") != "two\n" {
		t.Fatalf("recover newest: %+v %q", rec, h.read("d/x.md"))
	}
	h.wantCode("notes.recover", RecoverReq{TrashPath: filepath.Join(h.dir, ".trash/d/x.md")}, "conflict")
	h.must("notes.trash", nil, &tr)
	if len(tr) != 1 {
		t.Fatalf("%+v", tr)
	}
	if h.count("SELECT count(*) FROM notes WHERE id='d/x.md'") != 1 {
		t.Fatal("recover must index")
	}
	h.wantCode("notes.recover", RecoverReq{OriginalID: "nope.md"}, "not_found")
	var emp TrashEmptyResp
	h.must("notes.trash_empty", nil, &emp)
	if emp.Removed != 1 || h.exists(".trash/d/x.md") {
		t.Fatalf("%+v", emp)
	}
}

func TestTagsAddRemoveEditOnlyTags(t *testing.T) {
	h := newHarness(t)
	fm := "---\n# keep me\ntitle: \"T\"\ntags: [a, b]\nzzz:   odd   spacing\n---\nbody\n"
	h.write("n.md", fm)
	h.write("bare.md", "no frontmatter\n")
	h.write("single.md", "---\ntags: solo\n---\n")
	h.reconcile()
	var changed []string
	h.must("tags.add", TagsEditReq{Tag: "c", IDs: []string{"n.md", "bare.md", "single.md"}}, &changed)
	sort.Strings(changed)
	if len(changed) != 3 {
		t.Fatal(changed)
	}
	if got := h.read("n.md"); got != "---\n# keep me\ntitle: \"T\"\ntags:\n  - a\n  - b\n  - c\nzzz:   odd   spacing\n---\nbody\n" {
		t.Fatalf("%q", got)
	}
	if h.count("SELECT count(*) FROM tags WHERE note_id='n.md' AND tag='c'") != 1 {
		t.Fatal("index not updated")
	}
	h.must("tags.add", TagsEditReq{Tag: "c", IDs: []string{"n.md"}}, &changed)
	if len(changed) != 0 {
		t.Fatalf("dedupe: %v", changed)
	}
	h.must("tags.remove", TagsEditReq{Tag: "a", IDs: []string{"n.md", "bare.md"}}, &changed)
	if len(changed) != 1 || changed[0] != "n.md" {
		t.Fatal(changed)
	}
	if got := h.read("n.md"); got != "---\n# keep me\ntitle: \"T\"\ntags:\n  - b\n  - c\nzzz:   odd   spacing\n---\nbody\n" {
		t.Fatalf("%q", got)
	}
	var tl []TagCount
	h.must("tags.list", nil, &tl)
	if len(tl) == 0 || tl[0].Count < tl[len(tl)-1].Count {
		t.Fatalf("%+v", tl)
	}
	var nl []Ref
	h.must("tags.notes", TagsNotesReq{Tags: []string{"b", "c"}}, &nl)
	if len(nl) != 1 || nl[0].ID != "n.md" {
		t.Fatalf("%+v", nl)
	}
}
