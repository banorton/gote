package parse

import (
	"reflect"
	"testing"
	"time"

	"github.com/banorton/gote/internal/note"
)

func TestFrontmatterVariants(t *testing.T) {
	cases := []struct {
		name, src string
		tags      []string
		created   time.Time
		title     string
		errSet    bool
		sem       bool
	}{
		{"none", "# Hi\n", nil, time.Time{}, "My Note", false, true},
		{"empty", "---\n---\nbody", nil, time.Time{}, "My Note", false, true},
		{"invalid", "---\ntags: [\n---\nbody [[x]]", nil, time.Time{}, "My Note", true, true},
		{"tags string", "---\ntags: solo\n---\n", []string{"solo"}, time.Time{}, "My Note", false, true},
		{"tags list", "---\ntags: [a, b]\n---\n", []string{"a", "b"}, time.Time{}, "My Note", false, true},
		{"tags block", "---\ntags:\n  - a\n  - b\n---\n", []string{"a", "b"}, time.Time{}, "My Note", false, true},
		{"created compact int is not a date", "---\ncreated: 260102030405\n---\n", nil, time.Time{}, "My Note", true, true},
		{"created compact string is not a date", "---\ncreated: \"260102030405\"\n---\n", nil, time.Time{}, "My Note", true, true},
		{"created junk", "---\ncreated: old\n---\n", nil, time.Time{}, "My Note", true, true},
		{"created iso offset", "---\ncreated: \"2026-11-27T14:30:00-05:00\"\n---\n", nil, time.Date(2026, 11, 27, 14, 30, 0, 0, time.FixedZone("", -5*3600)), "My Note", false, true},
		{"created iso date", "---\ncreated: 2026-11-27\n---\n", nil, time.Date(2026, 11, 27, 0, 0, 0, 0, time.Local), "My Note", false, true},
		{"created iso local", "---\ncreated: \"2026-11-27T14:30:00\"\n---\n", nil, time.Date(2026, 11, 27, 14, 30, 0, 0, time.Local), "My Note", false, true},
		{"title", "---\ntitle: Custom\n---\n", nil, time.Time{}, "Custom", false, true},
		{"semantic false", "---\nsemantic: false\n---\n", nil, time.Time{}, "My Note", false, false},
	}
	for _, c := range cases {
		n := Note("sub/My Note.md", []byte(c.src))
		if !reflect.DeepEqual(n.Tags, c.tags) || !n.Created.Equal(c.created) || n.Title != c.title || (n.ParseErr != "") != c.errSet || n.Semantic != c.sem {
			t.Errorf("%s: %+v", c.name, n)
		}
		if n.Stem != "my note" || n.Kind != note.KindNote || n.ID != "sub/My Note.md" {
			t.Errorf("%s: id/stem/kind %+v", c.name, n)
		}
	}
}

func TestInvalidYAMLStillIndexesBody(t *testing.T) {
	n := Note("a.md", []byte("---\ntags: [\n---\nbody [[x]]\n"))
	if n.ParseErr == "" || len(n.Links) != 1 || n.Links[0].Target != "x" || len(n.Fields) != 0 {
		t.Fatalf("%+v", n)
	}
	if n.Links[0].Line != 4 {
		t.Fatalf("line %d", n.Links[0].Line)
	}
}

func TestAliasesAndFields(t *testing.T) {
	n := Note("a.md", []byte("---\naliases: [Foo, Bar]\nstatus: draft\n---\n"))
	if !reflect.DeepEqual(n.Aliases, []string{"Foo", "Bar"}) || n.Fields["status"] != "draft" {
		t.Fatalf("%+v", n)
	}
}

func TestLinks(t *testing.T) {
	src := "intro [[plain]] and [[dir/target|shown]] and [[note#Head]] and [[n#h|al]]\n" +
		"![[embedded]] ![[img#h]]\n" +
		"[rel](other/file.md) [frag](x.md#sec) [url](https://e.com/a.md) [mail](mailto:a@b.c) [anchor](#top) [pic](a.png)\n" +
		"`[[incode]]` text\n\n```\n[[block]]\n[x](c.md)\n```\n"
	n := Note("a.md", []byte(src))
	want := []note.Link{
		{Target: "plain", Kind: "wiki", Line: 1},
		{Target: "dir/target", Alias: "shown", Kind: "wiki", Line: 1},
		{Target: "note", Fragment: "Head", Kind: "wiki", Line: 1},
		{Target: "n", Fragment: "h", Alias: "al", Kind: "wiki", Line: 1},
		{Target: "embedded", Kind: "embed", Line: 2},
		{Target: "img", Fragment: "h", Kind: "embed", Line: 2},
		{Target: "other/file.md", Kind: "md", Line: 3},
		{Target: "x.md", Fragment: "sec", Kind: "md", Line: 3},
	}
	if !reflect.DeepEqual(n.Links, want) {
		t.Fatalf("got %+v", n.Links)
	}
}

func TestRelatedParent(t *testing.T) {
	n := Note("a.md", []byte("---\nrelated: [[[x]], y]\nparent: \"[[p]]\"\n---\n"))
	var got []string
	for _, l := range n.Links {
		got = append(got, l.Kind+":"+l.Target)
	}
	if !reflect.DeepEqual(got, []string{"related:x", "related:y", "parent:p"}) {
		t.Fatalf("%v", got)
	}
	n = Note("a.md", []byte("---\nrelated:\n  - [[x]]\n  - z\n---\n"))
	if len(n.Links) != 2 || n.Links[0].Target != "x" || n.Links[1].Target != "z" {
		t.Fatalf("%+v", n.Links)
	}
}

func TestHeadingsTasksLines(t *testing.T) {
	src := "---\ntitle: T\n---\n# One\ntext\n\n## Two *em*\n\n- [ ] todo\n- [x] done\n- plain\n\nSetext\n======\n"
	n := Note("a.md", []byte(src))
	wantH := []note.Heading{{1, "One", 4}, {2, "Two *em*", 7}, {1, "Setext", 13}}
	if !reflect.DeepEqual(n.Headings, wantH) {
		t.Fatalf("headings %+v", n.Headings)
	}
	wantT := []note.Task{{9, "todo", false}, {10, "done", true}}
	if !reflect.DeepEqual(n.Tasks, wantT) {
		t.Fatalf("tasks %+v", n.Tasks)
	}
	if n.Body != "# One\ntext\n\n## Two *em*\n\n- [ ] todo\n- [x] done\n- plain\n\nSetext\n======\n" {
		t.Fatalf("body %q", n.Body)
	}
}

func TestLineNumbersWithoutFrontmatter(t *testing.T) {
	n := Note("a.md", []byte("a\n\n[[x]]\n"))
	if n.Links[0].Line != 3 {
		t.Fatalf("%+v", n.Links)
	}
}
