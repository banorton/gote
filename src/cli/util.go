package cli

import (
	"fmt"

	"gote/src/data"
)

// LoadConfigAndUI loads the config and creates a UI instance.
// Returns early with error message if config loading fails.
// This is a common pattern used across most CLI commands.
func LoadConfigAndUI() (data.Config, *UI, bool) {
	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return data.Config{}, nil, false
	}
	return cfg, NewUI(cfg.FancyUI), true
}
