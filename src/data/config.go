package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	NoteDir         string `json:"noteDir"`
	Editor          string `json:"editor"`
	Interface       string `json:"interface"`       // "default", "minimal", "tui"
	TimestampNotes  string `json:"timestampNotes"`  // "none", "date", "datetime"
	DefaultPageSize int    `json:"defaultPageSize"` // default number of results to show
}

// IsTUI returns true if the interface mode is "tui"
func (c Config) IsTUI() bool {
	return c.Interface == "tui"
}

// IsMinimal returns true if the interface mode is "minimal"
func (c Config) IsMinimal() bool {
	return c.Interface == "minimal"
}

func DefaultConfig() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("could not determine home directory: %v", err))
	}
	return Config{
		NoteDir:         filepath.Join(homeDir, "gotes"),
		Editor:          "vim",
		DefaultPageSize: 10,
	}
}

// PageSize returns the effective page size, using default if not set
func (c Config) PageSize() int {
	if c.DefaultPageSize <= 0 {
		return 10
	}
	return c.DefaultPageSize
}

// GoteDir returns the gote config directory. It's a variable so tests can override it.
var GoteDir = func() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("could not determine home directory: %v", err))
	}
	return filepath.Join(homeDir, ".gote")
}

func configPath() string {
	return filepath.Join(GoteDir(), "config.json")
}

func SaveConfig(cfg Config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}
	return AtomicWriteJSON(configPath(), cfg)
}

func LoadConfig() (Config, error) {
	var cfg Config

	goteDir := GoteDir()
	if err := os.MkdirAll(goteDir, 0755); err != nil {
		return cfg, err
	}

	raw, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		if err := SaveConfig(DefaultConfig()); err != nil {
			return DefaultConfig(), err
		}
		return DefaultConfig(), nil
	} else if err != nil {
		return DefaultConfig(), err
	}

	// Check for fancyUI migration
	var rawMap map[string]interface{}
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		_ = SaveConfig(DefaultConfig())
		return DefaultConfig(), nil
	}

	if err := json.Unmarshal(raw, &cfg); err != nil {
		_ = SaveConfig(DefaultConfig())
		return DefaultConfig(), nil
	}

	// Migrate fancyUI -> interface
	if fancyVal, exists := rawMap["fancyUI"]; exists {
		if fancy, ok := fancyVal.(bool); ok && fancy {
			cfg.Interface = "tui"
		}
		if err := SaveConfig(cfg); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func FormatConfigFile() error {
	return FormatJSONFile(configPath())
}
