package core

import (
	"path/filepath"
	"sort"
	"strings"

	"gote/src/data"
)

// sortResultsByTitle sorts search results alphabetically by title
func sortResultsByTitle(results []SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Title < results[j].Title
	})
}

type SearchResult struct {
	Title    string
	FilePath string
	Score    int
}

func SearchNotesByTitle(query string, limit int) ([]SearchResult, error) {
	query = strings.ToLower(query)
	index, err := data.LoadIndex()
	if err != nil {
		return nil, err
	}
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

	sortResultsByTitle(results)

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}

// SearchNotesByTags returns notes matching ANY of the specified tags (OR logic)
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

// FilterNotesByTags returns notes that have ALL specified tags (AND logic)
func FilterNotesByTags(tags []string, limit int) ([]SearchResult, error) {
	tagsMap, err := data.LoadTags()
	if err != nil {
		return nil, err
	}

	if len(tags) == 0 {
		return nil, nil
	}

	// Count how many of the specified tags each note has
	noteCount := make(map[string]int)
	for _, tag := range tags {
		tm, exists := tagsMap[tag]
		if !exists {
			// If any tag doesn't exist, no notes can match all tags
			return nil, nil
		}
		for _, note := range tm.Notes {
			noteCount[note]++
		}
	}

	// Only include notes that have ALL the specified tags
	var results []SearchResult
	requiredCount := len(tags)
	for notePath, count := range noteCount {
		if count == requiredCount {
			title := strings.TrimSuffix(filepath.Base(notePath), ".md")
			results = append(results, SearchResult{
				Title:    title,
				FilePath: notePath,
				Score:    count,
			})
		}
	}

	sortResultsByTitle(results)

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	return results, nil
}
