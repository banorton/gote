package cli

import (
	"strconv"
	"strings"
)

// Args provides a simple, consistent way to parse command-line arguments.
// Supports:
//   - Bool flags: -o, --open
//   - Value flags: -n 5, --limit 5
//   - List flags: -t foo bar (collects until next flag or end)
//   - Positional args: everything not consumed by flags
type Args struct {
	flags      map[string][]string // flag name -> values (empty slice = bool flag)
	Positional []string
}

// ParseArgs parses command-line arguments into an Args struct.
// Flags start with - or --. A flag followed by non-flag args captures them as values.
func ParseArgs(args []string) Args {
	a := Args{
		flags:      make(map[string][]string),
		Positional: []string{},
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		if strings.HasPrefix(arg, "-") {
			// Strip leading dashes
			name := strings.TrimLeft(arg, "-")
			if name == "" {
				i++
				continue
			}

			// Collect values until next flag or end
			values := []string{}
			for i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				values = append(values, args[i])
			}
			a.flags[name] = values
		} else {
			a.Positional = append(a.Positional, arg)
		}
		i++
	}

	return a
}

// Has returns true if any of the given flag names are present.
func (a Args) Has(names ...string) bool {
	for _, name := range names {
		if _, ok := a.flags[name]; ok {
			return true
		}
	}
	return false
}

// String returns the first value for the given flag names, or empty string.
func (a Args) String(names ...string) string {
	for _, name := range names {
		if vals, ok := a.flags[name]; ok && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// Int returns the first value as int for the given flag names, or 0.
func (a Args) Int(names ...string) int {
	s := a.String(names...)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

// IntOr returns the first value as int, or the default if not present/invalid.
func (a Args) IntOr(def int, names ...string) int {
	for _, name := range names {
		if vals, ok := a.flags[name]; ok && len(vals) > 0 {
			if v, err := strconv.Atoi(vals[0]); err == nil {
				return v
			}
		}
	}
	return def
}

// List returns all values for the given flag names.
func (a Args) List(names ...string) []string {
	for _, name := range names {
		if vals, ok := a.flags[name]; ok {
			return vals
		}
	}
	return nil
}

// First returns the first positional arg, or empty string.
func (a Args) First() string {
	if len(a.Positional) > 0 {
		return a.Positional[0]
	}
	return ""
}

// Rest returns all positional args after the first.
func (a Args) Rest() []string {
	if len(a.Positional) > 1 {
		return a.Positional[1:]
	}
	return nil
}

// Joined returns all positional args joined with a space.
func (a Args) Joined() string {
	return strings.Join(a.Positional, " ")
}
