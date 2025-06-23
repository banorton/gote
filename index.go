package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type NoteMeta struct {
	FilePath  string `json:"filePath"`
	Title     string `json:"title"`
	Created   string `json:"created"`
	Modified  string `json:"modified"`
	WordCount int    `json:"wordCount"`
	CharCount int    `json:"charCount"`
}

func indexPath() string {
	return filepath.Join(goteDir(), "index.json")
}

func loadIndex() []NoteMeta {
	var notes []NoteMeta
	if data, err := os.ReadFile(indexPath()); err == nil {
		_ = json.Unmarshal(data, &notes)
	} else {
		fmt.Println("Error reading file during loadIndex:", err.Error())
	}
	return notes
}

func indexNotes() error {
	notesDir := noteDir()
	return filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		return indexNote(path)
	})
}

func indexNote(notePath string) error {
	indexFile := indexPath()
	var notes []NoteMeta
	if data, err := os.ReadFile(indexFile); err == nil {
		_ = json.Unmarshal(data, &notes)
	}

	info, err := os.Stat(notePath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(notePath)
	if err != nil {
		return err
	}
	text := string(data)
	title := strings.TrimSuffix(filepath.Base(notePath), ".md")
	wordCount := len(strings.Fields(text))
	charCount := len([]rune(text))

	created := info.ModTime().Format("060102.150405")
	modified := info.ModTime().Format("060102.150405")
	meta := NoteMeta{
		FilePath:  notePath,
		Title:     title,
		Created:   created,
		Modified:  modified,
		WordCount: wordCount,
		CharCount: charCount,
	}

	updated := false
	for i, n := range notes {
		if n.FilePath == notePath {
			notes[i] = meta
			updated = true
			break
		}
	}
	if !updated {
		notes = append(notes, meta)
	}

	f, err := os.Create(indexFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(notes)
}

func formatIndexFile() error {
	indPath := indexPath()
	data, err := os.ReadFile(indPath)
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}
	var m []NoteMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}
	pretty, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal pretty config: %w", err)
	}
	if err := os.WriteFile(indPath, pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty config: %w", err)
	}
	return nil
}
