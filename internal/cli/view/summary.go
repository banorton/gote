package view

import (
	"fmt"
	"io"
	"strings"

	"github.com/banorton/gote/internal/excal"
)

// WriteSummary prints the text rendering of a drawing used by `diagram read`
// and by the terminal pager.
func WriteSummary(w io.Writer, path string, s excal.Summary) {
	fmt.Fprintf(w, "%s\n", path)
	if s.Description != "" {
		fmt.Fprintf(w, "\ndescription\n  %s\n", strings.ReplaceAll(s.Description, "\n", "\n  "))
	}
	section := func(name string, n int) { fmt.Fprintf(w, "\n%s (%d)\n", name, n) }
	section("shapes", len(s.Shapes))
	for _, sh := range s.Shapes {
		fmt.Fprintf(w, "  %s %s %q\n", sh.ID, sh.Type, sh.Label)
	}
	section("texts", len(s.Texts))
	for _, t := range s.Texts {
		fmt.Fprintf(w, "  %s %q\n", t.ID, t.Text)
	}
	section("edges", len(s.Edges))
	for _, e := range s.Edges {
		end := func(p *string) string {
			if p == nil {
				return "(unbound)"
			}
			return *p
		}
		line := fmt.Sprintf("  %s -> %s", end(e.From), end(e.To))
		if e.Label != "" {
			line += fmt.Sprintf(" %q", e.Label)
		}
		fmt.Fprintln(w, strings.TrimRight(line, " "))
	}
}
