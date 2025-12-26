package core

import (
	"sort"

	"gote/src/data"
)

func GetRecentNotes(limit int) ([]data.NoteMeta, error) {
	index, err := data.LoadIndex()
	if err != nil {
		return nil, err
	}
	var notes []data.NoteMeta

	for _, n := range index {
		notes = append(notes, n)
	}

	// Sort by LastVisited (with Modified as fallback for notes never opened)
	sort.Slice(notes, func(i, j int) bool {
		vi := notes[i].LastVisited
		vj := notes[j].LastVisited
		if vi == "" {
			vi = notes[i].Modified
		}
		if vj == "" {
			vj = notes[j].Modified
		}
		return vi > vj
	})

	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	return notes, nil
}