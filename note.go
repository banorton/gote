package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// NoteMetadata holds metadata for a note file.
type NoteMetadata struct {
	Name         string // filename with relative path, e.g. "project/plan.md"
	Path         string // full absolute path
	Tags         []string
	Created      int64  // Unix timestamp for creation
	LastModified int64  // Unix timestamp for last edit
	CreatedStr   string // yymmdd.hhmmss format
	ModifiedStr  string // yymmdd.hhmmss format
	AccessCount  int    // number of times accessed
	WordCount    int    // number of words in note
	CharCount    int    // number of characters in note
}

func indexFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gote", "index.json")
}

// SaveIndex writes the index to ~/.gote/index.json
func SaveIndex(index []NoteMetadata) error {
	dir := filepath.Dir(indexFilePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(indexFilePath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(index)
}

// LoadIndex loads the index from ~/.gote/index.json
func LoadIndex() ([]NoteMetadata, error) {
	f, err := os.Open(indexFilePath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var index []NoteMetadata
	if err := json.NewDecoder(f).Decode(&index); err != nil {
		return nil, err
	}
	return index, nil
}

// ParseTagsFromFile reads the first line of a note and returns tags split by periods (with optional spaces), lowercased and trimmed.
func ParseTagsFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			return nil, nil
		}
		// Remove #, [, ], and | characters from the line before splitting
		cleanLine := strings.ReplaceAll(line, "#", "")
		cleanLine = strings.ReplaceAll(cleanLine, "[", "")
		cleanLine = strings.ReplaceAll(cleanLine, "]", "")
		cleanLine = strings.ReplaceAll(cleanLine, "|", "")
		// Split on periods with optional spaces around them
		parts := strings.Split(cleanLine, ".")
		var cleanTags []string
		for _, tag := range parts {
			tag = strings.ToLower(strings.TrimSpace(tag))
			if tag != "" {
				cleanTags = append(cleanTags, tag)
			}
		}
		return cleanTags, nil
	}
	return nil, nil
}

// SetTags replaces the first line of the note file with the given tags (joined by ' . ').
func SetTags(path string, tags []string) error {
	if !strings.HasSuffix(path, ".md") {
		return fmt.Errorf("file is not a markdown file: %s", path)
	}
	for i, t := range tags {
		tags[i] = strings.ToLower(strings.TrimSpace(t))
	}
	tagLine := strings.Join(tags, " . ")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.SplitN(string(data), "\n", 2)
	var rest string
	if len(lines) > 1 {
		rest = lines[1]
	}
	newContent := tagLine + "\n" + rest
	return os.WriteFile(path, []byte(newContent), 0644)
}

// shouldSkipDir returns true if the directory should be skipped during indexing
func shouldSkipDir(name string) bool {
	if name == "archive" || name == "quick" || name == ".git" || strings.HasPrefix(name, ".") {
		return true
	}
	return false
}

// getCreatedFilePath returns the path to the .created file in ~/.gote/created/ for a given note path
func getCreatedFilePath(notePath string) string {
	home, _ := os.UserHomeDir()
	// Find relative path from notes dir
	cfgPath := filepath.Join(home, ".gote", "config.json")
	cfgFile, err := os.Open(cfgPath)
	var notesDir string
	if err == nil {
		var cfg struct {
			NotesDir string `json:"notesDir"`
		}
		_ = json.NewDecoder(cfgFile).Decode(&cfg)
		notesDir = cfg.NotesDir
		cfgFile.Close()
	}
	if notesDir == "" {
		notesDir = filepath.Join(home, "gotes")
	}
	rel, _ := filepath.Rel(notesDir, notePath)
	return filepath.Join(home, ".gote", "created", rel)
}

// getOrCreateCreatedTime returns the creation time for a note, writing a .created file in ~/.gote/created/ if needed
func getOrCreateCreatedTime(notePath string, modTime int64) (int64, string) {
	createdPath := getCreatedFilePath(notePath)
	if fi, err := os.Stat(createdPath); err == nil {
		ts := fi.ModTime().Unix()
		return ts, time.Unix(ts, 0).Format("060102.150405")
	}
	// If .created file doesn't exist, create it with current time
	now := time.Now().Unix()
	if err := os.MkdirAll(filepath.Dir(createdPath), 0755); err == nil {
		f, err := os.Create(createdPath)
		if err == nil {
			f.Close()
			os.Chtimes(createdPath, time.Unix(now, 0), time.Unix(now, 0))
			return now, time.Unix(now, 0).Format("060102.150405")
		}
	}
	// Fallback: use modTime
	return modTime, time.Unix(modTime, 0).Format("060102.150405")
}

// Helper to count words and chars in a file
func CountWordsAndChars(path string) (int, int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	text := string(data)
	// Remove tag line
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[i+1:]
	}
	words := 0
	inWord := false
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			inWord = false
		} else if !inWord {
			words++
			inWord = true
		}
	}
	return words, len([]rune(text)), nil
}

