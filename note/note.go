package note

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type NoteMetadata struct {
	Path         string
	Tags         []string
	LastModified time.Time
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
			t = strings.ToLower(strings.TrimSpace(tag))
			if t != "" {
				cleanTags = append(cleanTags, t)
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

// IndexNotes walks notesDir recursively, indexing all .md files.
func IndexNotes(notesDir string) ([]NoteMetadata, error) {
	var notes []NoteMetadata
	err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		tags, tagErr := ParseTagsFromFile(path)
		if tagErr != nil {
			return tagErr
		}
		notes = append(notes, NoteMetadata{
			Path:         path,
			Tags:         tags,
			LastModified: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return notes, nil
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
