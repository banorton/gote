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
		tag()
	case "config":
		config(args[2:])
	default:
		note(args[1:])
	}
}

func note(args []string) {
	noteName := args[0]
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	noteDir := cfg.NoteDir
	if noteDir == "" {
		noteDir = defaultConfig().NoteDir
	}

	if err := os.MkdirAll(noteDir, 0755); err != nil {
		fmt.Println("Error creating notes directory:", err)
		return
	}

	notePath := filepath.Join(noteDir, noteName+".md")
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		f, err := os.Create(notePath)
		if err != nil {
			fmt.Println("Error creating note:", err)
			return
		}
		f.Close()
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
		indexFile := indexPath()
		var notes []NoteMeta
		if data, err := os.ReadFile(indexFile); err == nil {
			_ = json.Unmarshal(data, &notes)
		}
		var foundPath string
		for _, n := range notes {
			base := strings.TrimSuffix(filepath.Base(n.FilePath), ".md")
			if base == noteName {
				foundPath = n.FilePath
				break
			}
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
	indexFile := indexPath()
	var notes []NoteMeta
	if data, err := os.ReadFile(indexFile); err == nil {
		_ = json.Unmarshal(data, &notes)
	} else {
		fmt.Println("Could not read index file:", err)
		return
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
		var tags []TagMeta
		if err := json.Unmarshal(data, &tags); err != nil {
			fmt.Println("Could not parse tags file:", err)
			return
		}
		for _, tag := range tags {
			fmt.Printf("%s (%d)\n", tag.Tag, tag.Count)
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
		var tags []TagMeta
		if err := json.Unmarshal(data, &tags); err != nil {
			fmt.Println("Could not parse tags file:", err)
			return
		}
		sort.Slice(tags, func(i, j int) bool {
			return tags[i].Count > tags[j].Count
		})
		if n > len(tags) {
			n = len(tags)
		}
		fmt.Printf("Top %d tags by usage:\n", n)
		for i := 0; i < n; i++ {
			tag := tags[i]
			fmt.Printf("%s (%d)\n", tag.Tag, tag.Count)
		}
	default:
		fmt.Println("Error: gote tags doesn't support arg:", args[0])
	}
}

func tag() {

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
