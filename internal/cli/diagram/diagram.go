package diagram

import (
	"context"
	"fmt"
	"strings"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/cli/view"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
)

const usage = "gote diagram [name] | gote diagram open <ref> | gote diagram read <ref> | gote diagram check <ref>"

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
