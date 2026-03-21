package cli

import (
	"fmt"

	"gote/src/core"
	"gote/src/data"
)

// LoadConfigAndUI loads the config and creates a UI instance.
func LoadConfigAndUI() (data.Config, *UI, bool) {
	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return data.Config{}, nil, false
	}
	return cfg, NewUI(cfg.Interface), true
}

// ActionDefaults holds boolean flags for pre-selected menu actions
type ActionDefaults struct {
	Open, Delete, Pin, Unpin, View, Rename bool
}

// resolvePreSelectedAction determines the pre-selected action from positional args and defaults.
// It consumes the action keyword from args.Positional if present.
func resolvePreSelectedAction(args *Args, defaults ActionDefaults) string {
	first := args.First()
	type actionCheck struct {
		keyword string
		flag    bool
	}
	checks := []actionCheck{
		{"open", defaults.Open},
		{"delete", defaults.Delete},
		{"pin", defaults.Pin},
		{"unpin", defaults.Unpin},
		{"view", defaults.View},
		{"rename", defaults.Rename},
	}
	for _, c := range checks {
		if first == c.keyword || c.flag {
			if first == c.keyword {
				args.Positional = args.Positional[1:]
			}
			return c.keyword
		}
	}
	return ""
}

// ResolveNoteName resolves "-" to the last opened note's title.
func ResolveNoteName(name string) (string, error) {
	if name != "-" {
		return name, nil
	}
	notes, err := core.GetRecentNotes(1)
	if err != nil {
		return "", fmt.Errorf("could not get last note: %w", err)
	}
	if len(notes) == 0 {
		return "", fmt.Errorf("no recent notes found")
	}
	return notes[0].Title, nil
}
