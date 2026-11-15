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
