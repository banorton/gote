package note

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NoteMetadata holds metadata for a note file.
type NoteMetadata struct {
	Name         string // filename with relative path, e.g. "project/plan.md"
	Path         string // full absolute path
	Tags         []string
	Created      int64 // Unix timestamp for creation
	LastModified int64 // Unix timestamp for last edit
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

// ParseTagsFromFile reads the first line of a note and returns tags split by ' . ', lowercased and trimmed.
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
		tags := strings.Split(line, " . ")
		var cleanTags []string
		for _, tag := range tags {
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

// getOrCreateCreatedTime returns the creation time for a note, writing a .created file if needed
func getOrCreateCreatedTime(notePath string, modTime int64) int64 {
	createdPath := notePath + ".created"
	if fi, err := os.Stat(createdPath); err == nil {
		return fi.ModTime().Unix()
	}
	// If .created file doesn't exist, create it with current time
	now := time.Now().Unix()
	f, err := os.Create(createdPath)
	if err == nil {
		f.Close()
		os.Chtimes(createdPath, time.Unix(now, 0), time.Unix(now, 0))
		return now
	}
	// Fallback: use modTime
	return modTime
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
		created := getOrCreateCreatedTime(path, info.ModTime().Unix())
		notes = append(notes, NoteMetadata{
			Name:         rel,
			Path:         path,
			Tags:         tags,
			Created:      created,
			LastModified: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return notes, nil
}

// When creating a new note, write a .created file with the current timestamp
func WriteCreatedFile(notePath string) {
	createdPath := notePath + ".created"
	if _, err := os.Stat(createdPath); err == nil {
		return // already exists
	}
	now := time.Now().Unix()
	f, err := os.Create(createdPath)
	if err == nil {
		f.Close()
		os.Chtimes(createdPath, time.Unix(now, 0), time.Unix(now, 0))
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
		created := getOrCreateCreatedTime(path, info.ModTime().Unix())
		if !ok || m.LastModified != info.ModTime().Unix() {
			tags, _ := ParseTagsFromFile(path)
			m = NoteMetadata{
				Name:         rel,
				Path:         path,
				Tags:         tags,
				Created:      created,
				LastModified: info.ModTime().Unix(),
			}
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

// PrintTabular prints note names in a compact table, truncating to 10 chars per column.
func PrintTabular(notes []NoteMetadata) {
	colWidth := 12
	cols := 6
	for i, n := range notes {
		title := filepath.Base(n.Path)
		title = strings.TrimSuffix(title, ".md")
		if len(title) > 10 {
			title = title[:10]
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
