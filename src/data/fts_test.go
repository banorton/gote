package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveFTS(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	t.Run("LoadFTS returns empty map for missing file", func(t *testing.T) {
		idx, err := LoadFTS()
		if err != nil {
			t.Fatalf("LoadFTS failed: %v", err)
		}
		if len(idx) != 0 {
			t.Errorf("expected empty map, got %d entries", len(idx))
		}
	})

	t.Run("SaveFTS and LoadFTS roundtrip", func(t *testing.T) {
		idx := FTSIndex{
			"test-note": {
				Title:    "test-note",
				FilePath: "/path/to/test-note.md",
				Terms:    map[string]int{"hello": 3, "world": 1},
				Length:   4,
			},
		}
		if err := SaveFTS(idx); err != nil {
			t.Fatalf("SaveFTS failed: %v", err)
		}

		loaded, err := LoadFTS()
		if err != nil {
			t.Fatalf("LoadFTS failed: %v", err)
		}
		if len(loaded) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(loaded))
		}
		doc := loaded["test-note"]
		if doc.Title != "test-note" {
			t.Errorf("Title = %q, want test-note", doc.Title)
		}
		if doc.Terms["hello"] != 3 {
			t.Errorf("hello count = %d, want 3", doc.Terms["hello"])
		}
	})
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func([]string) bool
		desc  string
	}{
		{
			"basic words",
			"Hello World",
			func(tokens []string) bool { return len(tokens) == 2 },
			"should produce 2 tokens",
		},
		{
			"stop words filtered",
			"the quick brown fox is a very fast animal",
			func(tokens []string) bool {
				for _, t := range tokens {
					if t == "the" || t == "is" || t == "a" {
						return false
					}
				}
				return true
			},
			"should not contain stop words",
		},
		{
			"stemming works",
			"running runs",
			func(tokens []string) bool {
				// Both should stem to "run"
				for _, t := range tokens {
					if t != "run" {
						return false
					}
				}
				return len(tokens) == 2
			},
			"should stem to 'run'",
		},
		{
			"punctuation split",
			"hello-world foo.bar baz_qux",
			func(tokens []string) bool { return len(tokens) >= 4 },
			"should split on punctuation",
		},
		{
			"short words filtered",
			"I a x go the",
			func(tokens []string) bool {
				for _, t := range tokens {
					if t == "i" || t == "a" || t == "x" {
						return false
					}
				}
				return true
			},
			"should filter single-char words",
		},
		{
			"empty string",
			"",
			func(tokens []string) bool { return len(tokens) == 0 },
			"should return empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := Tokenize(tt.input)
			if !tt.check(tokens) {
				t.Errorf("Tokenize(%q): %s, got %v", tt.input, tt.desc, tokens)
			}
		})
	}
}

func TestBuildDocTerms(t *testing.T) {
	doc := BuildDocTerms("Meeting Notes", "/notes/meeting.md", "The meeting was about running the project")

	// "meeting" appears in title (stemmed, 3x weight) and content
	meetStem := Tokenize("meeting")[0]
	if doc.Terms[meetStem] < 4 {
		t.Errorf("expected 'meeting' stem count >= 4 (3 title + 1 content), got %d", doc.Terms[meetStem])
	}

	if doc.Length == 0 {
		t.Error("doc length should be > 0")
	}
}

func TestIndexDocFTS(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	err := IndexDocFTS("test-note", "/path/test-note.md", "hello world testing content")
	if err != nil {
		t.Fatalf("IndexDocFTS failed: %v", err)
	}

	idx, err := LoadFTS()
	if err != nil {
		t.Fatalf("LoadFTS failed: %v", err)
	}
	if _, ok := idx["test-note"]; !ok {
		t.Error("expected test-note in FTS index")
	}
}

func TestRemoveDocFTS(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	// Index a doc then remove it
	IndexDocFTS("test-note", "/path/test-note.md", "content")
	err := RemoveDocFTS("test-note")
	if err != nil {
		t.Fatalf("RemoveDocFTS failed: %v", err)
	}

	idx, err := LoadFTS()
	if err != nil {
		t.Fatalf("LoadFTS failed: %v", err)
	}
	if _, ok := idx["test-note"]; ok {
		t.Error("test-note should be removed from FTS index")
	}
}

func TestIndexAllFTS(t *testing.T) {
	dir, cleanup := testDir(t)
	defer cleanup()

	origGoteDir := GoteDir
	GoteDir = func() string { return dir }
	defer func() { GoteDir = origGoteDir }()

	notesDir := filepath.Join(dir, "notes")
	os.MkdirAll(notesDir, 0755)

	// Create test notes
	note1 := filepath.Join(notesDir, "note1.md")
	note2 := filepath.Join(notesDir, "note2.md")
	os.WriteFile(note1, []byte("First note about meetings"), 0644)
	os.WriteFile(note2, []byte("Second note about running"), 0644)

	index := map[string]NoteMeta{
		"note1": {FilePath: note1, Title: "note1"},
		"note2": {FilePath: note2, Title: "note2"},
	}

	err := IndexAllFTS(notesDir, index)
	if err != nil {
		t.Fatalf("IndexAllFTS failed: %v", err)
	}

	idx, err := LoadFTS()
	if err != nil {
		t.Fatalf("LoadFTS failed: %v", err)
	}
	if len(idx) != 2 {
		t.Errorf("expected 2 entries, got %d", len(idx))
	}
}
