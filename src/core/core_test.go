package core

import (
	"os"
	"path/filepath"
	"testing"

	"gote/src/data"
)

// testEnv sets up a test environment and returns a cleanup function
func testEnv(t *testing.T) (goteDir string, notesDir string, cleanup func()) {
	t.Helper()

	goteDir, err := os.MkdirTemp("", "gote-test-*")
	if err != nil {
		t.Fatal(err)
	}

	notesDir = filepath.Join(goteDir, "notes")
	os.MkdirAll(notesDir, 0755)

	// Override GoteDir
	origGoteDir := data.GoteDir
	data.GoteDir = func() string { return goteDir }

	// Create default config pointing to our test notes dir
	cfg := data.Config{NoteDir: notesDir, Editor: "vim"}
	data.SaveConfig(cfg)

	cleanup = func() {
		data.GoteDir = origGoteDir
		os.RemoveAll(goteDir)
	}

	return goteDir, notesDir, cleanup
}

// createTestNote creates a test note and indexes it
func createTestNote(t *testing.T, notesDir, name, content string) {
	t.Helper()
	notePath := filepath.Join(notesDir, name+".md")
	err := os.WriteFile(notePath, []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Index it
	info, _ := os.Stat(notePath)
	meta, _ := data.BuildNoteMeta(notePath, info)
	index, _ := data.LoadIndex()
	index[name] = meta
	data.SaveIndex(index)
	data.UpdateTagsIndex(index)
}

// --- Search tests ---

func TestSearchNotesByTitle(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "project-alpha", ".work\nAlpha project notes")
	createTestNote(t, notesDir, "project-beta", ".work\nBeta project notes")
	createTestNote(t, notesDir, "personal-journal", ".personal\nMy journal")

	t.Run("finds matching notes", func(t *testing.T) {
		results, err := SearchNotesByTitle("project", -1)
		if err != nil {
			t.Fatalf("SearchNotesByTitle failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		results, err := SearchNotesByTitle("PROJECT", -1)
		if err != nil {
			t.Fatalf("SearchNotesByTitle failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		results, err := SearchNotesByTitle("nonexistent", -1)
		if err != nil {
			t.Fatalf("SearchNotesByTitle failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results, got %d", len(results))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := SearchNotesByTitle("project", 1)
		if err != nil {
			t.Fatalf("SearchNotesByTitle failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})
}

func TestSearchNotesByTags(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "note1", ".work.urgent\nUrgent work")
	createTestNote(t, notesDir, "note2", ".work.project\nWork project")
	createTestNote(t, notesDir, "note3", ".personal\nPersonal stuff")

	t.Run("single tag", func(t *testing.T) {
		results, err := SearchNotesByTags([]string{"work"}, -1)
		if err != nil {
			t.Fatalf("SearchNotesByTags failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})

	t.Run("multiple tags scores higher", func(t *testing.T) {
		results, err := SearchNotesByTags([]string{"work", "urgent"}, -1)
		if err != nil {
			t.Fatalf("SearchNotesByTags failed: %v", err)
		}
		if len(results) < 1 {
			t.Fatal("Expected at least 1 result")
		}
		// note1 has both tags, should score 2
		for _, r := range results {
			if r.Title == "note1" && r.Score != 2 {
				t.Errorf("note1 should have score 2, got %d", r.Score)
			}
		}
	})

	t.Run("nonexistent tag", func(t *testing.T) {
		results, err := SearchNotesByTags([]string{"nonexistent"}, -1)
		if err != nil {
			t.Fatalf("SearchNotesByTags failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results, got %d", len(results))
		}
	})
}

// --- Pins tests ---

func TestPinOperations(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "test-note", ".tag\nContent")

	t.Run("PinNote", func(t *testing.T) {
		err := PinNote("test-note")
		if err != nil {
			t.Fatalf("PinNote failed: %v", err)
		}

		pins, _ := ListPinnedNotes()
		found := false
		for _, p := range pins {
			if p == "test-note" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Note should be in pinned list")
		}
	})

	t.Run("PinNote already pinned is idempotent", func(t *testing.T) {
		err := PinNote("test-note")
		if err != nil {
			t.Error("Should not error when pinning already pinned note (idempotent)")
		}
	})

	t.Run("PinNote nonexistent", func(t *testing.T) {
		err := PinNote("nonexistent")
		if err == nil {
			t.Error("Should error when pinning nonexistent note")
		}
	})

	t.Run("UnpinNote", func(t *testing.T) {
		err := UnpinNote("test-note")
		if err != nil {
			t.Fatalf("UnpinNote failed: %v", err)
		}

		pins, _ := ListPinnedNotes()
		for _, p := range pins {
			if p == "test-note" {
				t.Error("Note should not be in pinned list")
			}
		}
	})

	t.Run("UnpinNote not pinned", func(t *testing.T) {
		err := UnpinNote("test-note")
		if err == nil {
			t.Error("Should error when unpinning not-pinned note")
		}
	})
}

// --- Trash tests ---

func TestTrashOperations(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "to-delete", ".tag\nContent")

	t.Run("DeleteNote", func(t *testing.T) {
		err := DeleteNote("to-delete")
		if err != nil {
			t.Fatalf("DeleteNote failed: %v", err)
		}

		// Should be gone from index
		index, err := data.LoadIndex()
		if err != nil {
			t.Fatalf("LoadIndex failed: %v", err)
		}
		if _, exists := index["to-delete"]; exists {
			t.Error("Note should be removed from index")
		}

		// Should be in trash
		trashed, _ := ListTrashedNotes()
		found := false
		for _, n := range trashed {
			if n == "to-delete" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Note should be in trash")
		}
	})

	t.Run("DeleteNote nonexistent", func(t *testing.T) {
		err := DeleteNote("nonexistent")
		if err == nil {
			t.Error("Should error when deleting nonexistent note")
		}
	})

	t.Run("RecoverNote", func(t *testing.T) {
		err := RecoverNote("to-delete")
		if err != nil {
			t.Fatalf("RecoverNote failed: %v", err)
		}

		// Should be back in index
		index, err := data.LoadIndex()
		if err != nil {
			t.Fatalf("LoadIndex failed: %v", err)
		}
		if _, exists := index["to-delete"]; !exists {
			t.Error("Note should be back in index")
		}
	})
}

// --- Tags tests ---

func TestTagOperations(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "note1", ".work.urgent\nContent")
	createTestNote(t, notesDir, "note2", ".work.project\nContent")
	createTestNote(t, notesDir, "note3", ".work.project\nContent")

	t.Run("ListTags", func(t *testing.T) {
		tags, err := ListTags()
		if err != nil {
			t.Fatalf("ListTags failed: %v", err)
		}

		if _, exists := tags["work"]; !exists {
			t.Error("Should have 'work' tag")
		}
		if tags["work"].Count != 3 {
			t.Errorf("work count = %d, want 3", tags["work"].Count)
		}
	})

	t.Run("GetPopularTags", func(t *testing.T) {
		tags, err := GetPopularTags(2)
		if err != nil {
			t.Fatalf("GetPopularTags failed: %v", err)
		}

		if len(tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(tags))
		}

		// First should be 'work' (3 uses) or 'project' (2 uses)
		if tags[0].Tag != "work" && tags[0].Tag != "project" {
			t.Errorf("First tag should be 'work' or 'project', got %s", tags[0].Tag)
		}
	})
}

// --- Recent tests ---

func TestGetRecentNotes(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "note1", ".tag\nFirst")
	createTestNote(t, notesDir, "note2", ".tag\nSecond")
	createTestNote(t, notesDir, "note3", ".tag\nThird")

	t.Run("returns all notes", func(t *testing.T) {
		notes, err := GetRecentNotes(-1)
		if err != nil {
			t.Fatalf("GetRecentNotes failed: %v", err)
		}
		if len(notes) != 3 {
			t.Errorf("Expected 3 notes, got %d", len(notes))
		}
	})

	t.Run("sorted by last visited", func(t *testing.T) {
		// Manually set LastVisited to control sort order
		index, _ := data.LoadIndex()
		m1 := index["note1"]
		m2 := index["note2"]
		m3 := index["note3"]
		m1.LastVisited = "991231.235959" // Most recent
		m2.LastVisited = "990101.000000" // Oldest
		m3.LastVisited = "990601.120000" // Middle
		index["note1"] = m1
		index["note2"] = m2
		index["note3"] = m3
		data.SaveIndex(index)

		notes, err := GetRecentNotes(-1)
		if err != nil {
			t.Fatalf("GetRecentNotes failed: %v", err)
		}
		if len(notes) < 3 {
			t.Fatal("Expected 3 notes")
		}
		if notes[0].Title != "note1" {
			t.Errorf("First note should be 'note1', got %s", notes[0].Title)
		}
		if notes[1].Title != "note3" {
			t.Errorf("Second note should be 'note3', got %s", notes[1].Title)
		}
		if notes[2].Title != "note2" {
			t.Errorf("Third note should be 'note2', got %s", notes[2].Title)
		}
	})
}

// --- UpdateLastVisited tests ---

func TestUpdateLastVisited(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "test-note", ".tag\nContent")

	t.Run("updates LastVisited timestamp", func(t *testing.T) {
		err := UpdateLastVisited("test-note")
		if err != nil {
			t.Fatalf("UpdateLastVisited failed: %v", err)
		}

		index, _ := data.LoadIndex()
		meta := index["test-note"]
		if meta.LastVisited == "" {
			t.Error("LastVisited should be set")
		}
	})

	t.Run("nonexistent note returns nil", func(t *testing.T) {
		err := UpdateLastVisited("nonexistent")
		if err != nil {
			t.Errorf("Should return nil for nonexistent note, got %v", err)
		}
	})
}

// --- Date search tests ---

func TestParseDateInput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantStart string
		wantEnd   string
		wantErr   bool
	}{
		{"year only", "24", "240101.000000", "241231.235959", false},
		{"month", "2412", "241201.000000", "241231.235959", false},
		{"day", "241223", "241223.000000", "241223.235959", false},
		{"hour", "241223.15", "241223.150000", "241223.155959", false},
		{"minute", "241223.1530", "241223.153000", "241223.153059", false},
		{"second", "241223.153045", "241223.153045", "241223.153045", false},
		{"empty", "", "", "", true},
		{"invalid chars", "24abc", "", "", true},
		{"invalid length", "12345", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dr, err := ParseDateInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDateInput(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if dr.Start != tt.wantStart {
					t.Errorf("Start = %q, want %q", dr.Start, tt.wantStart)
				}
				if dr.End != tt.wantEnd {
					t.Errorf("End = %q, want %q", dr.End, tt.wantEnd)
				}
			}
		})
	}
}

func TestSearchNotesByDate(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "old-note", ".tag\nOld")
	createTestNote(t, notesDir, "new-note", ".tag\nNew")

	// Set specific Created dates for testing
	index, _ := data.LoadIndex()
	m1 := index["old-note"]
	m2 := index["new-note"]
	m1.Created = "240101.120000"
	m2.Created = "241215.120000"
	index["old-note"] = m1
	index["new-note"] = m2
	data.SaveIndex(index)

	t.Run("finds notes in range", func(t *testing.T) {
		results, err := SearchNotesByDate([]string{"2412"}, true, -1)
		if err != nil {
			t.Fatalf("SearchNotesByDate failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
		if len(results) > 0 && results[0].Title != "new-note" {
			t.Errorf("Expected 'new-note', got %s", results[0].Title)
		}
	})

	t.Run("finds all in year", func(t *testing.T) {
		results, err := SearchNotesByDate([]string{"24"}, true, -1)
		if err != nil {
			t.Fatalf("SearchNotesByDate failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})
}

// --- RenameNote tests ---

func TestRenameNote(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "original", ".tag\nContent")

	t.Run("renames successfully", func(t *testing.T) {
		err := RenameNote("original", "renamed")
		if err != nil {
			t.Fatalf("RenameNote failed: %v", err)
		}

		index, _ := data.LoadIndex()
		if _, exists := index["original"]; exists {
			t.Error("Old name should not exist in index")
		}
		if _, exists := index["renamed"]; !exists {
			t.Error("New name should exist in index")
		}

		// Check file was renamed
		oldPath := filepath.Join(notesDir, "original.md")
		newPath := filepath.Join(notesDir, "renamed.md")
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Error("Old file should not exist")
		}
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			t.Error("New file should exist")
		}
	})

	t.Run("fails for nonexistent", func(t *testing.T) {
		err := RenameNote("nonexistent", "newname")
		if err == nil {
			t.Error("Should error for nonexistent note")
		}
	})

	t.Run("fails for invalid name", func(t *testing.T) {
		createTestNote(t, notesDir, "valid", ".tag\nContent")
		err := RenameNote("valid", "../invalid")
		if err == nil {
			t.Error("Should error for invalid new name")
		}
	})
}

func TestFilterNotesByTags(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	// Create test notes with different tag combinations
	createTestNote(t, notesDir, "note1", ".work.urgent\nContent 1")
	createTestNote(t, notesDir, "note2", ".work.project\nContent 2")
	createTestNote(t, notesDir, "note3", ".personal\nContent 3")
	createTestNote(t, notesDir, "note4", ".work.urgent.project\nContent 4")

	t.Run("filters by single tag", func(t *testing.T) {
		results, err := FilterNotesByTags([]string{"personal"}, -1)
		if err != nil {
			t.Fatalf("FilterNotesByTags failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
		if len(results) > 0 && results[0].Title != "note3" {
			t.Errorf("Expected note3, got %s", results[0].Title)
		}
	})

	t.Run("filters by multiple tags AND logic", func(t *testing.T) {
		results, err := FilterNotesByTags([]string{"work", "urgent"}, -1)
		if err != nil {
			t.Fatalf("FilterNotesByTags failed: %v", err)
		}
		// Should find note1 (work+urgent) and note4 (work+urgent+project)
		if len(results) != 2 {
			t.Errorf("Expected 2 results for work+urgent, got %d", len(results))
		}
	})

	t.Run("returns empty for nonexistent tag", func(t *testing.T) {
		results, err := FilterNotesByTags([]string{"nonexistent"}, -1)
		if err != nil {
			t.Fatalf("FilterNotesByTags failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results for nonexistent tag, got %d", len(results))
		}
	})

	t.Run("returns empty when no notes have all tags", func(t *testing.T) {
		results, err := FilterNotesByTags([]string{"personal", "work"}, -1)
		if err != nil {
			t.Fatalf("FilterNotesByTags failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results (no note has both personal and work), got %d", len(results))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := FilterNotesByTags([]string{"work"}, 1)
		if err != nil {
			t.Fatalf("FilterNotesByTags failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result with limit=1, got %d", len(results))
		}
	})
}
