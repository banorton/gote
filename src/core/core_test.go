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
