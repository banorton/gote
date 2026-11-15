package search

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/banorton/gote/internal/cli"
	opsearch "github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/tui/fuzzy"
)

// Invoke runs another command by name. The merge step assigns cli.Invoke;
// until then commands the fuzzy finder triggers report "not available".
var Invoke func(c *cli.Ctx, name string, args []string) error

const usage = "gote search [-n <limit>] [-s|--semantic] [<query>...]"

func init() {
	cli.Register(cli.Command{
		Name:    "search",
		Aliases: []string{"s"},
		Summary: "search notes (no args: fuzzy finder)",
		Usage:   usage,
		Run:     run,
	})
}

func invoke(c *cli.Ctx, name string, args []string) error {
	if Invoke == nil {
		return fmt.Errorf("%s: not available", name)
	}
	return Invoke(c, name, args)
}

// invokeQuiet runs a command that must not write to the alt-screen TUI.
func invokeQuiet(c *cli.Ctx, name string, args []string) error {
	cc := *c
	cc.Out, cc.Err = io.Discard, io.Discard
	return invoke(&cc, name, args)
}

// parseArgs keeps tokens like -tag:x as query terms; only -n and -s/--semantic
// (and everything before a bare --) are flags.
var errHelp = fmt.Errorf("help")

func parseArgs(args []string) (words []string, limit int, semantic bool, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return append(words, args[i+1:]...), limit, semantic, nil
		case a == "-h" || a == "--help":
			return nil, 0, false, errHelp
		case a == "-s" || a == "--semantic":
			semantic = true
		case a == "-n" || a == "--n":
			if i+1 >= len(args) {
				return nil, 0, false, cli.Usagef("-n needs a value")
			}
			i++
			if limit, err = parseLimit(args[i]); err != nil {
				return nil, 0, false, err
			}
		case strings.HasPrefix(a, "-n=") || strings.HasPrefix(a, "--n="):
			if limit, err = parseLimit(a[strings.Index(a, "=")+1:]); err != nil {
				return nil, 0, false, err
			}
		default:
			words = append(words, a)
		}
	}
	return words, limit, semantic, nil
}

func parseLimit(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, cli.Usagef("-n must be a positive integer, got %q", s)
	}
	return n, nil
}

func run(c *cli.Ctx, args []string) error {
	words, limit, semantic, err := parseArgs(args)
	if err == errHelp {
		fmt.Fprintln(c.Out, "usage: "+usage)
		return nil
	}
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return runFuzzy(c)
	}
	var resp opsearch.QueryResp
	req := opsearch.QueryReq{Q: strings.Join(words, " "), Limit: limit, Semantic: semantic}
	if err := c.Client.Call(context.Background(), "search.query", req, &resp); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(resp)
	}
	for _, r := range resp.Results {
		loc := r.ID
		if r.Line > 0 {
			loc = fmt.Sprintf("%s:%d", r.ID, r.Line)
		}
		fmt.Fprintf(c.Out, "%s  (%s)\n", r.Title, loc)
		if s := strings.TrimSpace(strings.ReplaceAll(r.Snippet, "\n", " ")); s != "" {
			fmt.Fprintf(c.Out, "    %s\n", s)
		}
	}
	return nil
}

func noteSearchString(it opsearch.CorpusItem) string {
	s := it.Title + "  " + it.Path
	if len(it.Aliases) > 0 {
		s += "  " + strings.Join(it.Aliases, " ")
	}
	return s
}

func previewText(p opsearch.PreviewResp, line int) string {
	var b strings.Builder
	for i, l := range p.Lines {
		n := p.Start + i
		mark := " "
		if n == line {
			mark = ">"
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s%3d  %s", mark, n, l)
	}
	return b.String()
}

func runFuzzy(c *cli.Ctx) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return cli.Usagef("search: no query given and not a terminal\nusage: %s", usage)
	}
	ctx := context.Background()
	return fuzzy.Run(fuzzy.Config{
		Notes: func() ([]fuzzy.Item, error) {
			var r opsearch.CorpusResp
			if err := c.Client.Call(ctx, "search.corpus", nil, &r); err != nil {
				return nil, err
			}
			items := make([]fuzzy.Item, len(r.Items))
			for i, it := range r.Items {
				items[i] = fuzzy.Item{ID: it.ID, Search: noteSearchString(it)}
			}
			return items, nil
		},
		Lines: func() ([]fuzzy.Item, error) {
			var r opsearch.LinesResp
			if err := c.Client.Call(ctx, "search.lines", nil, &r); err != nil {
				return nil, err
			}
			items := make([]fuzzy.Item, len(r.Lines))
			for i, l := range r.Lines {
				items[i] = fuzzy.Item{ID: l.ID, Line: l.Line, Search: l.Text, Suffix: fmt.Sprintf("%s:%d", l.ID, l.Line)}
			}
			return items, nil
		},
		Preview: func(id string, line int) (string, error) {
			var r opsearch.PreviewResp
			if err := c.Client.Call(ctx, "search.preview", opsearch.PreviewReq{ID: id, Line: line}, &r); err != nil {
				return "", err
			}
			return previewText(r, line), nil
		},
		Invoke: func(name string, args []string) error {
			if name == "delete" || name == "pin" {
				return invokeQuiet(c, name, args)
			}
			return invoke(c, name, args)
		},
		Edit: func(id string, line int) error {
			fields := strings.Fields(c.Cfg.Editor)
			args := append(fields[1:], "+"+strconv.Itoa(line), filepath.Join(c.Cfg.NotesDir, filepath.FromSlash(id)))
			cmd := exec.Command(fields[0], args...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("editor %s: %w", fields[0], err)
			}
			if err := c.Client.Call(ctx, "notes.visit", map[string]string{"id": id}, nil); err != nil {
				return err
			}
			return c.Client.Call(ctx, "index.reconcile", nil, nil)
		},
		Editor: c.Cfg.Editor,
	})
}
