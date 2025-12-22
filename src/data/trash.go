package data

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func TrashPath() string {
	return filepath.Join(GoteDir(), "trash")
}

func TrashNote(noteName string, noteMeta NoteMeta) error {
	notePath := noteMeta.FilePath
	trashDir := TrashPath()
	if err := os.MkdirAll(trashDir, 0755); err != nil {
		return fmt.Errorf("error creating trash directory: %w", err)
	}
	trashFile := filepath.Join(trashDir, filepath.Base(notePath))
	if err := os.Rename(notePath, trashFile); err != nil {
		return fmt.Errorf("error moving note to trash: %w", err)
	}

	pins, err := LoadPins()
	if err == nil {
		if _, pinned := pins[noteName]; pinned {
			delete(pins, noteName)
			if err := SavePins(pins); err != nil {
				return fmt.Errorf("error saving pins: %w", err)
			}
		}
	}

	index := LoadIndex()
	delete(index, noteName)
	if err := SaveIndex(index); err != nil {
		return err
	}
	return UpdateTagsIndex(index)
}

func ListTrashedNotes() ([]string, error) {
	files, err := os.ReadDir(TrashPath())
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

func RecoverNote(noteName, notesDir string) error {
	trashDir := TrashPath()
	if notesDir == "" {
		return fmt.Errorf("could not determine notes directory")
	}

	trashedFile := filepath.Join(trashDir, noteName+".md")
	if _, err := os.Stat(trashedFile); os.IsNotExist(err) {
		return fmt.Errorf("note not found in trash: %s", noteName)
	}

	recoveredFile := filepath.Join(notesDir, noteName+".md")
	if err := os.Rename(trashedFile, recoveredFile); err != nil {
		return fmt.Errorf("error restoring note: %w", err)
	}

	index := LoadIndex()
	info, err := os.Stat(recoveredFile)
	if err != nil {
		return fmt.Errorf("error stating restored note: %w", err)
	}
	meta, err := BuildNoteMeta(recoveredFile, info)
	if err != nil {
		return fmt.Errorf("error indexing restored note: %w", err)
	}
	index[noteName] = meta
	if err := SaveIndex(index); err != nil {
		return err
	}
	return UpdateTagsIndex(index)
}

func SearchTrash(query string) ([]string, error) {
	files, err := os.ReadDir(TrashPath())
	if err != nil {
		return nil, err
	}
	var results []string
	queryLower := strings.ToLower(query)
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		title := strings.TrimSuffix(f.Name(), ".md")
		if strings.Contains(strings.ToLower(title), queryLower) {
			results = append(results, title)
		}
	}
	return results, nil
}

func EmptyTrash() (int, error) {
	trashDir := TrashPath()
	files, err := os.ReadDir(trashDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		filePath := filepath.Join(trashDir, f.Name())
		if err := os.Remove(filePath); err != nil {
			return count, fmt.Errorf("error removing %s: %w", f.Name(), err)
		}
		count++
	}
	return count, nil
}