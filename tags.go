package main

import (
	"path/filepath"
	"os"
	"encoding/json"
	"fmt"
)

type TagMeta struct {
	Tag      string   `json:"tag"`
	Notes    []string `json:"notes"`
	Count    int      `json:"count"`
}

func tagsPath() string {
	return filepath.Join(goteDir(), "tags.json")
}

func updateTagsIndex(notes []NoteMeta) error {
	tagMap := make(map[string]*TagMeta)
	for _, note := range notes {
		for _, tag := range note.Tags {
			tm, exists := tagMap[tag]
			if !exists {
				tm = &TagMeta{Tag: tag}
				tagMap[tag] = tm
			}
			tm.Notes = append(tm.Notes, note.FilePath)
			tm.Count++
		}
	}
	var tags []TagMeta
	for _, tm := range tagMap {
		tags = append(tags, *tm)
	}
	f, err := os.Create(tagsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tags)
}

func formatTagsFile() error {
	tagPath := tagsPath()
	data, err := os.ReadFile(tagPath)
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}
	var m []TagMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("could not parse tags file: %w", err)
	}
	pretty, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal pretty tags: %w", err)
	}
	if err := os.WriteFile(tagsPath(), pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty tags: %w", err)
	}
	return nil
}

