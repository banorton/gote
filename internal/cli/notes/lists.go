package notes

import (
	"errors"
	"fmt"
	"strings"

	"github.com/banorton/gote/internal/cli"
	opsnotes "github.com/banorton/gote/internal/ops/notes"
	"github.com/banorton/gote/internal/tui/menu"
)

var allActions = []menu.Action{"open", "delete", "pin", "view", "rename", "unpin"}

type hit struct {
	opsnotes.Ref
	Snippet string `json:"snippet,omitempty"`
}

type lister struct {
	name string
	list func(c *cli.Ctx, args []string) ([]hit, error)
}

func toHits(rs []opsnotes.Ref) []hit {
	out := make([]hit, len(rs))
	for i, r := range rs {
		out[i] = hit{Ref: r}
	}
	return out
}

func listRecent(c *cli.Ctx, args []string) ([]hit, error) {
	fs := c.FlagSet("recent")
	n := fs.Int("n", 0, "max notes")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return nil, err
	}
	if len(pos) > 0 {
		return nil, cli.Usagef("usage: gote recent [-n N]")
	}
	var rs []opsnotes.Ref
	if err := call(c, "notes.recent", opsnotes.RecentReq{Limit: *n}, &rs); err != nil {
		return nil, err
	}
	return toHits(rs), nil
}

func listPinned(c *cli.Ctx, args []string) ([]hit, error) {
	var rs []opsnotes.Ref
	if err := call(c, "pins.list", nil, &rs); err != nil {
		return nil, err
	}
	return toHits(rs), nil
}

func listAll(c *cli.Ctx, args []string) ([]hit, error) {
	var rs []opsnotes.Ref
	if err := call(c, "notes.list", opsnotes.ListReq{}, &rs); err != nil {
		return nil, err
	}
	return toHits(rs), nil
}

func listTagged(c *cli.Ctx, args []string) ([]hit, error) {
	if len(args) == 0 {
		return nil, cli.Usagef("usage: gote tag <tag> [tag...]")
	}
	var rs []opsnotes.Ref
	if err := call(c, "tags.notes", opsnotes.TagsNotesReq{Tags: args}, &rs); err != nil {
		return nil, err
	}
	return toHits(rs), nil
}

type searchResp struct {
	Results []struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Kind    string `json:"kind"`
		Path    string `json:"path"`
		Snippet string `json:"snippet"`
	} `json:"results"`
}

func searchQuery(c *cli.Ctx, q string) ([]hit, error) {
	var r searchResp
	if err := call(c, "search.query", map[string]any{"q": q, "limit": 0, "semantic": false}, &r); err != nil {
		return nil, err
	}
	out := make([]hit, len(r.Results))
	for i, x := range r.Results {
		out[i] = hit{Ref: opsnotes.Ref{ID: x.ID, Title: x.Title, Kind: x.Kind, Path: x.Path}, Snippet: x.Snippet}
	}
	return out, nil
}

func listSearch(c *cli.Ctx, args []string) ([]hit, error) {
	q := joined(args)
	if q == "" {
		return nil, cli.Usagef("usage: gote search <query>")
	}
	return searchQuery(c, q)
}

var listers = map[string]lister{
	"r": {"recent", listRecent},
	"s": {"search", listSearch},
	"t": {"tag", listTagged},
	"p": {"pinned", listPinned},
}

func confirm(c *cli.Ctx, msg string) (bool, error) {
	a, err := prompt(c, msg+" [y/N]", "")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(a, "y") || strings.EqualFold(a, "yes"), nil
}

func perform(c *cli.Ctx, h hit, action menu.Action) error {
	switch action {
	case "open":
		return editNote(c, h.ID, h.Path)
	case "delete":
		ok, err := confirm(c, fmt.Sprintf("Delete %q?", h.Title))
		if err != nil || !ok {
			return err
		}
		return deleteID(c, h.ID)
	case "pin":
		return pinID(c, h.ID, true)
	case "unpin":
		return pinID(c, h.ID, false)
	case "view":
		return cli.Invoke(c, "view", []string{h.ID})
	case "rename":
		n, err := prompt(c, "New name", h.Title)
		if err != nil || n == "" || n == h.Title {
			return err
		}
		return renameID(c, h.ID, n)
	}
	return fmt.Errorf("unknown action %q", action)
}

// pick lists hits and, on a terminal, runs the menu and the chosen action.
// Without a terminal (or with --json) it prints the list instead.
func pick(c *cli.Ctx, hits []hit, action menu.Action) error {
	if c.JSON {
		if hits == nil {
			hits = []hit{}
		}
		return c.PrintJSON(hits)
	}
	if len(hits) == 0 {
		fmt.Fprintln(c.Out, "no notes")
		return nil
	}
	items := make([]menu.Item, len(hits))
	for i, h := range hits {
		items[i] = menu.Item{ID: h.ID, Title: h.Title, Detail: h.Snippet}
	}
	it, act, err := menu.Run(items, action, allActions)
	switch {
	case errors.Is(err, menu.ErrCancelled):
		return nil
	case errors.Is(err, menu.ErrNotTTY):
		for _, h := range hits {
			fmt.Fprintf(c.Out, "%s\t%s\n", h.ID, h.Title)
		}
		return nil
	case err != nil:
		return err
	}
	for _, h := range hits {
		if h.ID == it.ID {
			return perform(c, h, act)
		}
	}
	return fmt.Errorf("menu returned unknown item %q", it.ID)
}

func listAndPick(c *cli.Ctx, l func(*cli.Ctx, []string) ([]hit, error), args []string, action menu.Action) error {
	hits, err := l(c, args)
	if err != nil {
		return err
	}
	return pick(c, hits, action)
}

var actionKeys = map[string]menu.Action{
	"o": "open", "d": "delete", "p": "pin", "v": "view", "r": "rename", "u": "unpin",
}

func init() {
	cli.Register(cli.Command{Name: "recent", Aliases: []string{"r"}, Summary: "menu of recently opened notes",
		Usage: "gote recent [-n N]",
		Run:   func(c *cli.Ctx, args []string) error { return listAndPick(c, listRecent, args, "open") }})
	cli.Register(cli.Command{Name: "get", Aliases: []string{"g"}, Summary: "menu of all notes",
		Usage: "gote get",
		Run:   func(c *cli.Ctx, args []string) error { return listAndPick(c, listAll, args, "open") }})

	combos := []string{"ro", "rd", "rp", "rv", "rr", "so", "sd", "sp", "sv", "sr", "to", "td", "tp", "tv", "tr", "po", "pd", "pv", "pu", "pr"}
	for _, name := range combos {
		l, act := listers[name[:1]], actionKeys[name[1:]]
		cli.Register(cli.Command{Name: name, Summary: fmt.Sprintf("%s menu, %s selected note", l.name, act),
			Usage: fmt.Sprintf("gote %s [args]", name),
			Run:   func(c *cli.Ctx, args []string) error { return listAndPick(c, l.list, args, act) }})
	}
}
