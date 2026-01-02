package data

import (
	"encoding/json"
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
	return FormatJSONFile(TagsPath())
}