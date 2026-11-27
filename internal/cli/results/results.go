// Package results renders search hits as readable text: a bold title, a dim
// path:line (and score), and one snippet line trimmed to the terminal width
// with the matched terms highlighted. The same Row feeds the plain list and
// the menu entry, so both read alike. Without a terminal there are no escape
// codes.
package results

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"

	"github.com/banorton/gote/internal/query"
	"github.com/banorton/gote/internal/tui/menu"
)

const (
	bold      = "\x1b[1m"
	dim       = "\x1b[2m"
	hilite    = "\x1b[1;33m"
	reset     = "\x1b[0m"
	listWidth = 100
	indent    = "    "
	menuPad   = 5
)

type Row struct {
	ID, Title string
	Line      int
	Score     float64
	HasScore  bool
	Snippet   string
}

type Style struct {
	Color bool
	// Width is the terminal width the snippet line is trimmed to.
	Width int
	// Terms are highlighted in the snippet (case-insensitive substrings).
	Terms []string
	// FirstLine shows only the first non-empty snippet line, for semantic
	// results whose snippet is a chunk of text with no matched terms.
	FirstLine bool
}

// IsTTY reports whether w is a terminal; tests replace it.
var IsTTY = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// StyleFor picks color and width for w: escapes and the real terminal width on
// a terminal, plain text at a fixed width otherwise.
func StyleFor(w io.Writer, terms []string, firstLine bool) Style {
	st := Style{Width: listWidth, Terms: terms, FirstLine: firstLine}
	if IsTTY(w) {
		st.Color = os.Getenv("NO_COLOR") == ""
		if f, ok := w.(*os.File); ok {
			if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 20 {
				st.Width = cols
			}
		}
	}
	return st
}

// Terms extracts the words and phrases a query matches on, for highlighting.
func Terms(q string) []string {
	pq, err := query.Parse(q)
	if err != nil {
		return strings.Fields(q)
	}
	var out []string
	for _, t := range pq.Terms {
		if !t.Neg {
			out = append(out, t.Text)
		}
	}
	return out
}

func (s Style) wrap(code, text string) string {
	if !s.Color || text == "" {
		return text
	}
	return code + text + reset
}

func where(r Row) string {
	if r.Line > 0 {
		return fmt.Sprintf("%s:%d", r.ID, r.Line)
	}
	return r.ID
}

func (s Style) meta(r Row) string {
	m := where(r)
	if r.HasScore {
		m += fmt.Sprintf("  %.2f", r.Score)
	}
	return m
}

// Head is the first line of a plain-list entry: title, then dim location.
func (s Style) Head(r Row) string {
	return s.wrap(bold, r.Title) + "  " + s.wrap(dim, s.meta(r))
}

// Snippet is the one-line snippet for a plain-list entry.
func (s Style) Snippet(r Row) string { return s.snippet(r, s.Width-len(indent)) }

func (s Style) snippet(r Row, width int) string {
	text := r.Snippet
	if s.FirstLine {
		text = ""
		for _, l := range strings.Split(r.Snippet, "\n") {
			if strings.TrimSpace(l) != "" {
				text = l
				break
			}
		}
	}
	text = strings.Join(strings.Fields(text), " ")
	if rs := []rune(text); width > 1 && len(rs) > width {
		text = string(rs[:width-1]) + "…"
	}
	if !s.Color || len(s.Terms) == 0 {
		return text
	}
	quoted := make([]string, 0, len(s.Terms))
	for _, t := range s.Terms {
		if t != "" {
			quoted = append(quoted, regexp.QuoteMeta(t))
		}
	}
	if len(quoted) == 0 {
		return text
	}
	re := regexp.MustCompile("(?i)" + strings.Join(quoted, "|"))
	return re.ReplaceAllStringFunc(text, func(m string) string { return hilite + m + reset })
}

// Print writes the plain list: head line, then the snippet line when present.
func Print(w io.Writer, rows []Row, s Style) {
	for _, r := range rows {
		fmt.Fprintln(w, s.Head(r))
		if sn := s.Snippet(r); sn != "" {
			fmt.Fprintln(w, indent+sn)
		}
	}
}

// MenuItem is the menu entry for r. Detail carries the dim location on its
// first line and the snippet on a second; the menu indents the second line.
func MenuItem(r Row, s Style) menu.Item {
	detail := s.wrap(dim, s.meta(r))
	if sn := s.snippet(r, s.Width-menuPad); sn != "" {
		detail += "\n" + sn
	}
	return menu.Item{ID: r.ID, Title: s.wrap(bold, r.Title), Detail: detail}
}
