package core

import (
	"os"
	"sort"
	"time"

	"gote/src/data"
)

func GetRecentNotes(limit int) ([]data.NoteMeta, error) {
	index := data.LoadIndex()
	var notes []data.NoteMeta

	for _, n := range index {
		notes = append(notes, n)
	}

	// Cache mod times to avoid O(n log n) stat calls during sort
	modTimes := make(map[string]time.Time, len(notes))
	for _, n := range notes {
		info, err := os.Stat(n.FilePath)
		if err == nil {
			modTimes[n.FilePath] = info.ModTime()
		}
	}

	sort.Slice(notes, func(i, j int) bool {
		return modTimes[notes[i].FilePath].After(modTimes[notes[j].FilePath])
	})

	if limit > 0 && limit < len(notes) {
		notes = notes[:limit]
	}

	return notes, nil
}