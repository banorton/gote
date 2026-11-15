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
	NotesDir   string   `yaml:"notes_dir"`
	Editor     string   `yaml:"editor"`
	Server     bool     `yaml:"server"`
	DiagramDir string   `yaml:"diagram_dir"`
	Semantic   Semantic `yaml:"semantic"`
}

type Semantic struct {
	Provider     string   `yaml:"provider"`
	BaseURL      string   `yaml:"base_url"`
	Model        string   `yaml:"model"`
	Dimensions   int      `yaml:"dimensions"`
	APIKey       string   `yaml:"api_key"`
	APIKeyEnv    string   `yaml:"api_key_env"`
	APIKeyCmd    string   `yaml:"api_key_cmd"`
	EmbedCmd     string   `yaml:"embed_cmd"`
	BatchSize    int      `yaml:"batch_size"`
	ExcludeTags  []string `yaml:"exclude_tags"`
	PricePerMTok float64  `yaml:"price_per_mtok"`
}

const (
	ProviderOpenAI  = "openai"
	ProviderCommand = "command"
)

func (s Semantic) isZero() bool {
	return s.Provider == "" && s.BaseURL == "" && s.Model == "" && s.Dimensions == 0 && s.APIKey == "" &&
		s.APIKeyEnv == "" && s.APIKeyCmd == "" && s.EmbedCmd == "" && s.BatchSize == 0 &&
		len(s.ExcludeTags) == 0 && s.PricePerMTok == 0
}

// Enabled reports whether semantic search is configured; an absent or empty block is disabled.
func (s Semantic) Enabled() bool { return s.Provider != "" }

func (s *Semantic) validate() error {
	if s.isZero() {
		return nil
	}
	if s.BatchSize < 0 {
		return fmt.Errorf("semantic.batch_size: must be >= 0, got %d", s.BatchSize)
	}
	if s.Dimensions < 0 {
		return fmt.Errorf("semantic.dimensions: must be >= 0, got %d", s.Dimensions)
	}
	if s.PricePerMTok < 0 {
		return fmt.Errorf("semantic.price_per_mtok: must be >= 0")
	}
	if s.BatchSize == 0 {
		s.BatchSize = 64
	}
	switch s.Provider {
	case ProviderOpenAI:
		if s.EmbedCmd != "" {
			return fmt.Errorf("semantic.embed_cmd: only valid with provider command")
		}
		n := 0
		for _, v := range []string{s.APIKey, s.APIKeyEnv, s.APIKeyCmd} {
			if v != "" {
				n++
			}
		}
		if n != 1 {
			return fmt.Errorf("semantic.api_key: exactly one of api_key, api_key_env, api_key_cmd is required for provider openai, got %d", n)
		}
		if s.BaseURL == "" {
			s.BaseURL = "https://api.openai.com/v1"
		}
		if s.Model == "" {
			s.Model = "text-embedding-3-small"
		}
	case ProviderCommand:
		if s.APIKey != "" || s.APIKeyEnv != "" || s.APIKeyCmd != "" {
			return fmt.Errorf("semantic.api_key: api key options are only valid with provider openai")
		}
		if s.EmbedCmd == "" {
			return fmt.Errorf("semantic.embed_cmd: required for provider command")
		}
		if s.Model == "" {
			return fmt.Errorf("semantic.model: required for provider command")
		}
	case "":
		return fmt.Errorf("semantic.provider: required (openai or command)")
	default:
		return fmt.Errorf("semantic.provider: must be openai or command, got %q", s.Provider)
	}
	return nil
}

// KeyFileWarning returns a one-line warning when an inline api_key sits in a
// config file readable by group or other; empty otherwise.
func (c *Config) KeyFileWarning() string {
	if c.Semantic.APIKey == "" {
		return ""
	}
	p := Path()
	st, err := os.Stat(p)
	if err != nil || st.Mode().Perm()&0o077 == 0 {
		return ""
	}
	return fmt.Sprintf("gote: warning: %s contains api_key and is accessible by group/other (mode %o); run chmod 600", p, st.Mode().Perm())
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
	if err := c.Semantic.validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", p, err)
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
