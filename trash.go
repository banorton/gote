package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func trashPath() string {
	return filepath.Join(goteDir(), "trash")
}

func trashNote(noteName string) error {
	index := loadIndex()
	noteMeta, exists := index[noteName]
	if !exists {
		return fmt.Errorf("Note not found: %s", noteName)
	}

	notePath := noteMeta.FilePath
	trashDir := trashPath()
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("Error creating trash directory: %w", err)
	}
	trashFile := filepath.Join(trashDir, filepath.Base(notePath))
	if err := os.Rename(notePath, trashFile); err != nil {
		return fmt.Errorf("Error moving note to trash: %w", err)
	}

	pins, err := loadPins()
	if err == nil {
		if _, pinned := pins[noteName]; pinned {
			delete(pins, noteName)
			savePins(pins)
		}
	}

	delete(index, noteName)
	f, err := os.Create(indexPath())
	if err != nil {
		return fmt.Errorf("Error updating index: %w", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(index); err != nil {
		return fmt.Errorf("Error saving index: %w", err)
	}

	return nil
}

func listTrashedNotes() ([]string, error) {
	files, err := os.ReadDir(trashPath())
	if err != nil {
		return nil, err
	}
	noteNames := []string{}
	for _, f := range files {
		if !f.IsDir() {
			noteNames = append(noteNames, strings.TrimSuffix(f.Name(), ".md"))
		}
	}
	return noteNames, nil
}

func recoverNote(noteName string) error {
	trashDir := trashPath()
	notesDir := noteDir()
	if notesDir == "" {
		return fmt.Errorf("Could not determine notes directory")
	}

	// Find the trashed file (assume .md extension)
	trashedFile := filepath.Join(trashDir, noteName+".md")
	if _, err := os.Stat(trashedFile); os.IsNotExist(err) {
		return fmt.Errorf("Note not found in trash: %s", noteName)
	}

	recoveredFile := filepath.Join(notesDir, noteName+".md")
	if err := os.Rename(trashedFile, recoveredFile); err != nil {
		return fmt.Errorf("Error restoring note: %w", err)
	}

	// Update index: re-index this note
	index := loadIndex()
	info, err := os.Stat(recoveredFile)
	if err != nil {
		return fmt.Errorf("Error stating restored note: %w", err)
	}
	meta, err := buildNoteMeta(recoveredFile, info)
	if err != nil {
		return fmt.Errorf("Error indexing restored note: %w", err)
	}
	index[noteName] = meta
	f, err := os.Create(indexPath())
	if err != nil {
		return fmt.Errorf("Error updating index: %w", err)
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(index); err != nil {
		return fmt.Errorf("Error saving index: %w", err)
	}
	return nil
}

func searchTrash(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote search trash <query>")
		return
	}
	query := strings.ToLower(strings.Join(args, " "))
	files, err := os.ReadDir(trashPath())
	if err != nil {
		fmt.Println("Could not read trash:", err)
		return
	}
	var results []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		title := strings.TrimSuffix(f.Name(), ".md")
		if strings.Contains(strings.ToLower(title), query) {
			results = append(results, title)
		}
	}
	if len(results) == 0 {
		fmt.Println("No matching trashed notes found.")
		return
	}
	for _, r := range results {
		fmt.Println(r)
	}
}
