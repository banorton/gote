package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gote/src/data"
)

// testEnv sets up a test environment and returns a cleanup function
func testEnv(t *testing.T) (goteDir string, notesDir string, cleanup func()) {
	t.Helper()

	goteDir, err := os.MkdirTemp("", "gote-cli-test-*")
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
	index := data.LoadIndex()
	index[name] = meta
	data.SaveIndex(index)
	data.UpdateTagsIndex(index)
}

// captureOutput captures stdout during a function call
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- HelpCommand tests ---

func TestHelpCommand(t *testing.T) {
	t.Run("displays help text", func(t *testing.T) {
		output := captureOutput(func() {
			HelpCommand([]string{})
		})

		if !strings.Contains(output, "gote") {
			t.Error("Help should mention 'gote'")
		}
		if !strings.Contains(output, "new") {
			t.Error("Help should mention 'new' command")
		}
		if !strings.Contains(output, "search") {
			t.Error("Help should mention 'search' command")
		}
	})
}


// --- TagsCommand tests ---

func TestTagsCommand(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "note1", ".work.urgent\nContent")
	createTestNote(t, notesDir, "note2", ".work.project\nContent")
	createTestNote(t, notesDir, "note3", ".personal\nContent")

	t.Run("lists all tags", func(t *testing.T) {
		output := captureOutput(func() {
			TagsCommand([]string{})
		})

		if !strings.Contains(output, "work") {
			t.Error("Should list 'work' tag")
		}
		if !strings.Contains(output, "personal") {
			t.Error("Should list 'personal' tag")
		}
	})

	t.Run("popular respects limit", func(t *testing.T) {
		output := captureOutput(func() {
			TagsCommand([]string{"popular", "-n", "2"})
		})

		// Should show header + 2 tags
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) > 3 { // "Top 2 tags by usage:" + 2 tags
			t.Errorf("Expected at most 3 lines with popular -n 2, got %d lines", len(lines))
		}
	})
}

// --- Pin commands tests ---

func TestPinCommands(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "test-note", ".tag\nContent")

	t.Run("PinCommand pins a note", func(t *testing.T) {
		output := captureOutput(func() {
			PinCommand([]string{"test-note"})
		})

		if !strings.Contains(output, "Pinned") {
			t.Errorf("Expected 'Pinned' message, got: %s", output)
		}
	})

	t.Run("PinCommand is idempotent for already pinned", func(t *testing.T) {
		output := captureOutput(func() {
			PinCommand([]string{"test-note"})
		})

		if strings.Contains(output, "Error") {
			t.Errorf("Should not error for already pinned (idempotent), got: %s", output)
		}
	})

	t.Run("PinCommand with no args lists pins", func(t *testing.T) {
		output := captureOutput(func() {
			PinCommand([]string{})
		})

		if !strings.Contains(output, "test-note") {
			t.Errorf("Expected pinned note in list, got: %s", output)
		}
	})

	t.Run("UnpinCommand unpins a note", func(t *testing.T) {
		output := captureOutput(func() {
			UnpinCommand([]string{"test-note"})
		})

		if !strings.Contains(output, "Unpinned") {
			t.Errorf("Expected 'Unpinned' message, got: %s", output)
		}
	})

	t.Run("UnpinCommand shows error for not pinned", func(t *testing.T) {
		output := captureOutput(func() {
			UnpinCommand([]string{"test-note"})
		})

		if !strings.Contains(output, "Error") {
			t.Errorf("Expected error for not pinned, got: %s", output)
		}
	})

	t.Run("UnpinCommand shows usage with no args", func(t *testing.T) {
		output := captureOutput(func() {
			UnpinCommand([]string{})
		})

		if !strings.Contains(output, "Usage") {
			t.Errorf("Expected usage message, got: %s", output)
		}
	})
}

// --- Trash commands tests ---

