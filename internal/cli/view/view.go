package view

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"golang.org/x/term"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/parse"
	"github.com/banorton/gote/internal/tui/pager"
)

const usage = "gote view [-T|--terminal] <note>"

var command = cli.Command{
	Name:    "view",
	Aliases: []string{"v"},
	Summary: "view a note in the browser, or -T in the terminal",
	Usage:   usage,
	Run:     run,
}

func init() { cli.Register(command) }

func escape(id string) string {
	segs := strings.Split(id, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

type resolved struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Path  string `json:"path"`
}

// Resolve maps a user reference to a note or drawing.
func Resolve(c *cli.Ctx, ref string) (resolved, error) {
	var r resolved
	err := c.Client.Call(context.Background(), "notes.resolve", map[string]string{"ref": ref}, &r)
	return r, err
}

func run(c *cli.Ctx, args []string) error {
	var terminal bool
	var words []string
	for _, a := range args {
		switch {
		case a == "-T" || a == "--terminal":
			terminal = true
		case a != "-" && strings.HasPrefix(a, "-"):
			return cli.Usagef("unknown flag %s\nusage: %s", a, usage)
		default:
			words = append(words, a)
		}
	}
	ref := strings.TrimSpace(strings.Join(words, " "))
	if ref == "" {
		return cli.Usagef("usage: %s", usage)
	}
	if terminal {
		return runTerminal(c, ref)
	}
	r, err := Resolve(c, ref)
	if err != nil {
		return err
	}
	return Launch(c, r.ID, Route(r.ID, r.Kind))
}

var isTTY = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func termWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
			return cols
		}
	}
	return 80
}

var runPager = func(c *cli.Ctx, m *pager.Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(c.Out)).Run()
	return err
}

func runTerminal(c *cli.Ctx, ref string) error {
	tty := isTTY(c.Out)
	width := termWidth(c.Out)
	doc, err := loadDoc(c, ref, width, tty)
	if err != nil {
		return err
	}
	if !tty {
		if c.JSON {
			return c.PrintJSON(map[string]string{"id": doc.Ref, "text": doc.Text})
		}
		_, err := fmt.Fprint(c.Out, strings.TrimLeft(doc.Text, "\n"))
		return err
	}
	for {
		m := pager.New(doc, func(target string) (pager.Doc, error) { return loadDoc(c, target, width, true) })
		if err := runPager(c, m); err != nil {
			return err
		}
		req := m.EditRequest()
		if req == "" {
			return nil
		}
		if err := cli.Invoke(c, "open", []string{req}); err != nil {
			return err
		}
		if doc, err = loadDoc(c, req, width, true); err != nil {
			return err
		}
	}
}

// loadDoc renders a note (markdown via glamour) or a drawing (the text
// summary) for the pager. styled selects ANSI styling; otherwise plain text.
func loadDoc(c *cli.Ctx, ref string, width int, styled bool) (pager.Doc, error) {
	r, err := Resolve(c, ref)
	if err != nil {
		return pager.Doc{}, err
	}
	if r.Kind == note.KindDiagram {
		var d struct {
			Path    string        `json:"path"`
			Summary excal.Summary `json:"summary"`
		}
		if err := c.Client.Call(context.Background(), "diagram.read", map[string]string{"id": r.ID}, &d); err != nil {
			return pager.Doc{}, err
		}
		var buf bytes.Buffer
		WriteSummary(&buf, d.Path, d.Summary)
		return pager.Doc{Ref: r.ID, Title: r.ID, Text: buf.String()}, nil
	}
	var rd struct {
		Text string `json:"text"`
	}
	if err := c.Client.Call(context.Background(), "notes.read", map[string]string{"id": r.ID}, &rd); err != nil {
		return pager.Doc{}, err
	}
	n := parse.Note(r.ID, []byte(rd.Text))
	opts := []glamour.TermRendererOption{glamour.WithWordWrap(width)}
	if styled {
		opts = append(opts, glamour.WithAutoStyle())
	} else {
		opts = append(opts, glamour.WithStandardStyle("notty"))
	}
	tr, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		return pager.Doc{}, fmt.Errorf("render %s: %w", r.ID, err)
	}
	body, err := tr.Render(n.Body)
	if err != nil {
		return pager.Doc{}, fmt.Errorf("render %s: %w", r.ID, err)
	}
	if len(n.Tags) > 0 {
		body = "  tags: " + strings.Join(n.Tags, ", ") + "\n" + body
	}
	return pager.Doc{Ref: r.ID, Title: n.Title, Text: body, Links: linksOf(r.ID, n)}, nil
}

func linksOf(id string, n note.Note) []pager.Link {
	var out []pager.Link
	for _, l := range n.Links {
		switch l.Kind {
		case note.LinkWiki, note.LinkEmbed:
			if imageExt[strings.ToLower(path.Ext(l.Target))] {
				continue
			}
			label := l.Alias
			if label == "" {
				label = "[[" + l.Target
				if l.Fragment != "" {
					label += "#" + l.Fragment
				}
				label += "]]"
			}
			out = append(out, pager.Link{Label: label, Target: l.Target})
		case note.LinkMD:
			out = append(out, pager.Link{Label: l.Target, Target: path.Join(path.Dir(id), l.Target)})
		}
	}
	return out
}

var imageExt = map[string]bool{
	".apng": true, ".avif": true, ".gif": true, ".jpg": true, ".jpeg": true,
	".jfif": true, ".png": true, ".svg": true, ".webp": true, ".pdf": true,
}
