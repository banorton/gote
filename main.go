package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	args := os.Args

	if len(args) == 1 {
		quick(args)
	}

	switch args[1] {
	case "quick":
		quick(args[2:])
	case "recent":
		recent(args[2:])
	case "popular":
		popular()
	case "index":
		index(args[2:])
	case "tags":
		tags(args[2:])
	case "tag":
		tag(args[2:])
	case "config":
		config(args[2:])
	case "search":
	case "journal":
	case "today":
	case "calendar":
	case "transfer":
	case "pin":
	case "pinned":
	case "archive":
	case "view":
	case "lint":
	case "export":
	case "delete":
	case "rename":
	case "help":
	case "info":
	default:
		note(args[1:])
	}
}

func note(args []string) {
	noteName := strings.Join(args, " ")

	index := loadIndex()
	noteMeta, exists := index[noteName]
	notePath := ""
	if exists {
		notePath = noteMeta.FilePath
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	noteDir := cfg.NoteDir
	if err := os.MkdirAll(noteDir, 0755); err != nil {
		fmt.Println("Error creating notes directory:", err)
		return
	}

	if notePath == "" {
		notePath = filepath.Join(noteDir, noteName+".md")
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			f, err := os.Create(notePath)
			if err != nil {
				fmt.Println("Error creating note:", err)
				return
			}
			f.Close()
		}
	}

	if err := openFileInEditor(notePath); err != nil {
		fmt.Println("Error opening note in editor:", err)
	}
}

func quick(args []string) {
	if len(args) > 0 {
		fmt.Printf("gote quick does not take extra args. Got: %v\n", args)
		return
	}
	note([]string{"quick"})
}

func index(args []string) {
	if len(args) == 0 {
		if err := indexNotes(); err != nil {
			fmt.Println("Error indexing notes:", err)
		} else {
			fmt.Println("All notes indexed.")
		}
		return
	}

	switch args[0] {
	case "edit":
		if err := formatIndexFile(); err != nil {
			fmt.Println("Error trying to format index file. Got:", err.Error())
		}
		if err := openFileInEditor(indexPath()); err != nil {
			fmt.Println("Error trying to edit index file. Got:", err.Error())
		}
	default:
		noteName := args[0]
		index := loadIndex()
		noteMeta, exists := index[noteName]
		foundPath := ""
		if exists {
			foundPath = noteMeta.FilePath
		}
		if foundPath != "" {
			if err := indexNote(foundPath); err != nil {
				fmt.Println("Error updating index for note:", err)
			} else {
				fmt.Println("Index updated for note:", noteName)
			}
			return
		}
		notesDir := noteDir()
		var notePath string
		err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			base := strings.TrimSuffix(filepath.Base(path), ".md")
			if base == noteName {
				notePath = path
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			fmt.Println("Error searching notes directory:", err)
			return
		}
		if notePath != "" {
			if err := indexNote(notePath); err != nil {
				fmt.Println("Error indexing note:", err)
			} else {
				fmt.Println("Index created for note:", noteName)
			}
			return
		}
		fmt.Println("Note not found:", noteName)
	}
}

func recent(args []string) {
	index := loadIndex()
	var notes []NoteMeta
	for _, n := range index {
		notes = append(notes, n)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Modified > notes[j].Modified
	})

	n := 10
	if len(args) > 0 {
		if v, err := strconv.Atoi(args[0]); err == nil && v > 0 {
			n = v
		}
	}
	if n > len(notes) {
		n = len(notes)
	}

	for i := 0; i < n; i++ {
		fmt.Println(notes[i].Title)
	}
}

func popular() {

}

