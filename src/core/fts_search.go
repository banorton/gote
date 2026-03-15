package core

import (
	"math"
	"sort"

	"gote/src/data"
)

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// SearchNotesFullText performs BM25-ranked full-text search
func SearchNotesFullText(query string, limit int) ([]SearchResult, error) {
	queryTerms := data.Tokenize(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}

	idx, err := data.LoadFTS()
	if err != nil {
		return nil, err
	}
	if len(idx) == 0 {
		return nil, nil
	}

	// Compute average document length
	var totalLen int
	for _, doc := range idx {
		totalLen += doc.Length
	}
	avgdl := float64(totalLen) / float64(len(idx))
	N := float64(len(idx))

	// Count document frequency for each query term
	df := make(map[string]int)
	for _, term := range queryTerms {
		for _, doc := range idx {
			if doc.Terms[term] > 0 {
				df[term]++
			}
		}
	}

	// Score each document
	type scored struct {
		title    string
		filePath string
		score    float64
	}
	var results []scored

	for _, doc := range idx {
		var score float64
		for _, term := range queryTerms {
			tf := float64(doc.Terms[term])
			if tf == 0 {
				continue
			}
			// IDF: log((N - df + 0.5) / (df + 0.5) + 1)
			dfVal := float64(df[term])
			idf := math.Log((N-dfVal+0.5)/(dfVal+0.5) + 1)
			// BM25 term score
			dl := float64(doc.Length)
			num := tf * (bm25K1 + 1)
			denom := tf + bm25K1*(1-bm25B+bm25B*dl/avgdl)
			score += idf * num / denom
		}
		if score > 0 {
			results = append(results, scored{
				title:    doc.Title,
				filePath: doc.FilePath,
				score:    score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}

	var out []SearchResult
	for _, r := range results {
		out = append(out, SearchResult{
			Title:    r.title,
			FilePath: r.filePath,
			Score:    int(r.score * 100), // scale for display
		})
	}
	return out, nil
}

// SearchNotesCombined runs both title and full-text search, deduplicating by FilePath
func SearchNotesCombined(query string, limit int) ([]SearchResult, error) {
	titleResults, err := SearchNotesByTitle(query, -1)
	if err != nil {
		return nil, err
	}

	ftsResults, err := SearchNotesFullText(query, -1)
	if err != nil {
		return nil, err
	}

	// Merge: keep higher score per filepath
	seen := make(map[string]SearchResult)
	for _, r := range titleResults {
		// Boost title matches so they rank well
		r.Score = r.Score * 1000
		seen[r.FilePath] = r
	}
	for _, r := range ftsResults {
		if existing, ok := seen[r.FilePath]; ok {
			if r.Score > existing.Score {
				seen[r.FilePath] = r
			}
		} else {
			seen[r.FilePath] = r
		}
	}

	var combined []SearchResult
	for _, r := range seen {
		combined = append(combined, r)
	}

	sort.Slice(combined, func(i, j int) bool {
		if combined[i].Score != combined[j].Score {
			return combined[i].Score > combined[j].Score
		}
		return combined[i].Created > combined[j].Created
	})

	if limit > 0 && limit < len(combined) {
		combined = combined[:limit]
	}

	return combined, nil
}
