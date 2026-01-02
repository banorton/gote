package data

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TemplatesDir returns the path to the templates directory
func TemplatesDir() string {
	return filepath.Join(GoteDir(), "templates")
}

// EnsureTemplatesDir creates the templates directory if it doesn't exist
func EnsureTemplatesDir() error {
	return os.MkdirAll(TemplatesDir(), 0755)
}

// templatePath returns the full path for a template file
func templatePath(name string) string {
	return filepath.Join(TemplatesDir(), name+".md")
}

// ListTemplateFiles returns the names of all templates (without .md extension)
func ListTemplateFiles() ([]string, error) {
	dir := TemplatesDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("could not read templates directory: %w", err)
	}

	var templates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") {
			templates = append(templates, strings.TrimSuffix(name, ".md"))
		}
	}

	sort.Strings(templates)
	return templates, nil
}

// LoadTemplate reads the content of a template file
func LoadTemplate(name string) (string, error) {
	path := templatePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("template '%s' not found", name)
		}
		return "", fmt.Errorf("could not read template: %w", err)
	}
	return string(data), nil
}

// SaveTemplate writes content to a template file
func SaveTemplate(name, content string) error {
	if err := EnsureTemplatesDir(); err != nil {
		return fmt.Errorf("could not create templates directory: %w", err)
	}

	path := templatePath(name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("could not write template: %w", err)
	}
	return nil
}

// DeleteTemplate removes a template file
func DeleteTemplate(name string) error {
	path := templatePath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("template '%s' not found", name)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("could not delete template: %w", err)
	}
	return nil
}

// TemplateExists checks if a template exists
func TemplateExists(name string) bool {
	path := templatePath(name)
	_, err := os.Stat(path)
	return err == nil
}
