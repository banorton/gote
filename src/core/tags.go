package core

import (
	"fmt"
	"sort"

	"gote/src/data"
)

func GetPopularTags(limit int) ([]data.TagMeta, error) {
	tags, err := data.LoadTags()
	if err != nil {
		return nil, fmt.Errorf("error loading tags: %w", err)
	}

	var tagSlice []data.TagMeta
	for _, tag := range tags {
		tagSlice = append(tagSlice, tag)
	}

	sort.Slice(tagSlice, func(i, j int) bool {
		return tagSlice[i].Count > tagSlice[j].Count
	})

	if limit > 0 && limit < len(tagSlice) {
		tagSlice = tagSlice[:limit]
	}

	return tagSlice, nil
}