// IndexNotes walks notesDir recursively, skipping certain dirs, and returns all .md file metadata
func IndexNotes(notesDir string) ([]NoteMetadata, error) {
	var notes []NoteMetadata
	absNotesDir, _ := filepath.Abs(notesDir)
	err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		tags, tagErr := ParseTagsFromFile(path)
		if tagErr != nil {
			return tagErr
		}
		rel, _ := filepath.Rel(absNotesDir, path)
		created, createdStr := getOrCreateCreatedTime(path, info.ModTime().Unix())
		modStr := time.Unix(info.ModTime().Unix(), 0).Format("060102.150405")
		wc, cc, _ := CountWordsAndChars(path)
		notes = append(notes, NoteMetadata{
			Name:         rel,
			Path:         path,
			Tags:         tags,
			Created:      created,
			LastModified: info.ModTime().Unix(),
			CreatedStr:   createdStr,
			ModifiedStr:  modStr,
			WordCount:    wc,
			CharCount:    cc,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return notes, nil
}

// When creating a new note, write a .created file in ~/.gote/created/
func WriteCreatedFile(notePath string) {
	createdPath := getCreatedFilePath(notePath)
	if _, err := os.Stat(createdPath); err == nil {
		return // already exists
	}
	now := time.Now().Unix()
	if err := os.MkdirAll(filepath.Dir(createdPath), 0755); err == nil {
		f, err := os.Create(createdPath)
		if err == nil {
			f.Close()
			os.Chtimes(createdPath, time.Unix(now, 0), time.Unix(now, 0))
		}
	}
}

// RefreshIndex loads the index and updates entries for changed/missing files, or rebuilds if force is true
func RefreshIndex(notesDir string, force bool) ([]NoteMetadata, error) {
	if force {
		return IndexNotes(notesDir)
	}
	index, err := LoadIndex()
	if err != nil {
		return IndexNotes(notesDir)
	}
	// Build a map for quick lookup
	metaMap := make(map[string]NoteMetadata)
	for _, m := range index {
		metaMap[m.Path] = m
	}
	var updated []NoteMetadata
	absNotesDir, _ := filepath.Abs(notesDir)
	filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && shouldSkipDir(info.Name()) {
			return filepath.SkipDir
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(absNotesDir, path)
		m, ok := metaMap[path]
		created, createdStr := getOrCreateCreatedTime(path, info.ModTime().Unix())
		modStr := time.Unix(info.ModTime().Unix(), 0).Format("060102.150405")
		wc, cc, _ := CountWordsAndChars(path)
		if !ok || m.LastModified != info.ModTime().Unix() {
			tags, _ := ParseTagsFromFile(path)
			m = NoteMetadata{
				Name:         rel,
				Path:         path,
				Tags:         tags,
				Created:      created,
				LastModified: info.ModTime().Unix(),
				CreatedStr:   createdStr,
				ModifiedStr:  modStr,
				WordCount:    wc,
				CharCount:    cc,
			}
		} else {
			m.CreatedStr = createdStr
			m.ModifiedStr = modStr
			m.WordCount = wc
			m.CharCount = cc
		}
		updated = append(updated, m)
		return nil
	})
	return updated, nil
}

// TagsFrequency returns a map of tag -> count for all notes in notesDir.
func TagsFrequency(notesDir string) (map[string]int, error) {
	notes, err := IndexNotes(notesDir)
	if err != nil {
		return nil, err
	}
	tagCounts := make(map[string]int)
	for _, n := range notes {
		for _, tag := range n.Tags {
			tagCounts[tag]++
		}
	}
	return tagCounts, nil
}

// NotesWithAllTags returns notes that contain all the specified tags.
func NotesWithAllTags(notes []NoteMetadata, tags []string) []NoteMetadata {
	var result []NoteMetadata
	// Normalize tags without mutating input
	normTags := make([]string, len(tags))
	for i, t := range tags {
		normTags[i] = strings.ToLower(strings.TrimSpace(t))
	}
	for _, n := range notes {
		tagSet := make(map[string]struct{})
		for _, t := range n.Tags {
			tagSet[t] = struct{}{}
		}
		all := true
		for _, t := range normTags {
			if _, ok := tagSet[t]; !ok {
				all = false
				break
			}
		}
		if all {
			result = append(result, n)
		}
	}
	return result
}

// PrintTabular prints note names in a compact table, truncating to 20 chars per column.
func PrintTabular(notes []NoteMetadata) {
	colWidth := 20
	cols := 6
	for i, n := range notes {
		title := filepath.Base(n.Path)
		title = strings.TrimSuffix(title, ".md")
		if len(title) > 20 {
			title = title[:20]
		}
		fmt.Printf("%-*s", colWidth, title)
		if (i+1)%cols == 0 {
			fmt.Println()
		}
	}
	fmt.Println()
}

// Pin management
func pinFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gote", "pinned.json")
}

