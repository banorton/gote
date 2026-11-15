package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/banorton/gote/internal/paths"
)

type Config struct {
	NotesDir   string         `yaml:"notes_dir"`
	Editor     string         `yaml:"editor"`
	Server     bool           `yaml:"server"`
	DiagramDir string         `yaml:"diagram_dir"`
	Semantic   map[string]any `yaml:"semantic"`
}

func Path() string { return filepath.Join(paths.ConfigDir(), "config.yaml") }

func Load() (*Config, error) {
	p := Path()
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("config file %s not found: create it with at least `notes_dir: ~/notes`", p)
		}
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	c := Config{Server: true, DiagramDir: "diagrams"}
	if err := yaml.UnmarshalWithOptions(data, &c, yaml.Strict()); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", p, err)
	}
	if c.NotesDir == "" {
		return nil, fmt.Errorf("config %s: missing required key notes_dir", p)
	}
	if c.NotesDir == "~" || strings.HasPrefix(c.NotesDir, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("config %s: expand ~ in notes_dir: %w", p, err)
		}
		c.NotesDir = filepath.Join(h, strings.TrimPrefix(c.NotesDir, "~"))
	}
	st, err := os.Stat(c.NotesDir)
	if err != nil || !st.IsDir() {
		return nil, fmt.Errorf("config %s: notes_dir %s does not exist or is not a directory", p, c.NotesDir)
	}
	if c.Editor == "" {
		c.Editor = os.Getenv("EDITOR")
	}
	if c.Editor == "" {
		c.Editor = "vi"
	}
	return &c, nil
}
