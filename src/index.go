package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type NoteMeta struct {
	FilePath  string   `json:"filePath"`
	Title     string   `json:"title"`
	Created   string   `json:"created"`
	Modified  string   `json:"modified"`
	WordCount int      `json:"wordCount"`
	CharCount int      `json:"charCount"`
	Tags      []string `json:"tags"`
}

func indexPath() string {
	return filepath.Join(goteDir(), "index.json")
}

func loadIndex() map[string]NoteMeta {
	index := make(map[string]NoteMeta)
	if data, err := os.ReadFile(indexPath()); err == nil {
		_ = json.Unmarshal(data, &index)
	} else {
		fmt.Println("Error reading file during loadIndex:", err.Error())
	}
	return index
}

func indexNotes() error {
	notesDir := noteDir()
	index := make(map[string]NoteMeta)
	err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		meta, err := buildNoteMeta(path, info)
		if err != nil {
			return err
		}

		index[meta.Title] = meta
		return nil
	})
	if err != nil {
		return err
	}

	f, err := os.Create(indexPath())
	if err != nil {
		return err
	}

	defer f.Close()
	if err := json.NewEncoder(f).Encode(index); err != nil {
		return err
	}

	if err := updateTagsIndex(index); err != nil {
		return err
	}

	if err := formatTagsFile(); err != nil {
		return err
	}

	return nil
}

func indexNote(notePath string) error {
	index := loadIndex()
	info, err := os.Stat(notePath)
	if err != nil {
		return err
	}

	meta, err := buildNoteMeta(notePath, info)
	if err != nil {
		return err
	}

	index[meta.Title] = meta
	f, err := os.Create(indexPath())
	if err != nil {
		return err
	}

	defer f.Close()
	if err := json.NewEncoder(f).Encode(index); err != nil {
		return err
	}

	return updateTagsIndex(index)
}

func buildNoteMeta(notePath string, info os.FileInfo) (NoteMeta, error) {
	data, err := os.ReadFile(notePath)
	if err != nil {
		return NoteMeta{}, err
	}
	text := string(data)
	title := strings.TrimSuffix(filepath.Base(notePath), ".md")
	wordCount := len(strings.Fields(text))
	charCount := len([]rune(text))
	created := info.ModTime().Format("060102.150405")
	modified := info.ModTime().Format("060102.150405")
	firstLine := ""
	scanner := bufio.NewScanner(strings.NewReader(text))
	if scanner.Scan() {
		firstLine = scanner.Text()
	}
	tags := parseTags(firstLine)
	meta := NoteMeta{
		FilePath:  notePath,
		Title:     title,
		Created:   created,
		Modified:  modified,
		WordCount: wordCount,
		CharCount: charCount,
		Tags:      tags,
	}
	return meta, nil
}

func parseTags(line string) []string {
	clean := strings.ReplaceAll(line, "#", "")
	clean = strings.ReplaceAll(clean, "[", "")
	clean = strings.ReplaceAll(clean, "]", "")
	clean = strings.ReplaceAll(clean, "|", "")
	parts := strings.Split(clean, ".")
	var tags []string
	for _, part := range parts {
		tag := strings.ToLower(part)
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func formatIndexFile() error {
	indPath := indexPath()
	data, err := os.ReadFile(indPath)
	if err != nil {
		return fmt.Errorf("could not read index file: %w", err)
	}
	var m map[string]NoteMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("could not parse index file: %w", err)
	}
	pretty, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal pretty index: %w", err)
	}
	if err := os.WriteFile(indPath, pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty index: %w", err)
	}
	return nil
}
