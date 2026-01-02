package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gote/src/data"
)

// ListTemplates returns the names of all available templates
func ListTemplates() ([]string, error) {
	return data.ListTemplateFiles()
}

// CreateOrEditTemplate opens a template in the editor (creates if doesn't exist)
func CreateOrEditTemplate(name string) error {
	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if err := data.EnsureTemplatesDir(); err != nil {
		return fmt.Errorf("error creating templates directory: %w", err)
	}

	templatePath := filepath.Join(data.TemplatesDir(), name+".md")

	// Create file if it doesn't exist
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		f, err := os.Create(templatePath)
		if err != nil {
			return fmt.Errorf("error creating template: %w", err)
		}
		f.Close()
	}

	return data.OpenFileInEditor(templatePath, cfg.Editor)
}

// CreateNoteFromTemplate creates a new note with content from a template
func CreateNoteFromTemplate(noteName, templateName string) error {
	if err := data.ValidateNoteName(noteName); err != nil {
		return err
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	// Check if note already exists
	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	if _, exists := index[noteName]; exists {
		return fmt.Errorf("note already exists: %s", noteName)
	}

	// Load template content
	content, err := data.LoadTemplate(templateName)
	if err != nil {
		return err
	}

	// Create note with template content
	noteDir := cfg.NoteDir
	if err := os.MkdirAll(noteDir, 0755); err != nil {
		return fmt.Errorf("error creating notes directory: %w", err)
	}

	notePath := filepath.Join(noteDir, noteName+".md")
	if err := os.WriteFile(notePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("error creating note: %w", err)
	}

	// Open in editor
	if err := data.OpenFileInEditor(notePath, cfg.Editor); err != nil {
		return fmt.Errorf("error opening note in editor: %w", err)
	}

	// Index the note after editing
	info, err := os.Stat(notePath)
	if err != nil {
		return fmt.Errorf("error stating note after edit: %w", err)
	}
	meta, err := data.BuildNoteMeta(notePath, info)
	if err != nil {
		return fmt.Errorf("error building note metadata: %w", err)
	}
	meta.LastVisited = time.Now().Format("060102.150405")
	index[noteName] = meta
	return data.SaveIndexWithTags(index)
}
