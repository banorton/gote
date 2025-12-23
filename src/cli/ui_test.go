package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureUIOutput captures stdout during a function call
func captureUIOutput(f func()) string {
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

// --- visibleLen tests ---

func TestVisibleLen(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"plain text", "hello", 5},
		{"empty string", "", 0},
		{"single ANSI code", "\033[36mhello\033[0m", 5},
		{"multiple ANSI codes", "\033[1m\033[36mbold cyan\033[0m", 9},
		{"mixed content", "prefix \033[36mcolored\033[0m suffix", 21},
		{"nested codes", "\033[1;36mbold cyan\033[0m", 9},
		{"dim text", "\033[2mdim\033[0m", 3},
		{"reverse video", "\033[7mreverse\033[0m", 7},
		{"no closing code", "\033[36munclosed", 8},
		// Note: visibleLen counts bytes, not runes. Multi-byte chars count as multiple.
		// This is fine for box alignment since we use ASCII for items.
		{"unicode characters", "héllo wörld", 13}, // é=2 bytes, ö=2 bytes
		{"ANSI with unicode", "\033[36mhéllo\033[0m", 6}, // é=2 bytes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleLen(tt.input)
			if got != tt.expected {
				t.Errorf("visibleLen(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

// --- UI constructor tests ---

func TestNewUI(t *testing.T) {
	t.Run("creates fancy UI", func(t *testing.T) {
		ui := NewUI(true)
		if !ui.Fancy {
			t.Error("Expected Fancy to be true")
		}
	})

	t.Run("creates non-fancy UI", func(t *testing.T) {
		ui := NewUI(false)
		if ui.Fancy {
			t.Error("Expected Fancy to be false")
		}
	})
}

// --- Non-fancy output tests ---

func TestUISuccess(t *testing.T) {
	t.Run("non-fancy mode", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.Success("operation completed")
		})
		if !strings.Contains(output, "operation completed") {
			t.Errorf("Expected success message, got: %s", output)
		}
	})

	t.Run("fancy mode has arrow prefix", func(t *testing.T) {
		ui := NewUI(true)
		output := captureUIOutput(func() {
			ui.Success("operation completed")
		})
		if !strings.Contains(output, "->") {
			t.Errorf("Expected arrow prefix in fancy mode, got: %s", output)
		}
	})
}

func TestUIError(t *testing.T) {
	t.Run("non-fancy mode", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.Error("something failed")
		})
		if !strings.Contains(output, "Error:") {
			t.Errorf("Expected 'Error:' prefix, got: %s", output)
		}
		if !strings.Contains(output, "something failed") {
			t.Errorf("Expected error message, got: %s", output)
		}
	})
}

func TestUIInfo(t *testing.T) {
	ui := NewUI(false)
	output := captureUIOutput(func() {
		ui.Info("info message")
	})
	if !strings.Contains(output, "info message") {
		t.Errorf("Expected info message, got: %s", output)
	}
}

func TestUIEmpty(t *testing.T) {
	ui := NewUI(false)
	output := captureUIOutput(func() {
		ui.Empty("nothing found")
	})
	if !strings.Contains(output, "nothing found") {
		t.Errorf("Expected empty message, got: %s", output)
	}
}

func TestUITitle(t *testing.T) {
	ui := NewUI(false)
	output := captureUIOutput(func() {
		ui.Title("My Title")
	})
	if !strings.Contains(output, "My Title") {
		t.Errorf("Expected title, got: %s", output)
	}
}

func TestUIKeyValue(t *testing.T) {
	ui := NewUI(false)
	output := captureUIOutput(func() {
		ui.KeyValue("Name", "test-note")
	})
	if !strings.Contains(output, "Name:") || !strings.Contains(output, "test-note") {
		t.Errorf("Expected key-value output, got: %s", output)
	}
}

func TestUITags(t *testing.T) {
	t.Run("with tags", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.Tags([]string{"work", "urgent"})
		})
		if !strings.Contains(output, "Tags:") {
			t.Errorf("Expected 'Tags:' prefix, got: %s", output)
		}
		if !strings.Contains(output, "work") || !strings.Contains(output, "urgent") {
			t.Errorf("Expected tag names, got: %s", output)
		}
	})

	t.Run("empty tags", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.Tags([]string{})
		})
		if output != "" {
			t.Errorf("Expected no output for empty tags, got: %s", output)
		}
	})
}

