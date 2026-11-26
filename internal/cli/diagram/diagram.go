package diagram

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/cli/view"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
	opsearch "github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/tui/menu"
)

const usage = "gote diagram [name] | gote diagram open <ref> | gote diagram list [-n N] [--json] | gote diagram read <ref> | gote diagram check <ref> | gote diagram render <ref> [--json] | gote diagram describe <ref> [<text>...|-|--clear] | gote diagram search [-n N] [-t|-s] <words>..."

// stdin is where `describe <ref> -` reads from; tests replace it.
var stdin io.Reader = os.Stdin

var allActions = []menu.Action{"open"}

var command = cli.Command{
	Name:    "diagram",
	Summary: "read, check or draw Excalidraw drawings",
	Usage:   usage,
	Run:     dispatch,
}

func init() { cli.Register(command) }

func dispatch(c *cli.Ctx, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "read":
			return withRef(c, args[1:], "read", read)
		case "check":
			return withRef(c, args[1:], "check", check)
		case "render":
			return withRef(c, args[1:], "render", renderPNG)
		case "open":
			return withRef(c, args[1:], "open", edit)
		case "list":
			return list(c, args[1:])
		case "describe":
			return describe(c, args[1:])
		case "search":
			return searchCmd(c, args[1:])
		}
	}
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			if a == "-h" || a == "--help" {
				fmt.Fprintln(c.Out, "usage:")
				for _, l := range strings.Split(usage, " | ") {
					fmt.Fprintln(c.Out, "  "+l)
				}
				return nil
			}
			return cli.Usagef("unknown flag %q\nusage: %s", a, usage)
		}
	}
	return create(c, strings.Join(args, " "))
}

func create(c *cli.Ctx, name string) error {
	var r struct {
		ID string `json:"id"`
	}
	if err := c.Client.Call(context.Background(), "diagram.create", map[string]string{"name": name}, &r); err != nil {
		return err
	}
	return view.Launch(c, r.ID, view.Route(r.ID, note.KindDiagram))
}

type listItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
}

func list(c *cli.Ctx, args []string) error {
	fs := c.FlagSet("diagram list")
	n := fs.Int("n", 0, "max drawings")
	if err := cli.ParseFlags(fs, args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return cli.Usagef("usage: gote diagram list [-n N] [--json]")
	}
	var items []listItem
	if err := c.Client.Call(context.Background(), "diagram.list", map[string]int{"limit": *n}, &items); err != nil {
		return err
	}
	if items == nil {
		items = []listItem{}
	}
	if c.JSON {
		return c.PrintJSON(items)
	}
	if len(items) == 0 {
		fmt.Fprintln(c.Out, "no drawings")
		return nil
	}
	mi := make([]menu.Item, len(items))
	for i, it := range items {
		mi[i] = menu.Item{ID: it.ID, Title: it.Title}
	}
	it, _, err := menu.Run(mi, "open", allActions)
	switch {
	case errors.Is(err, menu.ErrCancelled):
		return nil
	case errors.Is(err, menu.ErrNotTTY):
		for _, x := range items {
			fmt.Fprintf(c.Out, "%s  (%s)\n", x.Title, x.ID)
		}
		return nil
	case err != nil:
		return err
	}
	return edit(c, it.ID)
}

func edit(c *cli.Ctx, id string) error {
	return view.Launch(c, id, view.Route(id, note.KindDiagram))
}

func withRef(c *cli.Ctx, args []string, sub string, fn func(*cli.Ctx, string) error) error {
	if len(args) != 1 {
		return cli.Usagef("usage: gote diagram %s <ref>", sub)
	}
	id, err := resolveDrawing(c, args[0])
	if err != nil {
		return err
	}
	return fn(c, id)
}

func resolveDrawing(c *cli.Ctx, ref string) (string, error) {
	var r struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	if err := c.Client.Call(context.Background(), "notes.resolve", map[string]string{"ref": ref}, &r); err != nil {
		return "", err
	}
	if r.Kind != "diagram" {
		return "", ops.Errorf(ops.CodeInvalid, "%s is not a drawing", ref)
	}
	return r.ID, nil
}

const describeUsage = "usage: gote diagram describe <ref> [<text>...|-|--clear]"

func describe(c *cli.Ctx, args []string) error {
	if len(args) == 0 {
		return cli.Usagef("%s", describeUsage)
	}
	id, err := resolveDrawing(c, args[0])
	if err != nil {
		return err
	}
	clear := false
	var words []string
	for _, a := range args[1:] {
		if a == "--clear" {
			clear = true
		} else {
			words = append(words, a)
		}
	}
	if clear && len(words) > 0 {
		return cli.Usagef("describe: --clear takes no text\n%s", describeUsage)
	}
	text := strings.Join(words, " ")
	if len(words) == 1 && words[0] == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		if text = strings.TrimSpace(string(b)); text == "" {
			return cli.Usagef("describe: stdin was empty")
		}
	}
	var out struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}
	if !clear && text == "" {
		var r readResp
		if err := c.Client.Call(context.Background(), "diagram.read", map[string]string{"id": id}, &r); err != nil {
			return err
		}
		out.ID, out.Description = id, r.Summary.Description
		if c.JSON {
			return c.PrintJSON(out)
		}
		if out.Description != "" {
			fmt.Fprintln(c.Out, out.Description)
		}
		return nil
	}
	req := map[string]any{"id": id, "description": text, "clear": clear}
	if err := c.Client.Call(context.Background(), "diagram.describe", req, &out); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(out)
	}
	return nil
}

