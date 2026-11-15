package cli

import (
	"context"
	"fmt"

	"github.com/banorton/gote/internal/ops"
)

func init() {
	Register(Command{
		Name:          "reindex",
		Aliases:       []string{"idx"},
		Summary:       "drop and rebuild the search index",
		Usage:         "gote reindex",
		SkipReconcile: true,
		Run: func(c *Ctx, args []string) error {
			var r ops.StatsResp
			if err := c.Client.Call(context.Background(), "index.rebuild", nil, &r); err != nil {
				return err
			}
			if c.JSON {
				return c.PrintJSON(r)
			}
			fmt.Fprintf(c.Out, "indexed %d notes in %.1fms\n", r.Added, r.ElapsedMS)
			return nil
		},
	})
	Register(Command{
		Name:    "status",
		Summary: "show index counts",
		Usage:   "gote status",
		Run: func(c *Ctx, args []string) error {
			var r ops.StatusResp
			if err := c.Client.Call(context.Background(), "index.status", nil, &r); err != nil {
				return err
			}
			if c.JSON {
				return c.PrintJSON(r)
			}
			fmt.Fprintf(c.Out, "notes: %d\nlinks: %d (%d unresolved)\ndb: %s\n", r.Notes, r.Links, r.Unresolved, r.DB)
			return nil
		},
	})
}
