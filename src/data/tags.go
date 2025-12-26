package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type TagMeta struct {
	Tag   string   `json:"tag"`
	Notes []string `json:"notes"`
	Count int      `json:"count"`
}

func TagsPath() string {
	return filepath.Join(GoteDir(), "tags.json")
}

func UpdateTagsIndex(notes map[string]NoteMeta) error {
	tagMap := make(map[string]TagMeta)
	for _, note := range notes {
		for _, tag := range note.Tags {
			tm := tagMap[tag]
			tm.Tag = tag
			tm.Notes = append(tm.Notes, note.FilePath)
			tm.Count++
			tagMap[tag] = tm
		}
	}
	f, err := os.Create(TagsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tagMap)
}

func LoadTags() (map[string]TagMeta, error) {
	data, err := os.ReadFile(TagsPath())
	if err != nil {
		return nil, err
	}

	var tags map[string]TagMeta
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func FormatTagsFile() error {
	tagPath := TagsPath()
	data, err := os.ReadFile(tagPath)
	if err != nil {
		return fmt.Errorf("could not read tags file: %w", err)
	}
	var m map[string]TagMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("could not parse tags file: %w", err)
	}
	pretty, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal pretty tags: %w", err)
	}
	if err := os.WriteFile(TagsPath(), pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty tags: %w", err)
	}
	return nil
}