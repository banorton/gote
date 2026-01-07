package core

import (
	"fmt"
	"strings"

	"gote/src/data"
)

func PinNote(noteName string) error {
	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	_, exists := index[noteName]
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}

	pins, err := data.LoadPins()
	if err != nil {
		return fmt.Errorf("error loading pins: %w", err)
	}

	if _, already := pins[noteName]; already {
		return nil // idempotent - already pinned is success
	}

	pins[noteName] = data.EmptyStruct{}
	return data.SavePins(pins)
}

func UnpinNote(noteName string) error {
	index, err := data.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	_, exists := index[noteName]
	if !exists {
		return fmt.Errorf("note not found: %s", noteName)
	}

	pins, err := data.LoadPins()
	if err != nil {
		return fmt.Errorf("error loading pins: %w", err)
	}

	if _, pinned := pins[noteName]; !pinned {
		return nil // idempotent - already unpinned is success
	}

	delete(pins, noteName)
	return data.SavePins(pins)
}

func ListPinnedNotes() ([]string, error) {
	pins, err := data.LoadPins()
	if err != nil {
		return nil, fmt.Errorf("error loading pins: %w", err)
	}

	index, err := data.LoadIndex()
	if err != nil {
		return nil, fmt.Errorf("error loading index: %w", err)
	}

	// Build case-insensitive lookup from index
	lowerToActual := make(map[string]string)
	for title := range index {
		lowerToActual[strings.ToLower(title)] = title
	}

	var pinnedNotes []string
	for pin := range pins {
		// Use actual case from index, skip if note no longer exists
		if actual, exists := lowerToActual[strings.ToLower(pin)]; exists {
			pinnedNotes = append(pinnedNotes, actual)
		}
	}

	return pinnedNotes, nil
}