func tags(args []string) {
	if len(args) == 0 {
		tagsFile := tagsPath()
		data, err := os.ReadFile(tagsFile)
		if err != nil {
			fmt.Println("Could not read tags file:", err)
			return
		}

		var tags map[string]TagMeta
		if err := json.Unmarshal(data, &tags); err != nil {
			fmt.Println("Could not parse tags file:", err)
			return
		}
		for tagName, tag := range tags {
			fmt.Printf("%s (%d)\n", tagName, tag.Count)
		}
		return
	}

	switch args[0] {
	case "edit":
		if err := openFileInEditor(tagsPath()); err != nil {
			fmt.Println(err)
		}
	case "format":
		err := formatTagsFile()
		if err != nil {
			fmt.Println("Error formatting tags:", err)
			return
		}
		fmt.Println("Tags file formatted.")
	case "popular":
		n := 10
		if len(args) > 1 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		tagsFile := tagsPath()
		data, err := os.ReadFile(tagsFile)
		if err != nil {
			fmt.Println("Could not read tags file:", err)
			return
		}
		var tags map[string]TagMeta
		if err := json.Unmarshal(data, &tags); err != nil {
			fmt.Println("Could not parse tags file:", err)
			return
		}
		// Convert map to slice for sorting
		var tagSlice []TagMeta
		for _, tag := range tags {
			tagSlice = append(tagSlice, tag)
		}
		sort.Slice(tagSlice, func(i, j int) bool {
			return tagSlice[i].Count > tagSlice[j].Count
		})
		if n > len(tagSlice) {
			n = len(tagSlice)
		}
		fmt.Printf("Top %d tags by usage:\n", n)
		for i := 0; i < n; i++ {
			tag := tagSlice[i]
			fmt.Printf("%s (%d)\n", tag.Tag, tag.Count)
		}
	default:
		fmt.Println("Error: gote tags doesn't support arg:", args[0])
	}
}

func tag(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: gote tag <note name> -t <tag1> <tag2> ... <tagN>")
		return
	}

	tFlag := -1
	for i, arg := range args {
		if arg == "-t" {
			tFlag = i
			break
		}
	}
	if tFlag == -1 || tFlag == 0 || tFlag == len(args)-1 {
		fmt.Println("Usage: gote tag <note name> -t <tag1> <tag2> ... <tagN>")
		return
	}

	noteName := strings.Join(args[:tFlag], " ")
	tagsToAdd := args[tFlag+1:]

	index := loadIndex()
	noteMeta, exists := index[noteName]
	notePath := ""
	if exists {
		notePath = noteMeta.FilePath
	}
	if notePath == "" {
		fmt.Println("Note path missing. Need to manually call gote index <note>.", noteName)
		return
	}

	data, err := os.ReadFile(notePath)
	if err != nil {
		fmt.Println("Error reading note:", err)
		return
	}
	lines := strings.SplitN(string(data), "\n", 2)
	firstLine := ""
	rest := ""
	if len(lines) > 0 {
		firstLine = lines[0]
	}
	if len(lines) > 1 {
		rest = lines[1]
	}

	existingTags := parseTags(firstLine)
	tagSet := make(map[string]struct{})
	for _, t := range existingTags {
		tagSet[t] = struct{}{}
	}
	added := false
	for _, t := range tagsToAdd {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, exists := tagSet[t]; !exists {
			existingTags = append(existingTags, t)
			tagSet[t] = struct{}{}
			added = true
		}
	}
	if !added {
		fmt.Println("No new tags to add.")
		return
	}

	// Write updated tags to first line, preserve rest of note
	newFirstLine := strings.Join(existingTags, ".")
	newContent := newFirstLine
	if rest != "" {
		newContent += "\n" + rest
	}
	err = os.WriteFile(notePath, []byte(newContent), 0644)
	if err != nil {
		fmt.Println("Error writing note:", err)
		return
	}

	// Update the note's index
	if err := indexNote(notePath); err != nil {
		fmt.Println("Tags updated, but failed to update index:", err)
		return
	}

	fmt.Println("Tags updated for note:", noteName)
}

func config(args []string) {
	if len(args) == 0 {
		printConfig()
		return
	}

	switch args[0] {
	case "edit":
		cfgPath := configPath()
		if err := openFileInEditor(cfgPath); err != nil {
			fmt.Println(err)
		}
	case "format":
		err := formatConfigFile()
		if err != nil {
			fmt.Println("Error formatting config:", err)
			return
		}
		fmt.Println("Config file formatted.")
	default:
		printConfig()
	}
}
