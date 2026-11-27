package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T, yaml string) (cfgPath, notes string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	notes = filepath.Join(dir, "notes")
	os.MkdirAll(notes, 0o755)
	cfgPath = filepath.Join(dir, "gote", "config.yaml")
	if yaml != "" {
		os.MkdirAll(filepath.Dir(cfgPath), 0o755)
		os.WriteFile(cfgPath, []byte(strings.ReplaceAll(yaml, "NOTES", notes)), 0o644)
	}
	return
}

func TestLoadOK(t *testing.T) {
	t.Setenv("EDITOR", "ed")
	_, notes := setup(t, "notes_dir: NOTES\n")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.NotesDir != notes || c.Editor != "ed" || !c.Server || c.DiagramDir != "diagrams" {
		t.Fatalf("%+v", c)
	}
}

func TestEditorDefaults(t *testing.T) {
	t.Setenv("EDITOR", "")
	setup(t, "notes_dir: NOTES\n")
	c, _ := Load()
	if c.Editor != "vi" {
		t.Fatal(c.Editor)
	}
}

func TestExplicitValues(t *testing.T) {
	setup(t, "notes_dir: NOTES\neditor: nvim\nserver: false\ndiagram_dir: d\nsemantic: {}\n")
	c, err := Load()
	if err != nil || c.Editor != "nvim" || c.Server || c.DiagramDir != "d" {
		t.Fatalf("%v %+v", err, c)
	}
}

func TestTildeExpanded(t *testing.T) {
	cfg, _ := setup(t, "notes_dir: ~/notes\n")
	_ = cfg
	if c, err := Load(); err != nil || !filepath.IsAbs(c.NotesDir) || strings.Contains(c.NotesDir, "~") {
		t.Fatalf("%v %+v", err, c)
	}
}

func TestMissingFile(t *testing.T) {
	cfg, _ := setup(t, "")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), cfg) {
		t.Fatalf("%v", err)
	}
}

func TestMissingNotesDirKey(t *testing.T) {
	cfg, _ := setup(t, "editor: vi\n")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), cfg) || !strings.Contains(err.Error(), "notes_dir") {
		t.Fatalf("%v", err)
	}
}

func TestNotesDirMustExist(t *testing.T) {
	setup(t, "notes_dir: /nonexistent/xyz\n")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "/nonexistent/xyz") {
		t.Fatalf("%v", err)
	}
}

func TestUnknownKey(t *testing.T) {
	setup(t, "notes_dir: NOTES\nnotes_dri: x\n")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "notes_dri") {
		t.Fatalf("%v", err)
	}
}

func semCfg(t *testing.T, block string) (*Config, error) {
	t.Helper()
	setup(t, "notes_dir: NOTES\nsemantic:\n"+block)
	return Load()
}

func TestSemanticDisabledWhenAbsentOrEmpty(t *testing.T) {
	setup(t, "notes_dir: NOTES\n")
	c, _ := Load()
	if c.Semantic.Enabled() {
		t.Fatal("absent should be disabled")
	}
	c, err := semCfg(t, "  {}\n")
	if err != nil || c.Semantic.Enabled() {
		t.Fatalf("%v %+v", err, c.Semantic)
	}
}

func TestSemanticDefaults(t *testing.T) {
	c, err := semCfg(t, "  provider: openai\n  api_key_env: K\n")
	if err != nil || !c.Semantic.Enabled() {
		t.Fatalf("%v", err)
	}
	s := c.Semantic
	if s.BaseURL != "https://api.openai.com/v1" || s.Model != "text-embedding-3-small" || s.BatchSize != 64 {
		t.Fatalf("%+v", s)
	}
}

func TestSemanticValidationNamesKey(t *testing.T) {
	cases := map[string]struct{ block, key string }{
		"badprovider": {"  provider: foo\n", "semantic.provider"},
		"noprovider":  {"  model: x\n", "semantic.provider"},
		"nokey":       {"  provider: openai\n", "semantic.api_key"},
		"twokeys":     {"  provider: openai\n  api_key: a\n  api_key_env: B\n", "semantic.api_key"},
		"cmdnocmd":    {"  provider: command\n  model: m\n", "semantic.embed_cmd"},
		"cmdnomodel":  {"  provider: command\n  embed_cmd: x\n", "semantic.model"},
		"cmdwithkey":  {"  provider: command\n  model: m\n  embed_cmd: x\n  api_key: a\n", "semantic.api_key"},
		"openaicmd":   {"  provider: openai\n  api_key: a\n  embed_cmd: x\n", "semantic.embed_cmd"},
		"badbatch":    {"  provider: openai\n  api_key: a\n  batch_size: -1\n", "semantic.batch_size"},
		"baddims":     {"  provider: openai\n  api_key: a\n  dimensions: -1\n", "semantic.dimensions"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := semCfg(t, c.block)
			if err == nil || !strings.Contains(err.Error(), c.key) {
				t.Fatalf("want %s in %v", c.key, err)
			}
		})
	}
}

func TestSemanticCommandProvider(t *testing.T) {
	c, err := semCfg(t, "  provider: command\n  model: m\n  embed_cmd: ~/bin/x\n  exclude_tags: [private]\n  price_per_mtok: 0.02\n")
	if err != nil || c.Semantic.EmbedCmd != "~/bin/x" || c.Semantic.ExcludeTags[0] != "private" || c.Semantic.PricePerMTok != 0.02 {
		t.Fatalf("%v %+v", err, c.Semantic)
	}
}

func TestKeyFileWarning(t *testing.T) {
	cfg, _ := setup(t, "notes_dir: NOTES\nsemantic:\n  provider: openai\n  api_key: sk-secret\n")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	w := c.KeyFileWarning()
	if !strings.Contains(w, cfg) || strings.Contains(w, "sk-secret") {
		t.Fatalf("%q", w)
	}
	os.Chmod(cfg, 0o600)
	if w := c.KeyFileWarning(); w != "" {
		t.Fatalf("%q", w)
	}
}

func TestDiagramPNGDefaultAndValidation(t *testing.T) {
	setup(t, "notes_dir: NOTES\n")
	c, err := Load()
	if err != nil || c.DiagramPNG != "cache" {
		t.Fatalf("%+v %v", c, err)
	}
	for _, v := range []string{"cache", "sidecar", "off"} {
		setup(t, "notes_dir: NOTES\ndiagram_png: "+v+"\n")
		if c, err := Load(); err != nil || c.DiagramPNG != v {
			t.Fatalf("%s: %+v %v", v, c, err)
		}
	}
	setup(t, "notes_dir: NOTES\ndiagram_png: always\n")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "diagram_png") || !strings.Contains(err.Error(), "always") {
		t.Fatalf("%v", err)
	}
}

func TestDateFormatDefaultAndValidation(t *testing.T) {
	setup(t, "notes_dir: NOTES\n")
	c, err := Load()
	if err != nil || c.DateFormat != "iso" {
		t.Fatalf("%+v %v", c, err)
	}
	for _, v := range []string{"iso", "compact"} {
		setup(t, "notes_dir: NOTES\ndate_format: "+v+"\n")
		if c, err := Load(); err != nil || c.DateFormat != v {
			t.Fatalf("%s: %v", v, err)
		}
	}
	setup(t, "notes_dir: NOTES\ndate_format: us\n")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "date_format") || !strings.Contains(err.Error(), "us") {
		t.Fatalf("%v", err)
	}
}
