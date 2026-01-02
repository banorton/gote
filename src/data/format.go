package data

import (
	"encoding/json"
	"fmt"
	"os"
)

// FormatJSONFile reads a JSON file and rewrites it with proper indentation.
// Works with any valid JSON file regardless of structure.
func FormatJSONFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("could not parse JSON: %w", err)
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("could not format JSON: %w", err)
	}

	if err := os.WriteFile(path, pretty, 0644); err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}

	return nil
}
