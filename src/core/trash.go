package core

import (
	"fmt"

	"gote/src/data"
)

func DeleteNote(noteName string) error {
	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	actualName, noteMeta, exists := data.LookupNote(index, noteName)
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}
	return data.TrashNote(actualName, noteMeta)
}

func RecoverNote(noteName string) error {
	cfg, err := data.LoadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	return data.RecoverNote(noteName, cfg.NoteDir)
}
