package diagram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/cli/view"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
	"github.com/banorton/gote/internal/tui/menu"
)

const usage = "gote diagram [name] | gote diagram open <ref> | gote diagram list [-n N] [--json] | gote diagram read <ref> | gote diagram check <ref>"

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
		case "open":
			return withRef(c, args[1:], "open", edit)
		case "list":
			return list(c, args[1:])
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
	var r struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
	}
	if err := c.Client.Call(context.Background(), "notes.resolve", map[string]string{"ref": args[0]}, &r); err != nil {
		return err
	}
	if r.Kind != "diagram" {
		return ops.Errorf(ops.CodeInvalid, "%s is not a drawing", args[0])
	}
	return fn(c, r.ID)
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
