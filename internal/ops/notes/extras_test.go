package notes

import (
	"errors"
	"github.com/banorton/gote/internal/ops"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuickOpenAndSave(t *testing.T) {
	h := newHarness(t)
	var q PathResp
	h.must("quick.open", nil, &q)
	if q.Path != filepath.Join(h.dir, ".quick.md") || !h.exists(".quick.md") {
		t.Fatalf("%+v", q)
	}
	h.wantCode("quick.save", QuickSaveReq{Name: "saved"}, "invalid")
	h.write(".quick.md", "remember this\n")
	var r CreateResp
	h.must("quick.save", QuickSaveReq{Name: "saved"}, &r)
	if r.ID != "saved.md" || h.read("saved.md") != "---\ncreated: \""+h.iso()+"\"\ntags: []\naliases: []\n---\nremember this\n" {
		t.Fatalf("%+v %q", r, h.read("saved.md"))
	}
	if h.read(".quick.md") != "" {
		t.Fatal("quick not truncated")
	}
	h.write(".quick.md", "again\n")
	h.wantCode("quick.save", QuickSaveReq{Name: "saved"}, "conflict")
	if h.read(".quick.md") != "again\n" {
		t.Fatal("quick must survive a failed save")
	}
}

func TestDailyToday(t *testing.T) {
	h := newHarness(t)
	var r CreateResp
	h.must("daily.open", DailyReq{}, &r)
	want := "---\ncreated: \"" + h.iso() + "\"\ntags:\n  - daily\naliases: []\n---\n"
	if r.ID != "daily/260305_daily.md" || !r.Created || h.read(r.ID) != want {
		t.Fatalf("%+v %q", r, h.read(r.ID))
	}
	h.must("daily.open", DailyReq{}, &r)
	if r.Created || h.read(r.ID) != want {
		t.Fatal("second open must reuse")
	}
}

func TestDailyPastOpensExistingNeverCreates(t *testing.T) {
	h := newHarness(t)
	h.write("daily/260304_daily.md", "---\ntags: [daily]\n---\nyesterday\n")
	var r CreateResp
	h.must("daily.open", DailyReq{Offset: -1}, &r)
	if r.ID != "daily/260304_daily.md" || r.Created {
		t.Fatalf("%+v", r)
	}
	err := h.call("daily.open", DailyReq{Offset: -2}, &r)
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeNotFound || !strings.Contains(oe.Msg, "no daily note for 2026-03-03") {
		t.Fatalf("%v", err)
	}
	if h.exists("daily/260303_daily.md") {
		t.Fatal("past note was created")
	}
	if err := h.call("daily.open", DailyReq{Offset: 1}, &r); err == nil || h.exists("daily/260306_daily.md") {
		t.Fatalf("future note: %v", err)
	}
}

func TestDailyTemplateMergeKeepsDailyTag(t *testing.T) {
	h := newHarness(t)
	h.template("daily", "---\ntags: [daily]\nmood: ok\n---\n## Log\n")
	var r CreateResp
	h.must("daily.open", DailyReq{}, &r)
	if got, want := h.read(r.ID), "---\ntags: [daily]\nmood: ok\ncreated: \""+h.iso()+"\"\naliases: []\n---\n## Log\n"; got != want {
		t.Fatalf("%q", got)
	}
	h.now = h.now.AddDate(0, 0, 1)
	h.template("daily", "---\ntags: [journal, work]\n---\nbody\n")
	h.must("daily.open", DailyReq{}, &r)
	if got := h.read(r.ID); !strings.Contains(got, "journal") || !strings.Contains(got, "daily") || !strings.Contains(got, "work") {
		t.Fatalf("daily tag missing: %q", got)
	}
	h.now = h.now.AddDate(0, 0, 1)
	h.template("daily", "# no frontmatter\n")
	h.must("daily.open", DailyReq{}, &r)
	if got := h.read(r.ID); !strings.HasPrefix(got, "---\ncreated:") || !strings.Contains(got, "- daily") || !strings.HasSuffix(got, "# no frontmatter\n") {
		t.Fatalf("%q", got)
	}
}

func TestTemplates(t *testing.T) {
	h := newHarness(t)
	var l []string
	h.must("templates.list", nil, &l)
	if len(l) != 0 {
		t.Fatal(l)
	}
	h.template("b", "B")
	h.template("a", "A")
	h.must("templates.list", nil, &l)
	if len(l) != 2 || l[0] != "a" || l[1] != "b" {
		t.Fatal(l)
	}
	var rd TemplateResp
	h.must("templates.read", TemplateReq{Name: "a"}, &rd)
	if rd.Text != "A" || rd.Path != filepath.Join(h.cfg, "gote", "templates", "a.md") {
		t.Fatalf("%+v", rd)
	}
	h.must("templates.rename", TemplateRenameReq{Name: "a", New: "c"}, nil)
	h.wantCode("templates.rename", TemplateRenameReq{Name: "c", New: "b"}, "conflict")
	h.wantCode("templates.read", TemplateReq{Name: "a"}, "not_found")
	h.must("templates.delete", TemplateReq{Name: "c"}, nil)
	h.wantCode("templates.delete", TemplateReq{Name: "c"}, "not_found")
	var e TemplateResp
	h.must("templates.ensure", TemplateReq{Name: "fresh"}, &e)
	if _, err := os.Stat(e.Path); err != nil {
		t.Fatal(err)
	}
	h.wantCode("templates.read", TemplateReq{Name: "../x"}, "invalid")
}

func TestExportImportRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.write("a.md", "alpha [[b]]\n")
	h.write("d/b.md", "beta\n")
	h.write(".trash/old.md", "old\n")
	h.template("tpl", "T")
	h.reconcile()
	h.must("pins.add", IDReq{ID: "d/b.md"}, nil)
	arc := filepath.Join(t.TempDir(), "out.tar.gz")
	var ex ExportResp
	h.must("export", ExportReq{Path: arc}, &ex)
	if ex.Files < 4 {
		t.Fatalf("%+v", ex)
	}

	h2 := newHarness(t)
	h2.write("junk.md", "x")
	h2.wantCode("import", ImportReq{Path: arc}, "conflict")
	os.Remove(filepath.Join(h2.dir, "junk.md"))
	h2.must("import", ImportReq{Path: arc}, nil)
	for f, want := range map[string]string{"a.md": "alpha [[b]]\n", "d/b.md": "beta\n", ".trash/old.md": "old\n"} {
		if h2.read(f) != want {
			t.Fatalf("%s: %q", f, h2.read(f))
		}
	}
	b, err := os.ReadFile(filepath.Join(h2.cfg, "gote", "templates", "tpl.md"))
	if err != nil || string(b) != "T" {
		t.Fatalf("template: %q %v", b, err)
	}
	if h2.count("SELECT count(*) FROM notes") != 2 || h2.count("SELECT count(*) FROM pins WHERE note_id='d/b.md'") != 1 {
		t.Fatal("index/pins not restored")
	}
	h2.write("more.md", "x")
	h2.must("import", ImportReq{Path: arc, Force: true}, nil)
}
