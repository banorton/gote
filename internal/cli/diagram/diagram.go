package diagram

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
)

const usage = "gote diagram read <ref> | gote diagram check <ref> | gote diagram [name]"

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
		}
	}
	return errors.New("diagram editor not available yet")
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
	fmt.Fprintf(c.Out, "%s\n", r.Path)
	s := r.Summary
	section := func(name string, n int) { fmt.Fprintf(c.Out, "\n%s (%d)\n", name, n) }
	section("shapes", len(s.Shapes))
	for _, sh := range s.Shapes {
		fmt.Fprintf(c.Out, "  %s %s %q\n", sh.ID, sh.Type, sh.Label)
	}
	section("texts", len(s.Texts))
	for _, t := range s.Texts {
		fmt.Fprintf(c.Out, "  %s %q\n", t.ID, t.Text)
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
		fmt.Fprintln(c.Out, strings.TrimRight(line, " "))
	}
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
