package main

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

func tagsPath() string {
	return filepath.Join(goteDir(), "tags.json")
}

func updateTagsIndex(notes map[string]NoteMeta) error {
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
	tagsOut := make(map[string]TagMeta)
	for k, v := range tagMap {
		tagsOut[k] = *v
	}
	f, err := os.Create(tagsPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tagsOut)
}

func formatTagsFile() error {
	tagPath := tagsPath()
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
	if err := os.WriteFile(tagsPath(), pretty, 0644); err != nil {
		return fmt.Errorf("could not write pretty tags: %w", err)
	}
	return nil
}
