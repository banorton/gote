package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type NoteMeta struct {
	FilePath    string   `json:"filePath"`
	Title       string   `json:"title"`
	Created     string   `json:"created"`
	Modified    string   `json:"modified"`
	LastVisited string   `json:"lastVisited,omitempty"`
	WordCount   int      `json:"wordCount"`
	CharCount   int      `json:"charCount"`
	Tags        []string `json:"tags"`
}

func IndexPath() string {
	return filepath.Join(GoteDir(), "index.json")
}

func LoadIndex() (map[string]NoteMeta, error) {
	index := make(map[string]NoteMeta)
	data, err := os.ReadFile(IndexPath())
	if os.IsNotExist(err) {
		return index, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading index file: %w", err)
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parsing index file: %w", err)
	}
	return index, nil
}

func SaveIndex(index map[string]NoteMeta) error {
	f, err := os.Create(IndexPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(index)
}

// SaveIndexWithTags atomically saves the index and updates the tags index.
// Use this instead of separate SaveIndex + UpdateTagsIndex calls.
func SaveIndexWithTags(index map[string]NoteMeta) error {
	if err := SaveIndex(index); err != nil {
		return err
	}
	return UpdateTagsIndex(index)
}

func IndexNotes(notesDir string) error {
	existingIndex, err := LoadIndex()
	if err != nil {
		return fmt.Errorf("loading existing index: %w", err)
	}
	index := make(map[string]NoteMeta)
	err = filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
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

		if existing, ok := existingIndex[meta.Title]; ok {
			if existing.Created != "" {
				meta.Created = existing.Created
			}
			if existing.LastVisited != "" {
				meta.LastVisited = existing.LastVisited
			}
		}

		index[meta.Title] = meta
		return nil
	})
	if err != nil {
		return err
	}

	if err := SaveIndexWithTags(index); err != nil {
		return err
	}

	return FormatTagsFile()
}

func IndexNote(notePath string) error {
	index, err := LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	info, err := os.Stat(notePath)
	if err != nil {
		return err
	}

	meta, err := BuildNoteMeta(notePath, info)
	if err != nil {
		return err
	}

	if existing, ok := index[meta.Title]; ok {
		if existing.Created != "" {
			meta.Created = existing.Created
		}
		if existing.LastVisited != "" {
			meta.LastVisited = existing.LastVisited
		}
	}

	index[meta.Title] = meta
	return SaveIndexWithTags(index)
}

func BuildNoteMeta(notePath string, info os.FileInfo) (NoteMeta, error) {
	data, err := os.ReadFile(notePath)
	if err != nil {
		return NoteMeta{}, err
	}
	text := string(data)
	title := strings.TrimSuffix(filepath.Base(notePath), ".md")
	wordCount := len(strings.Fields(text))
	charCount := utf8.RuneCountInString(text)
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
	return FormatJSONFile(IndexPath())
}