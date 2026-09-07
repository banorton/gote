package core

import (
	"sort"

	"github.com/banorton/gote/src/data"
)

// lastTouched is the more recent of opening the note through gote and writing
// the file. Using LastVisited alone hides notes edited outside gote -- in vim
// directly, in another editor, or by a sync -- since those never update it.
// Both timestamps use the same sortable YYMMDD.HHMMSS layout.
func lastTouched(n data.NoteMeta) string {
	if n.Modified > n.LastVisited {
		return n.Modified
	}
	return n.LastVisited
}

func GetRecentNotes(limit int) ([]data.NoteMeta, error) {
	index, err := data.LoadIndex()
	if err != nil {
		return nil, err
	}
	notes := make([]data.NoteMeta, 0, len(index))
	for _, n := range index {
		notes = append(notes, n)
	}

	sort.Slice(notes, func(i, j int) bool {
		return lastTouched(notes[i]) > lastTouched(notes[j])
	})

	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	return notes, nil
}
