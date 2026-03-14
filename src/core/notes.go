package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gote/src/data"
)

const timeFmt = "060102.150405"

func CreateOrOpenNote(noteName string) error {
	if err := data.ValidateNoteName(noteName); err != nil {
		return err
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	return data.WithIndexLock(func(index map[string]data.NoteMeta) error {
		actualName, noteMeta, exists := data.LookupNote(index, noteName)
		if !exists {
			actualName = noteName
		}
		notePath := noteMeta.FilePath

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
		meta.LastVisited = time.Now().Format(timeFmt)
		index[actualName] = meta
		return nil
	})
}

// UpdateLastVisited updates the LastVisited timestamp for a note
func UpdateLastVisited(title string) error {
	return data.WithIndexLock(func(index map[string]data.NoteMeta) error {
		actualKey, meta, exists := data.LookupNote(index, title)
		if !exists {
			return nil
		}
		meta.LastVisited = time.Now().Format(timeFmt)
		index[actualKey] = meta
		return nil
	})
}

// OpenAndReindexNote opens a note in the editor and reindexes it afterward
// This should be used when opening existing notes to ensure tags/metadata stay in sync
func OpenAndReindexNote(filePath, title string) error {
	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if err := data.OpenFileInEditor(filePath, cfg.Editor); err != nil {
		return fmt.Errorf("error opening note: %w", err)
	}

	// Reindex the note to pick up any changes (tags, content, etc.)
	if err := data.IndexNote(filePath); err != nil {
		return fmt.Errorf("error reindexing note: %w", err)
	}

	// Update last visited timestamp
	return UpdateLastVisited(title)
}

func GetNoteInfo(noteName string) (data.NoteMeta, error) {
	index, err := data.LoadIndex()
	if err != nil {
		return data.NoteMeta{}, fmt.Errorf("loading index: %w", err)
	}
	_, meta, exists := data.LookupNote(index, noteName)
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

	return data.WithIndexLock(func(index map[string]data.NoteMeta) error {
		actualOldName, meta, exists := data.LookupNote(index, oldName)
		if !exists {
			return fmt.Errorf("note not found: %s", oldName)
		}

		oldPath := meta.FilePath
		newPath := filepath.Join(cfg.NoteDir, newName+".md")
		// Allow case-only renames (e.g., "rde" -> "RDE") on case-insensitive filesystems
		if !strings.EqualFold(oldName, newName) {
			if _, err := os.Stat(newPath); err == nil {
				return fmt.Errorf("a note with the new name already exists: %s", newName)
			}
		}

		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("error renaming note: %w", err)
		}

		delete(index, actualOldName)
		meta.Title = newName
		meta.FilePath = newPath
		meta.LastVisited = time.Now().Format(timeFmt)
		index[newName] = meta

		// Update pins inside the index lock to prevent inconsistency
		return data.WithPinsLock(func(pins map[string]data.EmptyStruct) error {
			if _, pinned := pins[actualOldName]; pinned {
				delete(pins, actualOldName)
				pins[newName] = data.EmptyStruct{}
			}
			return nil
		})
	})
}

// DuplicateNote copies a note's content to a new note with the given name
func DuplicateNote(oldName, newName string) error {
	if err := data.ValidateNoteName(newName); err != nil {
		return err
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	return data.WithIndexLock(func(index map[string]data.NoteMeta) error {
		_, meta, exists := data.LookupNote(index, oldName)
		if !exists {
			return fmt.Errorf("note not found: %s", oldName)
		}

		newPath := filepath.Join(cfg.NoteDir, newName+".md")
		if _, err := os.Stat(newPath); err == nil {
			return fmt.Errorf("a note with that name already exists: %s", newName)
		}

		content, err := os.ReadFile(meta.FilePath)
		if err != nil {
			return fmt.Errorf("error reading note: %w", err)
		}

		if err := os.WriteFile(newPath, content, 0644); err != nil {
			return fmt.Errorf("error creating duplicate: %w", err)
		}

		info, err := os.Stat(newPath)
		if err != nil {
			return fmt.Errorf("error stating duplicate: %w", err)
		}
		newMeta, err := data.BuildNoteMeta(newPath, info)
		if err != nil {
			return fmt.Errorf("error building metadata: %w", err)
		}
		index[newName] = newMeta
		return nil
	})
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

	// Check quick.md exists
	if _, err := os.Stat(quickPath); os.IsNotExist(err) {
		return fmt.Errorf("could not read quick.md: %w", err)
	}

	// Atomic rename: move quick.md to new note
	if err := os.Rename(quickPath, newPath); err != nil {
		return fmt.Errorf("error moving quick note: %w", err)
	}

	// Recreate empty quick.md
	if err := os.WriteFile(quickPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("error recreating quick.md: %w", err)
	}

	// Index the new note
	return data.IndexNote(newPath)
}