const searchUsage = "usage: gote diagram search [-n <limit>] <words>...   fuzzy on drawing titles and paths\n       gote diagram search [-n <limit>] -t <query>...   full-text with filters (labels + description)\n       gote diagram search [-n <limit>] -s <query>...   semantic"

func searchCmd(c *cli.Ctx, args []string) error {
	fs := c.FlagSet("diagram search")
	n := fs.Int("n", 0, "max results")
	text := fs.Bool("t", false, "full-text")
	sem := fs.Bool("s", false, "semantic")
	fs.BoolVar(text, "text", false, "full-text")
	fs.BoolVar(sem, "semantic", false, "semantic")
	if err := cli.ParseFlags(fs, args); err != nil {
		return err
	}
	if *text && *sem {
		return cli.Usagef("diagram search: -t and -s are mutually exclusive\n%s", searchUsage)
	}
	if *n < 0 || (*n == 0 && flagSet(fs, "n")) {
		return cli.Usagef("-n must be a positive integer")
	}
	if fs.NArg() == 0 {
		return cli.Usagef("diagram search: no query given\n%s", searchUsage)
	}
	q := strings.Join(fs.Args(), " ")
	if *text || *sem {
		return searchQuery(c, q, *n, *sem)
	}
	return searchFuzzy(c, q, *n)
}

func flagSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func searchQuery(c *cli.Ctx, q string, limit int, semantic bool) error {
	var resp opsearch.QueryResp
	req := opsearch.QueryReq{Q: q, Limit: limit, Semantic: semantic, Kind: "diagram"}
	if err := c.Client.Call(context.Background(), "search.query", req, &resp); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(resp)
	}
	for _, r := range resp.Results {
		fmt.Fprintf(c.Out, "%s  (%s)\n", r.Title, r.ID)
		if s := strings.TrimSpace(strings.ReplaceAll(r.Snippet, "\n", " ")); s != "" {
			fmt.Fprintf(c.Out, "    %s\n", s)
		}
	}
	return nil
}

func searchFuzzy(c *cli.Ctx, q string, limit int) error {
	var resp opsearch.FuzzyResp
	req := opsearch.FuzzyReq{Q: q, Limit: limit, Kind: "diagram"}
	if err := c.Client.Call(context.Background(), "search.fuzzy", req, &resp); err != nil {
		return err
	}
	if c.JSON {
		type result struct {
			ID    string  `json:"id"`
			Title string  `json:"title"`
			Path  string  `json:"path"`
			Score float64 `json:"score"`
		}
		out := struct {
			Results []result `json:"results"`
		}{[]result{}}
		for _, r := range resp.Results {
			out.Results = append(out.Results, result{r.ID, r.Title, r.Path, r.Score})
		}
		return c.PrintJSON(out)
	}
	if len(resp.Results) == 0 {
		fmt.Fprintln(c.Out, "no drawings")
		return nil
	}
	mi := make([]menu.Item, len(resp.Results))
	for i, it := range resp.Results {
		mi[i] = menu.Item{ID: it.ID, Title: it.Title}
	}
	it, _, err := menu.Run(mi, "open", allActions)
	switch {
	case errors.Is(err, menu.ErrCancelled):
		return nil
	case errors.Is(err, menu.ErrNotTTY):
		for _, x := range resp.Results {
			fmt.Fprintf(c.Out, "%s  (%s)\n", x.Title, x.ID)
		}
		return nil
	case err != nil:
		return err
	}
	return edit(c, it.ID)
}

type readResp struct {
	ID      string        `json:"id"`
	Path    string        `json:"path"`
	Summary excal.Summary `json:"summary"`
}

func read(c *cli.Ctx, id string) error {
	var r readResp
	if err := c.Client.Call(context.Background(), "diagram.read", map[string]string{"id": id}, &r); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(r)
	}
	view.WriteSummary(c.Out, r.Path, r.Summary)
	return nil
}

func check(c *cli.Ctx, id string) error {
	var r struct {
		Issues []excal.Issue `json:"issues"`
	}
	if err := c.Client.Call(context.Background(), "diagram.check", map[string]string{"id": id}, &r); err != nil {
		return err
	}
	switch {
	case c.JSON:
		if err := c.PrintJSON(r); err != nil {
			return err
		}
	case len(r.Issues) == 0:
		fmt.Fprintln(c.Out, "ok")
	default:
		for _, i := range r.Issues {
			fmt.Fprintf(c.Out, "%s: %s\n", i.Element, i.Problem)
		}
	}
	if len(r.Issues) > 0 {
		return ops.Errorf(ops.CodeInvalid, "%d binding issue(s) in %s", len(r.Issues), id)
	}
	return nil
}

func renderPNG(c *cli.Ctx, id string) error {
	var r struct {
		ID     string `json:"id"`
		Path   string `json:"path"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Fresh  bool   `json:"fresh"`
	}
	if err := c.Client.Call(context.Background(), "diagram.render", map[string]string{"id": id}, &r); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(r)
	}
	fmt.Fprintln(c.Out, r.Path)
	return nil
}
