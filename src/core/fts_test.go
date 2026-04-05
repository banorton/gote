package core

import (
	"os"
	"path/filepath"
	"testing"

	"gote/src/data"
)

func TestSearchNotesFullText(t *testing.T) {
	goteDir, notesDir, cleanup := testEnv(t)
	defer cleanup()

	// Create test notes
	note1 := filepath.Join(notesDir, "meeting.md")
	note2 := filepath.Join(notesDir, "project.md")
	note3 := filepath.Join(notesDir, "random.md")
	os.WriteFile(note1, []byte("Notes from the weekly meeting about project deadlines"), 0644)
	os.WriteFile(note2, []byte("Project plan for the new feature with running tests"), 0644)
	os.WriteFile(note3, []byte("Some random thoughts about cooking recipes"), 0644)

	// Build FTS index
	index := map[string]data.NoteMeta{
		"meeting": {FilePath: note1, Title: "meeting"},
		"project": {FilePath: note2, Title: "project"},
		"random":  {FilePath: note3, Title: "random"},
	}
	if err := data.IndexAllFTS(notesDir, index); err != nil {
		t.Fatalf("IndexAllFTS failed: %v", err)
	}

	t.Run("finds notes by content", func(t *testing.T) {
		results, err := SearchNotesFullText("deadline", -1)
		if err != nil {
			t.Fatalf("SearchNotesFullText failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least 1 result for 'deadline'")
		}
		if results[0].Title != "meeting" {
			t.Errorf("expected 'meeting' as top result, got %q", results[0].Title)
		}
	})

	t.Run("stemming matches", func(t *testing.T) {
		results, err := SearchNotesFullText("running", -1)
		if err != nil {
			t.Fatalf("SearchNotesFullText failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least 1 result for 'running' (stemmed)")
		}
		// "project" note has "running"
		found := false
		for _, r := range results {
			if r.Title == "project" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'project' in results for 'running'")
		}
	})

	t.Run("no results for unrelated query", func(t *testing.T) {
		results, err := SearchNotesFullText("quantum physics", -1)
		if err != nil {
			t.Fatalf("SearchNotesFullText failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("empty FTS returns nil", func(t *testing.T) {
		// Remove FTS file
		os.Remove(filepath.Join(goteDir, "fts.json"))
		results, err := SearchNotesFullText("anything", -1)
		if err != nil {
			t.Fatalf("SearchNotesFullText failed: %v", err)
		}
		if results != nil {
			t.Errorf("expected nil results for empty index, got %v", results)
		}
	})
}

func TestSearchNotesCombined(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	// Create notes
	note1 := filepath.Join(notesDir, "meeting.md")
	note2 := filepath.Join(notesDir, "agenda.md")
	os.WriteFile(note1, []byte("Weekly standup notes"), 0644)
	os.WriteFile(note2, []byte("Meeting agenda for next week"), 0644)

	// Build both indexes
	index := map[string]data.NoteMeta{
		"meeting": {FilePath: note1, Title: "meeting", Created: "250101.120000"},
		"agenda":  {FilePath: note2, Title: "agenda", Created: "250102.120000"},
	}
	data.SaveIndex(index)
	data.IndexAllFTS(notesDir, index)

	t.Run("title match appears in combined results", func(t *testing.T) {
		results, err := SearchNotesCombined("meeting", -1)
		if err != nil {
			t.Fatalf("SearchNotesCombined failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected results for 'meeting'")
		}
		// "meeting" should be first since it has both title and content match
		if results[0].Title != "meeting" {
			t.Errorf("expected 'meeting' as top result, got %q", results[0].Title)
		}
	})

	t.Run("content-only match included", func(t *testing.T) {
		results, err := SearchNotesCombined("agenda", -1)
		if err != nil {
			t.Fatalf("SearchNotesCombined failed: %v", err)
		}
		// Should find "agenda" (title match) and possibly "meeting" if its content doesn't match
		found := false
		for _, r := range results {
			if r.Title == "agenda" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected 'agenda' in results")
		}
	})

	t.Run("deduplication by filepath", func(t *testing.T) {
		results, err := SearchNotesCombined("meeting", -1)
		if err != nil {
			t.Fatalf("SearchNotesCombined failed: %v", err)
		}
		// Check no duplicate filepaths
		seen := make(map[string]bool)
		for _, r := range results {
			if seen[r.FilePath] {
				t.Errorf("duplicate filepath: %s", r.FilePath)
			}
			seen[r.FilePath] = true
		}
	})
}

func TestFTSIndexSync(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	// Create and index a note
	note1 := filepath.Join(notesDir, "sync-test.md")
	os.WriteFile(note1, []byte("Content for sync testing"), 0644)

	err := data.IndexDocFTS("sync-test", note1, "Content for sync testing")
	if err != nil {
		t.Fatalf("IndexDocFTS failed: %v", err)
	}

	// Verify it's indexed
	idx, _ := data.LoadFTS()
	if _, ok := idx["sync-test"]; !ok {
		t.Fatal("expected sync-test in FTS index")
	}

	// Remove it
	err = data.RemoveDocFTS("sync-test")
	if err != nil {
		t.Fatalf("RemoveDocFTS failed: %v", err)
	}

	// Verify it's gone
	idx, _ = data.LoadFTS()
	if _, ok := idx["sync-test"]; ok {
		t.Error("sync-test should be removed from FTS index")
	}
}
