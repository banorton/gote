package semantic

import (
	"context"
	"fmt"
	"strings"

	"github.com/banorton/gote/internal/cli"
	_ "github.com/banorton/gote/internal/cli/search"
	opsem "github.com/banorton/gote/internal/ops/semantic"
	"github.com/banorton/gote/internal/semantic"
)

const usage = "gote embed [--yes] [--json]\n       gote embed status [--json]"

func init() {
	cli.Register(cli.Command{
		Name:    "embed",
		Summary: "embed notes for semantic search",
		Usage:   usage,
		Run:     run,
	})
	wrapSearch()
}

// wrapSearch adds the open-key-file warning to `search -s` without editing the
// search command; the blank import above guarantees it is registered first.
func wrapSearch() {
	cmd, ok := cli.Lookup("search")
	if !ok {
		panic("cli/semantic: search command not registered")
	}
	inner := cmd.Run
	cmd.Run = func(c *cli.Ctx, args []string) error {
		for _, a := range args {
			if a == "--" {
				break
			}
			if a == "-s" || a == "--semantic" {
				warnKeyFile(c)
				break
			}
		}
		return inner(c, args)
	}
}

func warnKeyFile(c *cli.Ctx) {
	if c.Cfg == nil {
		return
	}
	if w := c.Cfg.KeyFileWarning(); w != "" {
		fmt.Fprintln(c.Err, w)
	}
}

func run(c *cli.Ctx, args []string) error {
	status, yes := false, false
	for i, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprintln(c.Out, "usage: "+usage)
			return nil
		case a == "status" && i == 0:
			status = true
		case a == "--yes" || a == "-y":
			yes = true
		default:
			return cli.Usagef("embed: unexpected argument %q\nusage: %s", a, usage)
		}
	}
	if status {
		return runStatus(c)
	}
	warnKeyFile(c)
	var r opsem.SyncResp
	if err := c.Client.Call(context.Background(), "semantic.sync", opsem.SyncReq{Confirm: yes}, &r); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(r)
	}
	var cost string
	if usd, ok := semantic.Cost(c.Cfg.Semantic, r.Tokens); ok {
		cost = fmt.Sprintf(", ~$%.4f", usd)
	}
	fmt.Fprintf(c.Out, "embedded %d of %d chunks (~%d tokens%s) in %.1fs\n", r.Embedded, r.Chunks, r.Tokens, cost, r.ElapsedMS/1000)
	return nil
}

func runStatus(c *cli.Ctx) error {
	var r opsem.StatusResp
	if err := c.Client.Call(context.Background(), "semantic.status", nil, &r); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(r)
	}
	if !r.Configured {
		fmt.Fprintln(c.Out, "semantic search not configured")
		return nil
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "provider: %s\nmodel: %s\nchunks: %d\nembedded: %d\npending: %d\n", r.Provider, r.Model, r.Chunks, r.Embedded, r.Pending)
	c.Out.Write([]byte(sb.String()))
	return nil
}
