package data

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

func IndexPath() string {
	return filepath.Join(GoteDir(), "index.json")
}

func LoadIndex() map[string]NoteMeta {
	index := make(map[string]NoteMeta)
	data, err := os.ReadFile(IndexPath())
	if os.IsNotExist(err) {
		return index
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading index file:", err.Error())
		return index
	}
	if err := json.Unmarshal(data, &index); err != nil {
		fmt.Fprintln(os.Stderr, "Error parsing index file:", err.Error())
	}
	return index
}

func SaveIndex(index map[string]NoteMeta) error {
	f, err := os.Create(IndexPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(index)
}

func IndexNotes(notesDir string) error {
	existingIndex := LoadIndex()
	index := make(map[string]NoteMeta)
	err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		meta, err := BuildNoteMeta(path, info)
		if err != nil {
			return err
		}

		if existing, ok := existingIndex[meta.Title]; ok && existing.Created != "" {
			meta.Created = existing.Created
		}

		index[meta.Title] = meta
		return nil
	})
	if err != nil {
		return err
	}

	if err := SaveIndex(index); err != nil {
		return err
	}

	if err := UpdateTagsIndex(index); err != nil {
		return err
	}

	return FormatTagsFile()
}

func IndexNote(notePath string) error {
	index := LoadIndex()
	info, err := os.Stat(notePath)
	if err != nil {
		return err
	}

	meta, err := BuildNoteMeta(notePath, info)
	if err != nil {
		return err
	}

	if existing, ok := index[meta.Title]; ok && existing.Created != "" {
		meta.Created = existing.Created
	}

	index[meta.Title] = meta
	if err := SaveIndex(index); err != nil {
		return err
	}

	return UpdateTagsIndex(index)
}

func BuildNoteMeta(notePath string, info os.FileInfo) (NoteMeta, error) {
	data, err := os.ReadFile(notePath)
	if err != nil {
		return NoteMeta{}, err
	}
	text := string(data)
	title := strings.TrimSuffix(filepath.Base(notePath), ".md")
	wordCount := len(strings.Fields(text))
	charCount := len([]rune(text))
	created := GetBirthtime(info).Format("060102.150405")
	modified := info.ModTime().Format("060102.150405")
	firstLine := ""
	scanner := bufio.NewScanner(strings.NewReader(text))
	if scanner.Scan() {
		firstLine = scanner.Text()
	}
	tags := ParseTags(firstLine)
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

func ParseTags(line string) []string {
	// Tags must start with a period
	if !strings.HasPrefix(line, ".") {
		return nil
	}
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

func FormatIndexFile() error {
	indPath := IndexPath()
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