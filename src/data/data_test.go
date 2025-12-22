package data

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// testDir creates a temp directory and returns a cleanup function
func testDir(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "gote-test-*")
	if err != nil {
		t.Fatal(err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

// --- ParseTags tests ---

func TestParseTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple tags", ".work.project", []string{"work", "project"}},
		{"no leading dot", "work.project", []string{"work", "project"}},
		{"with hash", "#.work.project", []string{"work", "project"}},
		{"with brackets", "[work].[project]", []string{"work", "project"}},
		{"with pipes", "|work|.project", []string{"work", "project"}},
		{"empty", "", []string{}},
		{"only dots", "...", []string{}},
		{"mixed case", ".Work.PROJECT", []string{"work", "project"}},
		{"with spaces", ". work . project ", []string{"work", "project"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTags(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return // both empty, ok
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTags(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// --- Index tests ---

func TestIndexOperations(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	// Override GoteDir for testing
	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	t.Run("SaveIndex and LoadIndex", func(t *testing.T) {
		index := map[string]NoteMeta{
			"test-note": {
				FilePath:  "/path/to/test-note.md",
				Title:     "test-note",
				Created:   "241201.120000",
				WordCount: 100,
				CharCount: 500,
				Tags:      []string{"work", "project"},
			},
		}

		err := SaveIndex(index)
		if err != nil {
			t.Fatalf("SaveIndex failed: %v", err)
		}

		loaded := LoadIndex()
		if !reflect.DeepEqual(loaded, index) {
			t.Errorf("LoadIndex() = %v, want %v", loaded, index)
		}
	})

	t.Run("LoadIndex returns empty map for missing file", func(t *testing.T) {
		// Use a different dir
		emptyDir, cleanup2 := testDir(t)
		defer cleanup2()
		GoteDir = func() string { return emptyDir }

		loaded := LoadIndex()
		if len(loaded) != 0 {
			t.Errorf("LoadIndex() should return empty map, got %v", loaded)
		}
	})
}

func TestBuildNoteMeta(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	// Create a test note
	notePath := filepath.Join(dir, "test-note.md")
	content := `.work.project
This is the content of my test note.
It has multiple lines.`
	err := os.WriteFile(notePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(notePath)
	if err != nil {
		t.Fatal(err)
	}

	meta, err := BuildNoteMeta(notePath, info)
	if err != nil {
		t.Fatalf("BuildNoteMeta failed: %v", err)
	}

	if meta.Title != "test-note" {
		t.Errorf("Title = %q, want %q", meta.Title, "test-note")
	}
	if meta.FilePath != notePath {
		t.Errorf("FilePath = %q, want %q", meta.FilePath, notePath)
	}
	if !reflect.DeepEqual(meta.Tags, []string{"work", "project"}) {
		t.Errorf("Tags = %v, want [work project]", meta.Tags)
	}
	if meta.WordCount != 13 {
		t.Errorf("WordCount = %d, want 13", meta.WordCount)
	}
}

// --- Tags tests ---

func TestTagsOperations(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	t.Run("UpdateTagsIndex and LoadTags", func(t *testing.T) {
		index := map[string]NoteMeta{
			"note1": {FilePath: "/note1.md", Title: "note1", Tags: []string{"work", "urgent"}},
			"note2": {FilePath: "/note2.md", Title: "note2", Tags: []string{"work", "project"}},
			"note3": {FilePath: "/note3.md", Title: "note3", Tags: []string{"personal"}},
		}

		err := UpdateTagsIndex(index)
		if err != nil {
			t.Fatalf("UpdateTagsIndex failed: %v", err)
		}

		tags, err := LoadTags()
		if err != nil {
			t.Fatalf("LoadTags failed: %v", err)
		}

		if tags["work"].Count != 2 {
			t.Errorf("work count = %d, want 2", tags["work"].Count)
		}
		if tags["urgent"].Count != 1 {
			t.Errorf("urgent count = %d, want 1", tags["urgent"].Count)
		}
		if tags["personal"].Count != 1 {
			t.Errorf("personal count = %d, want 1", tags["personal"].Count)
		}
	})
}

// --- Pins tests ---

func TestPinsOperations(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	t.Run("SavePins and LoadPins", func(t *testing.T) {
		pins := map[string]EmptyStruct{
			"note1": {},
			"note2": {},
		}

		err := SavePins(pins)
		if err != nil {
			t.Fatalf("SavePins failed: %v", err)
		}

		loaded, err := LoadPins()
		if err != nil {
			t.Fatalf("LoadPins failed: %v", err)
		}

		if !reflect.DeepEqual(loaded, pins) {
			t.Errorf("LoadPins() = %v, want %v", loaded, pins)
		}
	})

	t.Run("LoadPins returns empty map for missing file", func(t *testing.T) {
		emptyDir, cleanup2 := testDir(t)
		defer cleanup2()
		GoteDir = func() string { return emptyDir }

		loaded, err := LoadPins()
		if err == nil {
			t.Log("LoadPins returned no error for missing file (acceptable)")
		}
		if loaded == nil {
			loaded = make(map[string]EmptyStruct)
		}
		if len(loaded) != 0 {
			t.Errorf("LoadPins() should return empty map, got %v", loaded)
		}
	})
}

// --- Trash tests ---

func TestTrashOperations(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	notesDir := filepath.Join(dir, "notes")
	os.MkdirAll(notesDir, 0755)

	t.Run("TrashNote and RecoverNote", func(t *testing.T) {
		// Create a note
		notePath := filepath.Join(notesDir, "test-note.md")
		err := os.WriteFile(notePath, []byte(".tag1\nContent"), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Index it
		info, _ := os.Stat(notePath)
		meta, _ := BuildNoteMeta(notePath, info)
		index := map[string]NoteMeta{"test-note": meta}
		SaveIndex(index)

		// Trash it
		err = TrashNote("test-note", meta)
		if err != nil {
			t.Fatalf("TrashNote failed: %v", err)
		}

		// Verify it's gone from notes dir
		if _, err := os.Stat(notePath); !os.IsNotExist(err) {
			t.Error("Note should be removed from notes dir")
		}

		// Verify it's in trash
		trashPath := filepath.Join(TrashPath(), "test-note.md")
		if _, err := os.Stat(trashPath); os.IsNotExist(err) {
			t.Error("Note should be in trash")
		}

		// Verify index is updated
		loaded := LoadIndex()
		if _, exists := loaded["test-note"]; exists {
			t.Error("Note should be removed from index")
		}

		// Recover it
		err = RecoverNote("test-note", notesDir)
		if err != nil {
			t.Fatalf("RecoverNote failed: %v", err)
		}

		// Verify it's back
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			t.Error("Note should be recovered to notes dir")
		}

		// Verify index is updated
		loaded = LoadIndex()
		if _, exists := loaded["test-note"]; !exists {
			t.Error("Note should be back in index")
		}
	})

	t.Run("ListTrashedNotes", func(t *testing.T) {
		// Create some trashed notes
		trashDir := TrashPath()
		os.MkdirAll(trashDir, 0755)
		os.WriteFile(filepath.Join(trashDir, "trashed1.md"), []byte("content"), 0644)
		os.WriteFile(filepath.Join(trashDir, "trashed2.md"), []byte("content"), 0644)

		notes, err := ListTrashedNotes()
		if err != nil {
			t.Fatalf("ListTrashedNotes failed: %v", err)
		}

		if len(notes) < 2 {
			t.Errorf("Expected at least 2 trashed notes, got %d", len(notes))
		}
	})

	t.Run("SearchTrash", func(t *testing.T) {
		results, err := SearchTrash("trashed")
		if err != nil {
			t.Fatalf("SearchTrash failed: %v", err)
		}

		if len(results) < 2 {
			t.Errorf("Expected at least 2 results, got %d", len(results))
		}
	})
}

// --- Config tests ---

func TestConfigOperations(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	t.Run("SaveConfig and LoadConfig", func(t *testing.T) {
		cfg := Config{
			NoteDir: "/custom/notes",
			Editor:  "nano",
		}

		err := SaveConfig(cfg)
		if err != nil {
			t.Fatalf("SaveConfig failed: %v", err)
		}

		loaded, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if loaded.NoteDir != cfg.NoteDir {
			t.Errorf("NoteDir = %q, want %q", loaded.NoteDir, cfg.NoteDir)
		}
		if loaded.Editor != cfg.Editor {
			t.Errorf("Editor = %q, want %q", loaded.Editor, cfg.Editor)
		}
	})

	t.Run("LoadConfig creates default for missing file", func(t *testing.T) {
		emptyDir, cleanup2 := testDir(t)
		defer cleanup2()
		GoteDir = func() string { return emptyDir }

		loaded, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		// Should have default values
		if loaded.Editor != "vim" {
			t.Errorf("Default editor = %q, want vim", loaded.Editor)
		}
	})
}
