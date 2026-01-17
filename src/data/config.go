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
	FancyUI         bool   `json:"fancyUI"`
	TimestampNotes  string `json:"timestampNotes"`  // "none", "date", "datetime"
	DefaultPageSize int    `json:"defaultPageSize"` // default number of results to show
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

	goteDir := filepath.Dir(configPath())
	if err := os.MkdirAll(goteDir, 0755); err != nil {
		return cfg, err
	}

	f, err := os.Open(configPath())
	if os.IsNotExist(err) {
		if err := SaveConfig(DefaultConfig()); err != nil {
			return DefaultConfig(), err
		}
		return DefaultConfig(), nil
	} else if err != nil {
		return DefaultConfig(), err
	}

	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		_ = SaveConfig(DefaultConfig())
		return DefaultConfig(), nil
	}

	return cfg, nil
}

func FormatConfigFile() error {
	return FormatJSONFile(configPath())
}