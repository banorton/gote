package data

import (
	"encoding/json"
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
	return FormatJSONFile(PinsPath())
}