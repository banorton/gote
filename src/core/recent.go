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

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Modified > notes[j].Modified
	})

	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	return notes, nil
}