package cli

import (
	"fmt"
	"io"
	"strings"
)

func init() {
	Register(Command{
		Name:     "help",
		Aliases:  []string{"h", "man"},
		Summary:  "list commands or show help for one",
		Usage:    "gote help [command]",
		NoConfig: true,
		Run:      func(c *Ctx, args []string) error { return std.help(c, args) },
	})
}

func (r *registry) printHelp(w io.Writer) {
	fmt.Fprintln(w, "usage: gote [--json] [--no-server] <command> [args]")
	fmt.Fprintln(w, "       gote <note>    open or create a note")
	fmt.Fprintln(w, "\ncommands:")
	for _, cmd := range r.sorted() {
		name := cmd.Name
		if len(cmd.Aliases) > 0 {
			name += " (" + strings.Join(cmd.Aliases, ", ") + ")"
		}
		fmt.Fprintf(w, "  %-24s %s\n", name, cmd.Summary)
	}
}

func (r *registry) help(c *Ctx, args []string) error {
	if len(args) > 0 {
		cmd := r.byName[args[0]]
		if cmd == nil {
			return Usagef("unknown command %q", args[0])
		}
		if c.JSON {
			return c.PrintJSON(helpEntry(cmd))
		}
		fmt.Fprintf(c.Out, "%s\n", cmd.Summary)
		if cmd.Usage != "" {
			fmt.Fprintf(c.Out, "usage: %s\n", cmd.Usage)
		} else {
			fmt.Fprintf(c.Out, "usage: gote %s\n", cmd.Name)
		}
		if len(cmd.Aliases) > 0 {
			fmt.Fprintf(c.Out, "aliases: %s\n", strings.Join(cmd.Aliases, ", "))
		}
		return nil
	}
	if c.JSON {
		var all []map[string]any
		for _, cmd := range r.sorted() {
			all = append(all, helpEntry(cmd))
		}
		return c.PrintJSON(all)
	}
	r.printHelp(c.Out)
	return nil
}

func helpEntry(cmd *Command) map[string]any {
	return map[string]any{"name": cmd.Name, "aliases": cmd.Aliases, "summary": cmd.Summary, "usage": cmd.Usage}
}
