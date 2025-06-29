package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type emptyStruct struct{}

func pinsPath() string {
	return filepath.Join(goteDir(), "pins.json")
}

func loadPins() (map[string]emptyStruct, error) {
	pins := make(map[string]emptyStruct)
	f, err := os.Open(pinsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return pins, nil // no pins yet
		}
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&pins); err != nil {
		return nil, err
	}
	return pins, nil
}

func savePins(pins map[string]emptyStruct) error {
	f, err := os.Create(pinsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(pins)
}

func formatPinsFile() error {
	pinsPath := pinsPath()
	data, err := os.ReadFile(pinsPath)
	if err != nil {
		return fmt.Errorf("could not read pins file: %w", err)
	}
	var pins map[string]emptyStruct
	if err := json.Unmarshal(data, &pins); err != nil {
		return fmt.Errorf("could not parse pins file: %w", err)
	}
	pretty, err := json.MarshalIndent(pins, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal pretty pins: %w", err)
	}
	if err := os.WriteFile(pinsPath, pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty pins: %w", err)
	}
	return nil
}
