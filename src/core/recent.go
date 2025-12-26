package core

import (
	"sort"

	"gote/src/data"
)

func GetRecentNotes(limit int) ([]data.NoteMeta, error) {
	index := data.LoadIndex()
	var notes []data.NoteMeta

	for _, n := range index {
		notes = append(notes, n)
	}

	// Sort by Modified field (yymmdd.hhmmss format sorts correctly as string)
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Modified > notes[j].Modified
	})

	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	return notes, nil
}