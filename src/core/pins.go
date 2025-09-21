package core

import (
	"fmt"

	"gote/src/data"
)

func PinNote(noteName string) error {
	index := data.LoadIndex()
	_, exists := index[noteName]
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}

	pins, err := data.LoadPins()
	if err != nil {
		return fmt.Errorf("error loading pins: %w", err)
	}

	if _, already := pins[noteName]; already {
		return fmt.Errorf("note already pinned: %s", noteName)
	}

	pins[noteName] = data.EmptyStruct{}
	return data.SavePins(pins)
}

func UnpinNote(noteName string) error {
	index := data.LoadIndex()
	_, exists := index[noteName]
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}

	pins, err := data.LoadPins()
	if err != nil {
		return fmt.Errorf("error loading pins: %w", err)
	}

	if _, pinned := pins[noteName]; !pinned {
		return fmt.Errorf("note was not pinned: %s", noteName)
	}

	delete(pins, noteName)
	return data.SavePins(pins)
}

func ListPinnedNotes() ([]string, error) {
	pins, err := data.LoadPins()
	if err != nil {
		return nil, fmt.Errorf("error loading pins: %w", err)
	}

	var pinnedNotes []string
	for title := range pins {
		pinnedNotes = append(pinnedNotes, title)
	}

	return pinnedNotes, nil
}