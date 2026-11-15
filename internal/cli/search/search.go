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
var Invoke = cli.Invoke

const usage = "gote search [-n <limit>] [<words>...]   fuzzy on titles, paths, aliases (no words: finder)\n       gote search [-n <limit>] -t|--text <query>...   full-text with filters\n       gote search [-n <limit>] -s|--semantic <query>..."

func init() {
	cli.Register(cli.Command{
		Name:    "search",
		Aliases: []string{"s"},
		Summary: "fuzzy search titles (no args: finder); -t full-text, -s semantic",
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

// parseArgs keeps tokens like -tag:x as query terms; only -n, -t/--text and
// -s/--semantic (and everything before a bare --) are flags.
var errHelp = fmt.Errorf("help")

type opts struct {
	words          []string
	limit          int
	text, semantic bool
}

func parseArgs(args []string) (o opts, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			o.words = append(o.words, args[i+1:]...)
			return o, check(o)
		case a == "-h" || a == "--help":
			return opts{}, errHelp
		case a == "-s" || a == "--semantic":
			o.semantic = true
		case a == "-t" || a == "--text":
			o.text = true
		case a == "-n" || a == "--n":
			if i+1 >= len(args) {
				return opts{}, cli.Usagef("-n needs a value")
			}
			i++
			if o.limit, err = parseLimit(args[i]); err != nil {
				return opts{}, err
			}
		case strings.HasPrefix(a, "-n=") || strings.HasPrefix(a, "--n="):
			if o.limit, err = parseLimit(a[strings.Index(a, "=")+1:]); err != nil {
				return opts{}, err
			}
		default:
			o.words = append(o.words, a)
		}
	}
	return o, check(o)
}

func check(o opts) error {
	if o.text && o.semantic {
		return cli.Usagef("search: -t and -s are mutually exclusive\nusage: %s", usage)
	}
	return nil
}

func parseLimit(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, cli.Usagef("-n must be a positive integer, got %q", s)
	}
	return n, nil
}

func run(c *cli.Ctx, args []string) error {
	o, err := parseArgs(args)
	if err == errHelp {
		fmt.Fprintln(c.Out, "usage: "+usage)
		return nil
	}
	if err != nil {
		return err
	}
	if len(o.words) == 0 {
		if o.text || o.semantic {
			return cli.Usagef("search: -t/-s need a query\nusage: %s", usage)
		}
		return runFinder(c)
	}
	if o.text || o.semantic {
		return runQuery(c, o)
	}
	return runFuzzy(c, o)
}

func runQuery(c *cli.Ctx, o opts) error {
	var resp opsearch.QueryResp
	req := opsearch.QueryReq{Q: strings.Join(o.words, " "), Limit: o.limit, Semantic: o.semantic}
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

func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// runFuzzy on a terminal hands off to the "so" lister (home-row menu, open
// preselected); otherwise it prints the ranked list.
func runFuzzy(c *cli.Ctx, o opts) error {
	if !c.JSON && isTTY() {
		var args []string
		if o.limit > 0 {
			args = append(args, "-n", strconv.Itoa(o.limit))
		}
		return invoke(c, "so", append(args, o.words...))
	}
	var resp opsearch.FuzzyResp
	req := opsearch.FuzzyReq{Q: strings.Join(o.words, " "), Limit: o.limit}
	if err := c.Client.Call(context.Background(), "search.fuzzy", req, &resp); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(resp)
	}
	for _, r := range resp.Results {
		fmt.Fprintf(c.Out, "%s  (%s)\n", r.Title, r.ID)
	}
	return nil
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

func runFinder(c *cli.Ctx) error {
	if !isTTY() {
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
				items[i] = fuzzy.Item{ID: it.ID, Search: opsearch.SearchString(it)}
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