func TestUIListItem(t *testing.T) {
	t.Run("with key", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.ListItem('a', "item text", false)
		})
		if !strings.Contains(output, "[a]") || !strings.Contains(output, "item text") {
			t.Errorf("Expected keyed list item, got: %s", output)
		}
	})

	t.Run("without key", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.ListItem(0, "plain item", false)
		})
		if !strings.Contains(output, "plain item") {
			t.Errorf("Expected plain list item, got: %s", output)
		}
	})
}

func TestUIListItemWithMeta(t *testing.T) {
	ui := NewUI(false)
	output := captureUIOutput(func() {
		ui.ListItemWithMeta('a', "note-name", "(3 tags)")
	})
	if !strings.Contains(output, "[a]") {
		t.Errorf("Expected key in output, got: %s", output)
	}
	if !strings.Contains(output, "note-name") {
		t.Errorf("Expected item text, got: %s", output)
	}
	if !strings.Contains(output, "(3 tags)") {
		t.Errorf("Expected metadata, got: %s", output)
	}
}

// --- Box rendering tests (non-fancy) ---

func TestUIBox(t *testing.T) {
	t.Run("non-fancy mode shows title and content", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.Box("Results", []string{"item1", "item2"}, 0)
		})
		if !strings.Contains(output, "Results:") {
			t.Errorf("Expected title with colon, got: %s", output)
		}
		if !strings.Contains(output, "item1") || !strings.Contains(output, "item2") {
			t.Errorf("Expected items, got: %s", output)
		}
	})

	t.Run("fancy mode draws box characters", func(t *testing.T) {
		ui := NewUI(true)
		output := captureUIOutput(func() {
			ui.Box("Results", []string{"item1", "item2"}, 0)
		})
		if !strings.Contains(output, BoxTopLeft) {
			t.Errorf("Expected box top-left corner, got: %s", output)
		}
		if !strings.Contains(output, BoxBottomRight) {
			t.Errorf("Expected box bottom-right corner, got: %s", output)
		}
	})
}

func TestUISelectableList(t *testing.T) {
	t.Run("non-fancy mode", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.SelectableList("Select", []string{"opt1", "opt2"}, -1, []rune{'a', 's'})
		})
		if !strings.Contains(output, "Select:") {
			t.Errorf("Expected title, got: %s", output)
		}
		if !strings.Contains(output, "[a]") || !strings.Contains(output, "[s]") {
			t.Errorf("Expected keys, got: %s", output)
		}
	})
}

func TestUINavHint(t *testing.T) {
	t.Run("non-fancy single page", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.NavHint(1, 1)
		})
		if !strings.Contains(output, "(1/1)") {
			t.Errorf("Expected page indicator, got: %s", output)
		}
		if !strings.Contains(output, "[q]uit") {
			t.Errorf("Expected quit hint, got: %s", output)
		}
		// Single page shouldn't show next/prev
		if strings.Contains(output, "[n]ext") {
			t.Errorf("Single page shouldn't show next, got: %s", output)
		}
	})

	t.Run("non-fancy multi page", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.NavHint(1, 3)
		})
		if !strings.Contains(output, "(1/3)") {
			t.Errorf("Expected page indicator, got: %s", output)
		}
		if !strings.Contains(output, "[n]ext") || !strings.Contains(output, "[p]rev") {
			t.Errorf("Expected next/prev hints, got: %s", output)
		}
	})
}

func TestUIInfoBox(t *testing.T) {
	t.Run("non-fancy mode", func(t *testing.T) {
		ui := NewUI(false)
		output := captureUIOutput(func() {
			ui.InfoBox("Note Info", [][2]string{
				{"Name", "test-note"},
				{"Tags", "work, urgent"},
			})
		})
		if !strings.Contains(output, "Note Info") {
			t.Errorf("Expected title, got: %s", output)
		}
		if !strings.Contains(output, "Name:") || !strings.Contains(output, "test-note") {
			t.Errorf("Expected key-value pairs, got: %s", output)
		}
	})
}
