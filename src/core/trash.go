package core

import (
	"fmt"

	"gote/src/data"
)

func DeleteNote(noteName string) error {
	index := data.LoadIndex()
	noteMeta, exists := index[noteName]
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}

	return data.TrashNote(noteName, noteMeta)
}

func RecoverNote(noteName string) error {
	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	return data.RecoverNote(noteName, cfg.NoteDir)
}

func ListTrashedNotes() ([]string, error) {
	return data.ListTrashedNotes()
}