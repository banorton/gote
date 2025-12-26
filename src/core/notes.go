package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gote/src/data"
)

func CreateOrOpenNote(noteName string) error {
	if err := data.ValidateNoteName(noteName); err != nil {
		return err
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	noteMeta, exists := index[noteName]
	notePath := ""
	if exists {
		notePath = noteMeta.FilePath
	}

	noteDir := cfg.NoteDir
	if err := os.MkdirAll(noteDir, 0755); err != nil {
		return fmt.Errorf("error creating notes directory: %w", err)
	}

	if notePath == "" {
		notePath = filepath.Join(noteDir, noteName+".md")
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			f, err := os.Create(notePath)
			if err != nil {
				return fmt.Errorf("error creating note: %w", err)
			}
			f.Close()
		}
	}

	if err := data.OpenFileInEditor(notePath, cfg.Editor); err != nil {
		return fmt.Errorf("error opening note in editor: %w", err)
	}

	info, err := os.Stat(notePath)
	if err != nil {
		return fmt.Errorf("error stating note after edit: %w", err)
	}
	meta, err := data.BuildNoteMeta(notePath, info)
	if err != nil {
		return fmt.Errorf("error building note metadata: %w", err)
	}
	index[noteName] = meta
	if err := data.SaveIndex(index); err != nil {
		return err
	}
	return data.UpdateTagsIndex(index)
}

func GetNoteInfo(noteName string) (data.NoteMeta, error) {
	index, err := data.LoadIndex()
	if err != nil {
		return data.NoteMeta{}, fmt.Errorf("loading index: %w", err)
	}
	meta, exists := index[noteName]
	if !exists {
		return data.NoteMeta{}, fmt.Errorf("note not found: %s", noteName)
	}
	return meta, nil
}

func RenameNote(oldName, newName string) error {
	if err := data.ValidateNoteName(newName); err != nil {
		return err
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	meta, exists := index[oldName]
	if !exists {
		return fmt.Errorf("note not found: %s", oldName)
	}

	oldPath := meta.FilePath
	newPath := filepath.Join(cfg.NoteDir, newName+".md")
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("a note with the new name already exists: %s", newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("error renaming note: %w", err)
	}

	delete(index, oldName)
	meta.Title = newName
	meta.FilePath = newPath
	index[newName] = meta

	if err := data.SaveIndex(index); err != nil {
		return fmt.Errorf("error updating index: %w", err)
	}
	if err := data.UpdateTagsIndex(index); err != nil {
		return fmt.Errorf("error updating tags index: %w", err)
	}

	pins, err := data.LoadPins()
	if err == nil {
		if _, pinned := pins[oldName]; pinned {
			delete(pins, oldName)
			pins[newName] = data.EmptyStruct{}
			if err := data.SavePins(pins); err != nil {
				return fmt.Errorf("error saving pins: %w", err)
			}
		}
	}

	return nil
}

func AddTagsToNote(noteName string, tagsToAdd []string) error {
	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	noteMeta, exists := index[noteName]
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}

	notePath := noteMeta.FilePath
	if notePath == "" {
		return fmt.Errorf("note path missing for: %s", noteName)
	}

	fileData, err := os.ReadFile(notePath)
	if err != nil {
		return fmt.Errorf("error reading note: %w", err)
	}

	lines := strings.SplitN(string(fileData), "\n", 2)
	firstLine := ""
	rest := ""
	if len(lines) > 0 {
		firstLine = lines[0]
	}
	if len(lines) > 1 {
		rest = lines[1]
	}

	existingTags := data.ParseTags(firstLine)
	tagSet := make(map[string]struct{})
	for _, t := range existingTags {
		tagSet[t] = struct{}{}
	}

	added := false
	for _, t := range tagsToAdd {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, exists := tagSet[t]; !exists {
			existingTags = append(existingTags, t)
			tagSet[t] = struct{}{}
			added = true
		}
	}

	if !added {
		return fmt.Errorf("no new tags to add")
	}

	newFirstLine := "." + strings.Join(existingTags, ".")
	newContent := newFirstLine
	if rest != "" {
		newContent += "\n" + rest
	}

	if err := os.WriteFile(notePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("error writing note: %w", err)
	}

	return data.IndexNote(notePath)
}

// PromoteQuickNote moves content from quick.md to a new named note
func PromoteQuickNote(newName string) error {
	if err := data.ValidateNoteName(newName); err != nil {
		return err
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	quickPath := filepath.Join(cfg.NoteDir, "quick.md")
	newPath := filepath.Join(cfg.NoteDir, newName+".md")

	// Check if target note already exists
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("note already exists: %s", newName)
	}

	// Read quick.md content
	content, err := os.ReadFile(quickPath)
	if err != nil {
		return fmt.Errorf("could not read quick.md: %w", err)
	}

	// Write content to new note
	if err := os.WriteFile(newPath, content, 0644); err != nil {
		return fmt.Errorf("error creating note: %w", err)
	}

	// Clear quick.md
	if err := os.WriteFile(quickPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("error clearing quick.md: %w", err)
	}

	// Index the new note
	return data.IndexNote(newPath)
}