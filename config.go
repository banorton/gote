package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	NoteDir string
	Editor  string
}

func defaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	return Config{
		NoteDir: filepath.Join(homeDir, "gotes"),
		Editor:  "vim",
	}
}

func configDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".gote")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func saveConfig(cfg Config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("unable to create config directory: %w", err)
	}
	f, err := os.Create(configPath())
	if err != nil {
		return fmt.Errorf("unable to create config file: %w", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
}

func loadConfig() (Config, error) {
	var cfg Config

	goteDir := filepath.Dir(configPath())
	if err := os.MkdirAll(goteDir, 0755); err != nil {
		return cfg, err
	}

	f, err := os.Open(configPath())
	if os.IsNotExist(err) {
		if err := saveConfig(defaultConfig()); err != nil {
			return defaultConfig(), err
		}
		return defaultConfig(), nil
	} else if err != nil {
		return defaultConfig(), err
	}

	defer f.Close()

	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		_ = saveConfig(defaultConfig())
		return defaultConfig(), nil
	}

	return cfg, nil
}
