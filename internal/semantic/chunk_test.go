package semantic

import (
	"strings"
	"testing"

	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/parse"
)

func noteOf(id, src string) (note.Note, int) {
	n := parse.Note(id, []byte(src))
	return n, frontmatterLines(src)
}

func chunksOf(id, src string, exclude ...string) []Chunk {
	n, off := noteOf(id, src)
	return ChunkNote(n, off, exclude)
}

func TestChunkMixedHeadingsAndPreamble(t *testing.T) {
	src := "---\ntitle: T\ntags: [a, b]\naliases: [x, y]\n---\nintro text\n\n# One\nbody one\n## Sub\nbody sub\n### Deep\ndeep body\n# Two\nbody two\n"
	cs := chunksOf("t.md", src)
	type want struct {
		path string
		line int
		text string
	}
	ws := []want{
		{"", 6, "intro text"},
		{"One", 8, "body one"},
		{"One > Sub", 10, "body sub"},
		{"One > Sub > Deep", 12, "deep body"},
		{"Two", 14, "body two"},
	}
	if len(cs) != len(ws) {
		t.Fatalf("%d chunks: %+v", len(cs), cs)
	}
	for i, w := range ws {
		c := cs[i]
		if c.HeadingPath != w.path || c.StartLine != w.line || c.Text != w.text || c.Ord != i {
			t.Errorf("chunk %d: %+v want %+v", i, c, w)
		}
	}
}

func TestChunkHeaderComposition(t *testing.T) {
	cs := chunksOf("t.md", "---\ntitle: T\ntags: [a, b]\naliases: [x, y]\n---\n# One\n## Sub\nhello\n")
	if len(cs) != 1 {
		t.Fatalf("%+v", cs)
	}
	want := "Title: T\nSection: One > Sub\nTags: a, b\nAliases: x, y\n\nhello"
	if cs[0].Embed != want {
		t.Fatalf("%q", cs[0].Embed)
	}
	cs = chunksOf("plain.md", "just text\n")
	if cs[0].Embed != "Title: plain\n\njust text" {
		t.Fatalf("%q", cs[0].Embed)
	}
}

func TestChunkEmptyDropped(t *testing.T) {
	cs := chunksOf("t.md", "# A\n\n# B\ntext\n# C\n   \n")
	if len(cs) != 1 || cs[0].HeadingPath != "B" {
		t.Fatalf("%+v", cs)
	}
}

func TestChunkHeadingInCodeFenceIgnored(t *testing.T) {
	cs := chunksOf("t.md", "# A\n```\n# not heading\n```\n")
	if len(cs) != 1 || !strings.Contains(cs[0].Text, "# not heading") {
		t.Fatalf("%+v", cs)
	}
}

func TestChunkOversizeSplit(t *testing.T) {
	para := strings.Repeat("a", 1500)
	cs := chunksOf("t.md", "# Big\n"+para+"\n\n"+para+"\n\n"+strings.Repeat("b", 4500)+"\n")
	for _, c := range cs {
		if len([]rune(c.Text)) > maxChunk {
			t.Fatalf("chunk too big: %d", len(c.Text))
		}
		if c.HeadingPath != "Big" {
			t.Fatalf("%+v", c)
		}
	}
	if len(cs) != 5 {
		t.Fatalf("want 5 chunks (2 paragraphs, 3 hard splits), got %d", len(cs))
	}
	if cs[0].StartLine != 1 || cs[1].StartLine != 4 || cs[2].StartLine != 6 {
		t.Fatalf("lines %d %d %d", cs[0].StartLine, cs[1].StartLine, cs[2].StartLine)
	}
}

func TestChunkExclusions(t *testing.T) {
	if cs := chunksOf("t.md", "---\nsemantic: false\n---\n# A\ntext\n"); len(cs) != 0 {
		t.Fatal("semantic:false not excluded")
	}
	if cs := chunksOf("t.md", "---\ntags: [Private, x]\n---\ntext\n", "private"); len(cs) != 0 {
		t.Fatal("exclude tag not honoured")
	}
	if cs := chunksOf("t.md", "---\ntags: [x]\n---\ntext\n", "private"); len(cs) != 1 {
		t.Fatal("unexpected exclusion")
	}
}

func TestChunkDiagram(t *testing.T) {
	n := note.Note{ID: "d.excalidraw", Kind: note.KindDiagram, Title: "d", Semantic: true, Body: "box one\nbox two"}
	cs := ChunkNote(n, 0, []string{"private"})
	if len(cs) != 1 || cs[0].StartLine != 0 || cs[0].Text != "box one\nbox two" || cs[0].Embed != "Title: d\n\nbox one\nbox two" {
		t.Fatalf("%+v", cs)
	}
	n.Body = ""
	if len(ChunkNote(n, 0, nil)) != 0 {
		t.Fatal("empty diagram chunked")
	}
}

func hashes(cs []Chunk) []string {
	var h []string
	for _, c := range cs {
		h = append(h, c.Hash)
	}
	return h
}

func TestChunkHashStability(t *testing.T) {
	base := "---\ntags: [a]\n---\n# One\nfirst\n# Two\nsecond\n"
	h0 := hashes(chunksOf("t.md", base))
	h1 := hashes(chunksOf("t.md", strings.Replace(base, "first", "first edited", 1)))
	if h0[0] == h1[0] || h0[1] != h1[1] {
		t.Fatalf("edit should change only chunk 0: %v %v", h0, h1)
	}
	h2 := hashes(chunksOf("t.md", strings.Replace(base, "[a]", "[a, c]", 1)))
	for i := range h0 {
		if h0[i] == h2[i] {
			t.Fatalf("tag change must change every hash: %v %v", h0, h2)
		}
	}
}
