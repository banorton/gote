package cli

import "fmt"

// Invoke runs another registered command in the same context. Phases call
// commands they do not own (view, open, delete, pin) through it.
func Invoke(c *Ctx, name string, args []string) error {
	cmd, ok := Lookup(name)
	if !ok {
		return fmt.Errorf("%s: not available", name)
	}
	return cmd.Run(c, args)
}

// Lookup finds a command by name or alias.
func Lookup(name string) (*Command, bool) {
	cmd, ok := std.byName[name]
	return cmd, ok
}
