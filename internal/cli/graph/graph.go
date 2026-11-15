package graph

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/banorton/gote/internal/cli"
	opgraph "github.com/banorton/gote/internal/ops/graph"
)

func init() {
	cli.Register(cli.Command{Name: "links", Summary: "list outgoing links of a note", Usage: "gote links <note>", Run: runLinks})
	cli.Register(cli.Command{Name: "backlinks", Summary: "list notes linking to a note", Usage: "gote backlinks <note>", Run: runBacklinks})
	cli.Register(cli.Command{Name: "orphans", Summary: "list notes with no links in or out", Usage: "gote orphans", Run: runOrphans})
	cli.Register(cli.Command{Name: "broken", Summary: "list unresolved links", Usage: "gote broken", Run: runBroken})
	cli.Register(cli.Command{Name: "graph", Summary: "show a note's link neighborhood as a tree", Usage: "gote graph <note> [-d N]", Run: runGraph})
}

func resolve(c *cli.Ctx, ref string) (string, error) {
	var r struct {
		ID string `json:"id"`
	}
	if err := c.Client.Call(context.Background(), "notes.resolve", map[string]string{"ref": ref}, &r); err != nil {
		return "", err
	}
	return r.ID, nil
}

func oneNote(args []string, usage string) (string, error) {
	if len(args) != 1 {
		return "", cli.Usagef("usage: %s", usage)
	}
	return args[0], nil
}

func suffix(kind string, line int) string {
	var b strings.Builder
	if kind != "wiki" {
		fmt.Fprintf(&b, " [%s]", kind)
	}
	if line > 0 {
		fmt.Fprintf(&b, " :%d", line)
	}
	return b.String()
}

func runLinks(c *cli.Ctx, args []string) error {
	ref, err := oneNote(args, "gote links <note>")
	if err != nil {
		return err
	}
	id, err := resolve(c, ref)
	if err != nil {
		return err
	}
	var out []opgraph.LinkOut
	if err := c.Client.Call(context.Background(), "graph.links", opgraph.IDReq{ID: id}, &out); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(out)
	}
	for _, l := range out {
		target := l.Raw + " (unresolved)"
		if l.ID != nil {
			target = *l.ID
		}
		fmt.Fprintln(c.Out, target+suffix(l.Kind, l.Line))
	}
	return nil
}

func runBacklinks(c *cli.Ctx, args []string) error {
	ref, err := oneNote(args, "gote backlinks <note>")
	if err != nil {
		return err
	}
	id, err := resolve(c, ref)
	if err != nil {
		return err
	}
	var out []opgraph.BackOut
	if err := c.Client.Call(context.Background(), "graph.backlinks", opgraph.IDReq{ID: id}, &out); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(out)
	}
	for _, b := range out {
		fmt.Fprintln(c.Out, b.ID+suffix(b.Kind, b.Line))
	}
	return nil
}

func runOrphans(c *cli.Ctx, args []string) error {
	if len(args) != 0 {
		return cli.Usagef("usage: gote orphans")
	}
	var out []opgraph.NoteRef
	if err := c.Client.Call(context.Background(), "graph.orphans", nil, &out); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(out)
	}
	for _, n := range out {
		fmt.Fprintln(c.Out, n.ID)
	}
	return nil
}

func runBroken(c *cli.Ctx, args []string) error {
	if len(args) != 0 {
		return cli.Usagef("usage: gote broken")
	}
	var out []opgraph.Broken
	if err := c.Client.Call(context.Background(), "graph.broken", nil, &out); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(out)
	}
	for _, b := range out {
		fmt.Fprintf(c.Out, "%s:%d %s\n", b.Src, b.Line, b.Raw)
	}
	return nil
}

func runGraph(c *cli.Ctx, args []string) error {
	fs := c.FlagSet("graph")
	depth := fs.Int("d", 1, "depth (1-5)")
	var pos []string
	rest := args
	for {
		if err := cli.ParseFlags(fs, rest); err != nil {
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		rest = fs.Args()[1:]
	}
	ref, err := oneNote(pos, "gote graph <note> [-d N]")
	if err != nil {
		return err
	}
	if *depth < 1 || *depth > 5 {
		return cli.Usagef("-d must be 1-5, got %d", *depth)
	}
	id, err := resolve(c, ref)
	if err != nil {
		return err
	}
	var root opgraph.Node
	if err := c.Client.Call(context.Background(), "graph.neighbors", opgraph.NeighborsReq{ID: id, Depth: *depth}, &root); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(root)
	}
	fmt.Fprintln(c.Out, root.Title)
	writeTree(c.Out, root.Children, "")
	return nil
}

func writeTree(w io.Writer, nodes []opgraph.Node, prefix string) {
	for i, n := range nodes {
		branch, pad := "├── ", "│   "
		if i == len(nodes)-1 {
			branch, pad = "└── ", "    "
		}
		arrow := "→"
		if n.Dir == "in" {
			arrow = "←"
		}
		line := arrow + " " + n.Title
		if n.Kind != "wiki" {
			line += " [" + n.Kind + "]"
		}
		if n.Seen {
			line += " (seen)"
		}
		fmt.Fprintln(w, prefix+branch+line)
		writeTree(w, n.Children, prefix+pad)
	}
}
