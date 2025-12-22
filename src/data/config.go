package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	NoteDir string `json:"noteDir"`
	Editor  string `json:"editor"`
}

func DefaultConfig() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("could not determine home directory: %v", err))
	}
	return Config{
		NoteDir: filepath.Join(homeDir, "gotes"),
		Editor:  "vim",
	}
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
	f, err := os.Create(configPath())
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer f.Close()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("unable to write config: %w", err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return fmt.Errorf("unable to write newline: %w", err)
	}
	return nil
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
	cfgPath := configPath()
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}
	pretty, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal pretty config: %w", err)
	}
	if err := os.WriteFile(cfgPath, pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty config: %w", err)
	}
	return nil
}