package core

import (
	"path/filepath"
	"sort"
	"strings"

	"gote/src/data"
)

type SearchResult struct {
	Title    string
	FilePath string
	Score    int
}

func SearchNotesByTitle(query string, limit int) ([]SearchResult, error) {
	query = strings.ToLower(query)
	index := data.LoadIndex()
	var results []SearchResult

	for title := range index {
		if strings.Contains(strings.ToLower(title), query) {
			meta := index[title]
			results = append(results, SearchResult{
				Title:    title,
				FilePath: meta.FilePath,
				Score:    1,
			})
		}
	}

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}

func SearchNotesByTags(tags []string, limit int) ([]SearchResult, error) {
	tagsMap, err := data.LoadTags()
	if err != nil {
		return nil, err
	}

	noteCount := make(map[string]int)
	for _, tag := range tags {
		tm, exists := tagsMap[tag]
		if !exists {
			continue
		}
		for _, note := range tm.Notes {
			noteCount[note]++
		}
	}

	var results []SearchResult
	for notePath, count := range noteCount {
		title := strings.TrimSuffix(filepath.Base(notePath), ".md")
		results = append(results, SearchResult{
			Title:    title,
			FilePath: notePath,
			Score:    count,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}

func SearchTrash(query string) ([]string, error) {
	return data.SearchTrash(query)
}