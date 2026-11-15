package cli

import "fmt"

func init() {
	Register(Command{
		Name:     "version",
		Summary:  "print the gote version",
		Usage:    "gote version",
		NoConfig: true,
		Run: func(c *Ctx, args []string) error {
			if c.JSON {
				return c.PrintJSON(map[string]string{"version": Version})
			}
			fmt.Fprintln(c.Out, "gote "+Version)
			return nil
		},
	})
}
