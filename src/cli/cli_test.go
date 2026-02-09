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
	index, _ := data.LoadIndex()
	index[name] = meta
	data.SaveIndexWithTags(index)
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
		if !strings.Contains(output, "Recent") {
			t.Error("Help should mention recent notes")
		}
		if !strings.Contains(output, "Search") {
			t.Error("Help should mention search")
		}
	})
}


// --- TagCommand tests ---

func TestTagCommand(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "note1", ".work.urgent\nContent")
	createTestNote(t, notesDir, "note2", ".work.project\nContent")
	createTestNote(t, notesDir, "note3", ".personal\nContent")

	t.Run("lists all tags", func(t *testing.T) {
		output := captureOutput(func() {
			TagCommand([]string{}, false, false, false, false, false)
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
			TagCommand([]string{"popular", "-n", "2"}, false, false, false, false, false)
		})

		// Should show header + 2 tags
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) > 3 { // "Top 2 tags by usage:" + 2 tags
			t.Errorf("Expected at most 3 lines with popular -n 2, got %d lines", len(lines))
		}
	})

	// Note: Tag filtering tests are skipped because they now use interactive menus
	// that wait for user input. The core filtering logic is tested in core package.
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

	// Note: PinCommand with no args now shows interactive menu, tested manually

	t.Run("UnpinCommand unpins a note", func(t *testing.T) {
		output := captureOutput(func() {
			UnpinCommand([]string{"test-note"})
		})

		if !strings.Contains(output, "Unpinned") {
			t.Errorf("Expected 'Unpinned' message, got: %s", output)
		}
	})

	t.Run("UnpinCommand is idempotent for not pinned", func(t *testing.T) {
		output := captureOutput(func() {
			UnpinCommand([]string{"test-note"})
		})

		// Idempotent - unpinning an unpinned note should succeed
		if strings.Contains(output, "Error") {
			t.Errorf("Should succeed when unpinning not-pinned note, got: %s", output)
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
		index, err := data.LoadIndex()
		if err != nil {
			t.Fatalf("LoadIndex failed: %v", err)
		}
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
		index, err := data.LoadIndex()
		if err != nil {
			t.Fatalf("LoadIndex failed: %v", err)
		}
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

// --- Search command tests ---
// Note: Search commands now use interactive menus. Testing no-results case only.

func TestSearchCommand(t *testing.T) {
	_, notesDir, cleanup := testEnv(t)
	defer cleanup()

	createTestNote(t, notesDir, "project-alpha", ".work\nAlpha content")

	t.Run("search with no results", func(t *testing.T) {
		output := captureOutput(func() {
			SearchCommand([]string{"nonexistent"}, false, false, false, false, false)
		})

		if !strings.Contains(output, "No matching") {
			t.Errorf("Expected no results message, got: %s", output)
		}
	})
}

// Note: RecentCommand now uses interactive menus. Core logic tested in core package.

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
