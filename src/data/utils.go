package data

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func PrettyPrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(v)
		return
	}
	fmt.Println(string(data))
}

func OpenFileInEditor(filePath, editor string) error {
	if editor == "" {
		return fmt.Errorf("no editor specified in config")
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error opening editor: %w", err)
	}
	return nil
}

// ValidateNoteName checks if a note name is safe to use as a filename.
func ValidateNoteName(name string) error {
	if name == "" {
		return fmt.Errorf("note name cannot be empty")
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("note name cannot start with -")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("note name cannot contain / or \\")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("note name cannot contain ..")
	}
	if strings.ContainsAny(name, "\x00") {
		return fmt.Errorf("note name cannot contain null bytes")
	}
	return nil
}

// AtomicWriteJSON writes data to path atomically via temp file + rename.
func AtomicWriteJSON(path string, data interface{}) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gote-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encoding json: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing file: %w", err)
	}
	tmp.Close()

	return os.Rename(tmpPath, path)
}

