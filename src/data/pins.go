package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type EmptyStruct struct{}

func PinsPath() string {
	return filepath.Join(GoteDir(), "pins.json")
}

func LoadPins() (map[string]EmptyStruct, error) {
	pins := make(map[string]EmptyStruct)
	f, err := os.Open(PinsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return pins, nil
		}
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&pins); err != nil {
		return nil, err
	}
	return pins, nil
}

func SavePins(pins map[string]EmptyStruct) error {
	f, err := os.Create(PinsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(pins)
}

func FormatPinsFile() error {
	pinsPath := PinsPath()
	data, err := os.ReadFile(pinsPath)
	if err != nil {
		return fmt.Errorf("could not read pins file: %w", err)
	}
	var pins map[string]EmptyStruct
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