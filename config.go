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

func formatConfigFile() error {
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

func openConfigInEditor() {
	cfgPath := configPath()
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	editor := cfg.Editor
	if editor == "" {
		editor = "vim"
	}

	if err := openFileInEditor(editor, cfgPath), err != nil {
		fmt.Println(err)
	}
}

func printConfig() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err.Error())
		return
	}

	fmt.Println("Config settings:")
	prettyPrintJSON(cfg)
}