// PinNote adds a note to the pinned list
func PinNote(relPath string) error {
	pins, _ := ListPinned()
	for _, p := range pins {
		if p == relPath {
			return nil // already pinned
		}
	}
	pins = append(pins, relPath)
	dir := filepath.Dir(pinFilePath())
	_ = os.MkdirAll(dir, 0755)
	f, err := os.Create(pinFilePath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(pins)
}

// UnpinNote removes a note from the pinned list
func UnpinNote(relPath string) error {
	pins, _ := ListPinned()
	var newPins []string
	for _, p := range pins {
		if p != relPath {
			newPins = append(newPins, p)
		}
	}
	dir := filepath.Dir(pinFilePath())
	_ = os.MkdirAll(dir, 0755)
	f, err := os.Create(pinFilePath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(newPins)
}

// ListPinned returns a slice of pinned relative note paths
func ListPinned() ([]string, error) {
	f, err := os.Open(pinFilePath())
	if err != nil {
		return nil, nil // no pins yet
	}
	defer f.Close()
	var pins []string
	_ = json.NewDecoder(f).Decode(&pins)
	return pins, nil
}

// IsPinned checks if a note is pinned
func IsPinned(relPath string) bool {
	pins, _ := ListPinned()
	for _, p := range pins {
		if p == relPath {
			return true
		}
	}
	return false
}

// ArchiveNote moves a note to the archive/ subdir inside notesDir
func ArchiveNote(notesDir, relPath string) error {
	absNotesDir, _ := filepath.Abs(notesDir)
	src := filepath.Join(absNotesDir, relPath)
	archiveDir := filepath.Join(absNotesDir, "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return err
	}
	dst := filepath.Join(archiveDir, filepath.Base(relPath))
	return os.Rename(src, dst)
}

// Access log file path
func accessLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gote", "access.json")
}

var accessMu sync.Mutex

// IncrementAccess increments the access count for a note and persists it.
func IncrementAccess(relPath string) error {
	accessMu.Lock()
	defer accessMu.Unlock()
	accessMap, _ := LoadAccessLog()
	accessMap[relPath]++
	return SaveAccessLog(accessMap)
}

// LoadAccessLog loads the access log from disk.
func LoadAccessLog() (map[string]int, error) {
	path := accessLogPath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]int), nil
		}
		return nil, err
	}
	defer f.Close()
	var m map[string]int
	if err := json.NewDecoder(f).Decode(&m); err != nil {
		return make(map[string]int), nil
	}
	return m, nil
}

// SaveAccessLog saves the access log to disk.
func SaveAccessLog(m map[string]int) error {
	path := accessLogPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(m)
}

// MergeAccessCounts merges access counts into the metadata slice.
func MergeAccessCounts(notes []NoteMetadata, accessMap map[string]int) []NoteMetadata {
	for i := range notes {
		notes[i].AccessCount = accessMap[notes[i].Name]
	}
	return notes
}

// PopularNotes returns the top N notes by access count.
func PopularNotes(notes []NoteMetadata, N int) []NoteMetadata {
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].AccessCount > notes[j].AccessCount
	})
	if N > len(notes) {
		N = len(notes)
	}
	return notes[:N]
}

// UpdateNoteInIndex updates the index entry for a single note, or reindexes all if links are involved.
func UpdateNoteInIndex(notesDir, noteName string, forceFull bool) error {
	index, err := LoadIndex()
	if err != nil {
		index = []NoteMetadata{}
	}
	absNotesDir, _ := filepath.Abs(notesDir)
	notePath := filepath.Join(absNotesDir, noteName)
	if !strings.HasSuffix(notePath, ".md") {
		notePath += ".md"
	}
	// If forceFull, do a full reindex
	if forceFull {
		notes, err := IndexNotes(notesDir)
		if err != nil {
			return err
		}
		return SaveIndex(notes)
	}
	// Otherwise, update just this note's metadata
	info, err := os.Stat(notePath)
	if err != nil {
		return err
	}
	tags, _ := ParseTagsFromFile(notePath)
	rel, _ := filepath.Rel(absNotesDir, notePath)
	created, createdStr := getOrCreateCreatedTime(notePath, info.ModTime().Unix())
	modStr := time.Unix(info.ModTime().Unix(), 0).Format("060102.150405")
	wc, cc, _ := CountWordsAndChars(notePath)
	meta := NoteMetadata{
		Name:         rel,
		Path:         notePath,
		Tags:         tags,
		Created:      created,
		LastModified: info.ModTime().Unix(),
		CreatedStr:   createdStr,
		ModifiedStr:  modStr,
		WordCount:    wc,
		CharCount:    cc,
	}
	// Replace or add in index
	found := false
	for i := range index {
		if index[i].Name == rel {
			index[i] = meta
			found = true
			break
		}
	}
	if !found {
		index = append(index, meta)
	}
	return SaveIndex(index)
}
