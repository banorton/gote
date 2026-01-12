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
	return cfg, NewUI(cfg.FancyUI), true
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
