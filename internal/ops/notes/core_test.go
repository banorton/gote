package notes

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreatePrefixesSubdirAndConflict(t *testing.T) {
	h := newHarness(t)
	var r CreateResp
	h.must("notes.create", CreateReq{Name: "plain"}, &r)
	if r.ID != "plain.md" || !r.Created || r.Path != filepath.Join(h.dir, "plain.md") {
		t.Fatalf("%+v", r)
	}
	if got := h.read("plain.md"); got != "---\ncreated: \"260305140709\"\ntags: []\naliases: []\n---\n" {
		t.Fatalf("content %q", got)
	}
	h.must("notes.create", CreateReq{Name: "d", Prefix: "date"}, &r)
	if r.ID != "260305_d.md" {
		t.Fatal(r.ID)
	}
	h.must("notes.create", CreateReq{Name: "sub/dir/x", Prefix: "datetime"}, &r)
	if r.ID != "sub/dir/260305140709_x.md" || !h.exists(r.ID) {
		t.Fatal(r.ID)
	}
	h.wantCode("notes.create", CreateReq{Name: "plain"}, "conflict")
	h.wantCode("notes.create", CreateReq{Name: "../evil"}, "invalid")
	h.wantCode("notes.create", CreateReq{Name: "a", Prefix: "bogus"}, "invalid")
	if h.count("SELECT count(*) FROM notes WHERE id='plain.md'") != 1 {
		t.Fatal("create must index")
	}
}

func TestCreateTemplateMerge(t *testing.T) {
	h := newHarness(t)
	h.template("meet", "---\ntags: [meeting]\ncreated: \"old\"\nextra: 1\n---\n# Agenda\n- x\n")
	var r CreateResp
	h.must("notes.create", CreateReq{Name: "standup", Template: "meet"}, &r)
	got := h.read("standup.md")
	if got != "---\ntags: [meeting]\ncreated: \"260305140709\"\nextra: 1\naliases: []\n---\n# Agenda\n- x\n" {
		t.Fatalf("%q", got)
	}
	h.wantCode("notes.create", CreateReq{Name: "z", Template: "missing"}, "not_found")
}

func TestResolveReadInfoVisitRecent(t *testing.T) {
	h := newHarness(t)
	h.write("a.md", "---\ntags: [x, y]\naliases: [Ay]\ncreated: \"250101000000\"\n---\nbody [[b]]\n")
	h.write("b.md", "hello [[a]]\n")
	h.reconcile()
	var rr ResolveResp
	h.must("notes.resolve", ResolveReq{Ref: "ay"}, &rr)
	if rr.ID != "a.md" || rr.Kind != "note" || rr.Path != filepath.Join(h.dir, "a.md") || rr.Title != "a" {
		t.Fatalf("%+v", rr)
	}
	h.wantCode("notes.resolve", ResolveReq{Ref: "zzz"}, "not_found")
	var rd ReadResp
	h.must("notes.read", IDReq{ID: "b.md"}, &rd)
	if rd.Text != "hello [[a]]\n" || rd.MtimeNS == 0 || rd.Path != filepath.Join(h.dir, "b.md") {
		t.Fatalf("%+v", rd)
	}
	h.wantCode("notes.read", IDReq{ID: "nope.md"}, "not_found")
	h.wantCode("notes.read", IDReq{ID: "../x.md"}, "invalid")

}

func TestVisitRecentInfo(t *testing.T) {
	h := newHarness(t)
	h.write("a.md", "---\ntags: [x, y]\naliases: [Ay]\ncreated: \"250101000000\"\n---\nbody [[b]]\n")
	h.write("b.md", "hello [[a]]\n")
	h.reconcile()
	h.must("notes.visit", IDReq{ID: "a.md"}, nil)
	h.now = h.now.Add(time.Second)
	h.must("notes.visit", IDReq{ID: "b.md"}, nil)
	h.now = h.now.Add(time.Second)
	h.must("notes.visit", IDReq{ID: "a.md"}, nil)
	var rec []Ref
	h.must("notes.recent", RecentReq{}, &rec)
	if len(rec) != 2 || rec[0].ID != "a.md" || rec[1].ID != "b.md" {
		t.Fatalf("%+v", rec)
	}
	h.must("notes.recent", RecentReq{Limit: 1}, &rec)
	if len(rec) != 1 {
		t.Fatal(len(rec))
	}
	var rr ResolveResp
	h.must("notes.resolve", ResolveReq{Ref: "-"}, &rr)
	if rr.ID != "a.md" {
		t.Fatal(rr.ID)
	}
	h.must("pins.add", IDReq{ID: "a.md"}, nil)
	var info InfoResp
	h.must("notes.info", IDReq{ID: "a.md"}, &info)
	if info.Created != "250101000000" || !info.Pinned || info.Links != 1 || info.Backlinks != 1 ||
		strings.Join(info.Tags, ",") != "x,y" || strings.Join(info.Aliases, ",") != "Ay" || info.Size == 0 || len(info.Modified) != 12 {
		t.Fatalf("%+v", info)
	}
	var all []Ref
	h.must("notes.list", ListReq{}, &all)
	if len(all) != 2 {
		t.Fatal(len(all))
	}
}

func TestDuplicate(t *testing.T) {
	h := newHarness(t)
	h.write("a.md", "---\ncreated: \"250101000000\"\n---\nbody\n")
	h.reconcile()
	var r CreateResp
	h.must("notes.duplicate", DupReq{ID: "a.md", New: "b"}, &r)
	if r.ID != "b.md" || h.read("b.md") != "---\ncreated: \"260305140709\"\ntags: []\naliases: []\n---\nbody\n" {
		t.Fatalf("%+v %q", r, h.read("b.md"))
	}
	h.wantCode("notes.duplicate", DupReq{ID: "a.md", New: "b"}, "conflict")
}

func TestPins(t *testing.T) {
	h := newHarness(t)
	h.write("a.md", "x")
	h.reconcile()
	h.must("pins.add", IDReq{ID: "a.md"}, nil)
	h.must("pins.add", IDReq{ID: "a.md"}, nil)
	var l []Ref
	h.must("pins.list", nil, &l)
	if len(l) != 1 || l[0].ID != "a.md" {
		t.Fatalf("%+v", l)
	}
	h.must("pins.remove", IDReq{ID: "a.md"}, nil)
	h.must("pins.list", nil, &l)
	if len(l) != 0 {
		t.Fatal("not removed")
	}
	h.wantCode("pins.add", IDReq{ID: "ghost.md"}, "not_found")
}
