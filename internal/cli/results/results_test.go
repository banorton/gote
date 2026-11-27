package results

import (
	"bytes"
	"strings"
	"testing"
)

var rows = []Row{
	{ID: "projects/gote.md", Title: "Gote", Line: 12, Snippet: "the unix socket\nserver speaks JSON", HasScore: false},
	{ID: "ideas.md", Title: "Ideas", Snippet: ""},
	{ID: "d.excalidraw", Title: "Diagram", Line: 3, Score: 0.8249, HasScore: true, Snippet: "first line\nsecond line"},
}

func TestPrintPlainGolden(t *testing.T) {
	var b bytes.Buffer
	Print(&b, rows[:2], Style{Width: 80, Terms: []string{"socket"}})
	want := "Gote  projects/gote.md:12\n    the unix socket server speaks JSON\nIdeas  ideas.md\n"
	if b.String() != want {
		t.Fatalf("got %q want %q", b.String(), want)
	}
	if strings.Contains(b.String(), "\x1b") {
		t.Fatal("ANSI in plain output")
	}
}

func TestPrintSemanticScoreAndFirstLine(t *testing.T) {
	var b bytes.Buffer
	Print(&b, rows[2:], Style{Width: 80, FirstLine: true})
	if want := "Diagram  d.excalidraw:3  0.82\n    first line\n"; b.String() != want {
		t.Fatalf("got %q", b.String())
	}
}

func TestSnippetTrimmedToWidth(t *testing.T) {
	r := Row{ID: "a.md", Title: "A", Snippet: strings.Repeat("word ", 40)}
	var b bytes.Buffer
	Print(&b, []Row{r}, Style{Width: 30})
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	if got := []rune(lines[1]); len(got) != 30 || got[29] != '…' {
		t.Fatalf("%d %q", len(got), lines[1])
	}
}

func TestColorStyleAndHighlight(t *testing.T) {
	st := Style{Color: true, Width: 80, Terms: []string{"socket"}}
	head, snip := st.Head(rows[0]), st.Snippet(rows[0])
	if !strings.HasPrefix(head, "\x1b[1mGote\x1b[0m  \x1b[2mprojects/gote.md:12\x1b[0m") {
		t.Fatalf("%q", head)
	}
	if !strings.Contains(snip, "\x1b[1;33msocket\x1b[0m") {
		t.Fatalf("%q", snip)
	}
	if strings.Contains((Style{Width: 80, Terms: []string{"socket"}}).Snippet(rows[0]), "\x1b") {
		t.Fatal("highlight without color")
	}
}

func TestHighlightCaseInsensitiveAndAfterTrim(t *testing.T) {
	st := Style{Color: true, Width: 20, Terms: []string{"json", "x.y"}}
	s := st.Snippet(Row{Snippet: "speaks JSON over a socket right here"})
	plain := stripANSI(s)
	if len([]rune(plain)) != 20-4 || !strings.Contains(s, "\x1b[1;33mJSON\x1b[0m") {
		t.Fatalf("%q", s)
	}
}

func TestTermsFromQuery(t *testing.T) {
	got := Terms(`foo "two words" -neg tag:x created:2026`, "iso")
	if strings.Join(got, "|") != "foo|two words" {
		t.Fatalf("%q", got)
	}
	if got := Terms(`"unterminated`, "iso"); len(got) != 1 {
		t.Fatalf("%q", got)
	}
}

func TestMenuItem(t *testing.T) {
	st := Style{Color: false, Width: 80, Terms: []string{"socket"}}
	it := MenuItem(rows[0], st)
	if it.ID != "projects/gote.md" || it.Title != "Gote" || it.Detail != "projects/gote.md:12\nthe unix socket server speaks JSON" {
		t.Fatalf("%+v", it)
	}
	it = MenuItem(rows[1], st)
	if it.Detail != "ideas.md" {
		t.Fatalf("%+v", it)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
