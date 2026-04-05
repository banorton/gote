package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/kljensen/snowball"
)

// DocTerms stores stemmed term frequencies for one document
type DocTerms struct {
	Title    string         `json:"title"`
	FilePath string         `json:"filePath"`
	Terms    map[string]int `json:"terms"`
	Length   int            `json:"length"`
}

// FTSIndex maps note title to its term data
type FTSIndex map[string]DocTerms

func FTSPath() string {
	return filepath.Join(GoteDir(), "fts.json")
}

func LoadFTS() (FTSIndex, error) {
	idx := make(FTSIndex)
	data, err := os.ReadFile(FTSPath())
	if os.IsNotExist(err) {
		return idx, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading FTS index: %w", err)
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing FTS index: %w", err)
	}
	return idx, nil
}

func SaveFTS(idx FTSIndex) error {
	return AtomicWriteJSON(FTSPath(), idx)
}

var stopWords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true,
	"at": true, "be": true, "but": true, "by": true, "for": true,
	"if": true, "in": true, "into": true, "is": true, "it": true,
	"no": true, "not": true, "of": true, "on": true, "or": true,
	"such": true, "that": true, "the": true, "their": true, "then": true,
	"there": true, "these": true, "they": true, "this": true, "to": true,
	"was": true, "will": true, "with": true, "from": true, "have": true,
	"has": true, "had": true, "been": true, "would": true, "could": true,
	"should": true, "do": true, "does": true, "did": true, "can": true,
	"may": true, "which": true, "who": true, "what": true, "when": true,
	"where": true, "how": true, "all": true, "each": true, "every": true,
	"both": true, "more": true, "other": true, "some": true, "its": true,
	"than": true, "also": true, "just": true, "about": true, "over": true,
}

// Tokenize splits text into stemmed tokens, filtering stop words
func Tokenize(text string) []string {
	// Split on non-alphanumeric characters
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var tokens []string
	for _, w := range words {
		if len(w) < 2 || stopWords[w] {
			continue
		}
		stemmed, err := snowball.Stem(w, "english", false)
		if err != nil || stemmed == "" {
			stemmed = w
		}
		tokens = append(tokens, stemmed)
	}
	return tokens
}

// BuildDocTerms creates term frequency data for a document.
// Title tokens are repeated 3x for weighting.
func BuildDocTerms(title, filePath, content string) DocTerms {
	titleTokens := Tokenize(title)
	contentTokens := Tokenize(content)

	terms := make(map[string]int)
	// Title weight: count title tokens 3x
	for _, t := range titleTokens {
		terms[t] += 3
	}
	for _, t := range contentTokens {
		terms[t]++
	}

	return DocTerms{
		Title:    title,
		FilePath: filePath,
		Terms:    terms,
		Length:   len(titleTokens)*3 + len(contentTokens),
	}
}

// IndexDocFTS updates a single document in the FTS index
func IndexDocFTS(title, filePath, content string) error {
	idx, err := LoadFTS()
	if err != nil {
		return err
	}
	idx[title] = BuildDocTerms(title, filePath, content)
	return SaveFTS(idx)
}

// IndexAllFTS rebuilds the entire FTS index from note files
func IndexAllFTS(notesDir string, index map[string]NoteMeta) error {
	idx := make(FTSIndex)
	for title, meta := range index {
		content, err := os.ReadFile(meta.FilePath)
		if err != nil {
			continue // skip unreadable files
		}
		idx[title] = BuildDocTerms(title, meta.FilePath, string(content))
	}
	return SaveFTS(idx)
}

// RemoveDocFTS removes a document from the FTS index
func RemoveDocFTS(title string) error {
	idx, err := LoadFTS()
	if err != nil {
		return err
	}
	delete(idx, title)
	return SaveFTS(idx)
}
