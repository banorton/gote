package tasks

import (
	"context"
	"fmt"
	"strings"

	"github.com/banorton/gote/internal/cli"
	optasks "github.com/banorton/gote/internal/ops/tasks"
)

const usage = "gote tasks [--all] [<query>...]   (--all also includes done tasks and archived notes)"

func init() {
	cli.Register(cli.Command{
		Name:    "tasks",
		Summary: "list open tasks (- [ ]) across notes",
		Usage:   usage,
		Run:     run,
	})
}

// Hand-parsed so query terms like -tag:x are not mistaken for flags.
func run(c *cli.Ctx, args []string) error {
	all := false
	var words []string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--":
			words = append(words, args[i+1:]...)
			i = len(args)
		case "-h", "--help":
			fmt.Fprintln(c.Out, "usage: "+usage)
			return nil
		case "--all":
			all = true
		default:
			words = append(words, a)
		}
	}
	var out []optasks.Task
	req := optasks.ListReq{All: all, Archived: all, Q: strings.Join(words, " ")}
	if err := c.Client.Call(context.Background(), "tasks.list", req, &out); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(out)
	}
	for _, t := range out {
		box := "[ ]"
		if t.Done {
			box = "[x]"
		}
		fmt.Fprintf(c.Out, "%s (%s:%d)  %s %s\n", t.Title, t.ID, t.Line, box, t.Text)
	}
	return nil
}