func TestTrashCommands(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "trash-me", ".tag\nContent")

	t.Run("DeleteCommand moves note to trash", func(t *testing.T) {
		output := captureOutput(func() {
			DeleteCommand([]string{"trash-me"})
		})

		if !strings.Contains(output, "moved to trash") || !strings.Contains(output, "trash-me") {
			t.Errorf("Expected trash confirmation, got: %s", output)
		}

		// Verify note is gone from index
		index := data.LoadIndex()
		if _, exists := index["trash-me"]; exists {
			t.Error("Note should be removed from index")
		}
	})

	t.Run("RecoverCommand recovers note", func(t *testing.T) {
		output := captureOutput(func() {
			RecoverCommand([]string{"trash-me"})
		})

		if !strings.Contains(output, "recovered") {
			t.Errorf("Expected recovery confirmation, got: %s", output)
		}

		// Verify note is back in index
		index := data.LoadIndex()
		if _, exists := index["trash-me"]; !exists {
			t.Error("Note should be back in index")
		}
	})

	t.Run("DeleteCommand shows error for nonexistent", func(t *testing.T) {
		output := captureOutput(func() {
			DeleteCommand([]string{"nonexistent"})
		})

		if !strings.Contains(output, "Error") {
			t.Errorf("Expected error message, got: %s", output)
		}
	})

	t.Run("DeleteCommand shows usage with no args", func(t *testing.T) {
		output := captureOutput(func() {
			DeleteCommand([]string{})
		})

		if !strings.Contains(output, "Usage") {
			t.Errorf("Expected usage message, got: %s", output)
		}
	})
}

// --- Search command tests (non-interactive) ---

func TestSearchCommand(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "project-alpha", ".work\nAlpha content")
	createTestNote(t, notesDir, "project-beta", ".work\nBeta content")
	createTestNote(t, notesDir, "personal-journal", ".personal\nJournal content")

	t.Run("search by title shows results", func(t *testing.T) {
		output := captureOutput(func() {
			SearchCommand([]string{"project"}, false, false, false)
		})

		if !strings.Contains(output, "project-alpha") {
			t.Error("Should find project-alpha")
		}
		if !strings.Contains(output, "project-beta") {
			t.Error("Should find project-beta")
		}
	})

	t.Run("search with no results", func(t *testing.T) {
		output := captureOutput(func() {
			SearchCommand([]string{"nonexistent"}, false, false, false)
		})

		if !strings.Contains(output, "No matching") {
			t.Errorf("Expected no results message, got: %s", output)
		}
	})

	t.Run("search by tags", func(t *testing.T) {
		output := captureOutput(func() {
			SearchCommand([]string{"-t", "work"}, false, false, false)
		})

		if !strings.Contains(output, "project-alpha") {
			t.Error("Should find project-alpha with work tag")
		}
		if !strings.Contains(output, "project-beta") {
			t.Error("Should find project-beta with work tag")
		}
	})

	t.Run("search shows usage when no query", func(t *testing.T) {
		output := captureOutput(func() {
			SearchCommand([]string{}, false, false, false)
		})

		if !strings.Contains(output, "Usage") {
			t.Errorf("Expected usage message, got: %s", output)
		}
	})
}

// --- Recent command tests (non-interactive) ---

func TestRecentCommand(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "note1", ".tag\nFirst")
	createTestNote(t, notesDir, "note2", ".tag\nSecond")
	createTestNote(t, notesDir, "note3", ".tag\nThird")

	t.Run("lists recent notes", func(t *testing.T) {
		output := captureOutput(func() {
			RecentCommand([]string{}, false, false, false)
		})

		if !strings.Contains(output, "note1") {
			t.Error("Should list note1")
		}
		if !strings.Contains(output, "note2") {
			t.Error("Should list note2")
		}
		if !strings.Contains(output, "note3") {
			t.Error("Should list note3")
		}
	})
}

// --- InfoCommand tests ---

func TestInfoCommand(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "info-test", ".work.project\nThis is test content for the info command.")

	t.Run("shows note info", func(t *testing.T) {
		output := captureOutput(func() {
			InfoCommand([]string{"info-test"})
		})

		if !strings.Contains(output, "info-test") {
			t.Error("Should show note title")
		}
		if !strings.Contains(output, "work") {
			t.Error("Should show tags")
		}
	})

	t.Run("shows error for nonexistent", func(t *testing.T) {
		output := captureOutput(func() {
			InfoCommand([]string{"nonexistent"})
		})

		if !strings.Contains(output, "not found") {
			t.Errorf("Expected not found error, got: %s", output)
		}
	})

	t.Run("shows usage with no args", func(t *testing.T) {
		output := captureOutput(func() {
			InfoCommand([]string{})
		})

		if !strings.Contains(output, "Usage") {
			t.Errorf("Expected usage message, got: %s", output)
		}
	})
}
