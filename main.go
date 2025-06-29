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
	case "quick", "q":
		quick(args[2:])
	case "recent", "r":
		recent(args[2:])
	case "index", "idx":
		index(args[2:])
	case "tags", "ts":
		tags(args[2:])
	case "tag", "t":
		tag(args[2:])
	case "config", "c":
		config(args[2:])
	case "search", "s":
		search(args[2:])
	case "pin", "p":
		pin(args[2:])
	case "unpin", "u", "up":
		unpin(args[2:])
	case "pinned", "pd":
		pinned(args[2:])
	case "delete", "d", "del", "trash":
		del(args[2:])
	case "rename", "mv", "rn":
	case "help", "h":
	case "view", "v":
	case "info", "i":
	case "popular", "pop":
		popular()
	case "today":
	case "journal", "j":
	case "transfer":
	case "calendar", "cal":
	case "lint", "l":
	case "recover":
		recoverCmd(args[2:])
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

func search(args []string) {
	if len(args) > 0 && args[0] == "trash" {
		searchTrash(args[1:])
		return
	}

	if len(args) == 0 {
		fmt.Println("Usage: gote search <query> OR gote search -t <tag1> ... [-n <number>]")
		return
	}

	n := -1 // -1 means print all by default
	tagsMode := false
	tags := []string{}
	// Parse flags
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" {
			if n == -1 {
				n = 10
			} // if -n is present but no number, default to 10
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil && v > 0 {
					n = v
					i++ // skip next arg
				}
			}
		} else if args[i] == "-t" {
			tagsMode = true
			// All following args until -n or end are tags
			for j := i + 1; j < len(args) && args[j] != "-n"; j++ {
				tags = append(tags, args[j])
				i = j
			}
		}
	}

	if tagsMode {
		if len(tags) == 0 {
			fmt.Println("Usage: gote search -t <tag1> ... [-n <number>]")
			return
		}
		tagsFile := tagsPath()
		data, err := os.ReadFile(tagsFile)
		if err != nil {
			fmt.Println("Could not read tags file:", err)
			return
		}
		var tagsMap map[string]TagMeta
		if err := json.Unmarshal(data, &tagsMap); err != nil {
			fmt.Println("Could not parse tags file:", err)
			return
		}

		noteCount := make(map[string]int)
		for _, tag := range tags {
			tm, exists := tagsMap[tag]
			if !exists {
				continue
			}
			for _, note := range tm.Notes {
				noteCount[note]++
			}
		}

		type noteHit struct {
			NotePath string
			Count    int
		}
		var noteHits []noteHit
		for note, count := range noteCount {
			noteHits = append(noteHits, noteHit{NotePath: note, Count: count})
		}
		sort.Slice(noteHits, func(i, j int) bool {
			return noteHits[i].Count > noteHits[j].Count
		})

		if len(noteHits) == 0 {
			fmt.Println("No notes found for the given tags.")
			return
		}

		if n == -1 || n > len(noteHits) {
			n = len(noteHits)
		}
		fmt.Println("Notes matching the most tags (most hits first):")
		for i := 0; i < n; i++ {
			nh := noteHits[i]
			title := strings.TrimSuffix(filepath.Base(nh.NotePath), ".md")
			fmt.Printf("%s (matched %d tags)\n", title, nh.Count)
		}
		return
	}

	query := strings.ToLower(strings.Join(args, " "))
	index := loadIndex()
	var results []string
	for title := range index {
		if strings.Contains(strings.ToLower(title), query) {
			results = append(results, title)
		}
	}
	if len(results) == 0 {
		fmt.Println("No matching note titles found.")
		return
	}
	if n == -1 || n > len(results) {
		n = len(results)
	}
	for i := 0; i < n; i++ {
		fmt.Println(results[i])
	}
}

func pin(args []string) {
	if len(args) == 1 && args[0] == "format" {
		err := formatPinsFile()
		if err != nil {
			fmt.Println("Error formatting pins:", err)
			return
		}
		fmt.Println("Pins file formatted.")
		return
	}

	if len(args) == 0 {
		// Show all pinned notes
		pins, err := loadPins()
		if err != nil {
			fmt.Println("Error loading pins:", err)
			return
		}
		if len(pins) == 0 {
			fmt.Println("No pinned notes.")
			return
		}
		fmt.Println("Pinned notes:")
		for title := range pins {
			fmt.Println(title)
		}
		return
	}

	noteName := strings.Join(args, " ")
	index := loadIndex()
	_, exists := index[noteName]
	if !exists {
		fmt.Println("Note not found:", noteName)
		return
	}

	pins, err := loadPins()
	if err != nil {
		fmt.Println("Error loading pins:", err)
		return
	}
	if _, already := pins[noteName]; already {
		fmt.Println("Note already pinned:", noteName)
		return
	}
	pins[noteName] = emptyStruct{}
	if err := savePins(pins); err != nil {
		fmt.Println("Error saving pins:", err)
		return
	}
	fmt.Println("Pinned note:", noteName)
}

func pinned(args []string) {
	pins, err := loadPins()
	if err != nil {
		fmt.Println("Error loading pins:", err)
		return
	}
	if len(pins) == 0 {
		fmt.Println("No pinned notes.")
		return
	}
	fmt.Println("Pinned notes:")
	for title := range pins {
		fmt.Println(title)
	}
}

func unpin(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote unpin <note name>")
		return
	}
	noteName := strings.Join(args, " ")
	index := loadIndex()
	_, exists := index[noteName]
	if !exists {
		fmt.Println("Note not found:", noteName)
		return
	}

	pins, err := loadPins()
	if err != nil {
		fmt.Println("Error loading pins:", err)
		return
	}
	if _, pinned := pins[noteName]; !pinned {
		fmt.Println("Note was not pinned:", noteName)
		return
	}
	delete(pins, noteName)
	if err := savePins(pins); err != nil {
		fmt.Println("Error saving pins:", err)
		return
	}
	fmt.Println("Unpinned note:", noteName)
}

func del(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote delete <note name>")
		return
	}
	noteName := strings.Join(args, " ")
	if err := trashNote(noteName); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Note moved to trash:", noteName)
}

func recoverCmd(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote recover <note name>")
		return
	}
	noteName := strings.Join(args, " ")
	if err := recoverNote(noteName); err != nil {
		fmt.Println("Error recovering note:", err)
		return
	}
	fmt.Println("Note recovered:", noteName)
}
