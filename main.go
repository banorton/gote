package main

import (
	"fmt"
	"gotes/note"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// defaultNotesDir is the directory where notes are stored.
var defaultNotesDir = filepath.Join(os.Getenv("HOME"), "gote-notes")

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gote <command> [args]")
		os.Exit(1)
	}
	arg := os.Args[1]

	if arg == "tag" && len(os.Args) > 3 {
		notePath := os.Args[2]
		tags := os.Args[3:]
		if !strings.HasSuffix(notePath, ".md") {
			notePath += ".md"
		}
		if _, err := os.Stat(notePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: note file does not exist: %s\n", notePath)
			os.Exit(1)
		}
		if err := note.SetTags(notePath, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting tags: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tags for %s set to: %s\n", notePath, strings.Join(tags, " . "))
		return
	}

	if arg == "tags" {
		tagCounts, err := note.TagsFrequency(defaultNotesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error indexing tags: %v\n", err)
			os.Exit(1)
		}
		var tags []string
		for tag := range tagCounts {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		for _, tag := range tags {
			fmt.Printf("%s (%d)\n", tag, tagCounts[tag])
		}
		return
	}

	if arg == "index" {
		notes, err := note.IndexNotes(defaultNotesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Index error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Indexed notes:")
		for _, n := range notes {
			tags := strings.Join(n.Tags, ", ")
			fmt.Printf("- %s\n  Tags: %s\n  Last Modified: %s\n", n.Path, tags, n.LastModified.Format("2006-01-02 15:04:05"))
		}
		return
	}

	if arg == "search" && len(os.Args) > 2 && os.Args[2] == "--tags" && len(os.Args) > 3 {
		tags := os.Args[3:]
		notes, err := note.IndexNotes(defaultNotesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		matches := note.NotesWithAllTags(notes, tags)
		note.PrintTabular(matches)
		return
	}

	if arg == "quick" && len(os.Args) > 2 {
		text := os.Args[2]
		quickDir := filepath.Join(defaultNotesDir, "quick")
		if err := os.MkdirAll(quickDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create quick dir: %v\n", err)
			os.Exit(1)
		}
		timestamp := time.Now().Format("2006-01-02T15:04:05")
		filename := timestamp + ".md"
		path := filepath.Join(quickDir, filename)
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create quick note: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Fprintf(f, ".quick\n\n# Quick Note\n\n%s\n", text)
		cmd := exec.Command("vim", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}

	if arg == "today" {
		today := time.Now().Format("2006-01-02")
		dailyDir := filepath.Join(defaultNotesDir, "daily")
		if err := os.MkdirAll(dailyDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create daily dir: %v\n", err)
			os.Exit(1)
		}
		filename := today + ".md"
		path := filepath.Join(dailyDir, filename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			f, err := os.Create(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create daily note: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()
			fmt.Fprintf(f, ".daily\n\n# %s\n", today)
		}
		cmd := exec.Command("vim", path)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}

	if arg == "recent" {
		notes, err := note.IndexNotes(defaultNotesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Recent error: %v\n", err)
			os.Exit(1)
		}
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].LastModified.After(notes[j].LastModified)
		})
		fmt.Println("Recent notes:")
		for _, n := range notes {
			title := filepath.Base(n.Path)
			title = strings.TrimSuffix(title, ".md")
			if len(title) > 10 {
				title = title[:10]
			}
			fmt.Printf("%-12s %s\n", title, n.LastModified.Format("2006-01-02 15:04:05"))
		}
		return
	}

	// For now, treat any argument as a note name.
	noteName := arg
	tags := os.Args[2:]
	if err := openOrCreateNote(noteName, tags); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(tags) > 0 {
		notePath := filepath.Join(defaultNotesDir, noteName)
		if !strings.HasSuffix(notePath, ".md") {
			notePath += ".md"
		}
		if err := note.SetTags(notePath, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting tags: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Tags for %s set to: %s\n", noteName, strings.Join(tags, " . "))
	}
}

// openOrCreateNote ensures the note exists, creates it if needed, and opens it in Vim.
func openOrCreateNote(noteName string, tags []string) error {
	// 1. Append .md if not present
	if !strings.HasSuffix(noteName, ".md") {
		noteName += ".md"
	}
	// 2. Resolve full path under notes dir
	notePath := filepath.Join(defaultNotesDir, noteName)

	// 3. If file doesn't exist, create it with template
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(notePath), 0755); err != nil {
			return fmt.Errorf("failed to create note directory: %w", err)
		}
		f, err := os.Create(notePath)
		if err != nil {
			return fmt.Errorf("failed to create note: %w", err)
		}
		defer f.Close()
		var tagsLine string
		if len(tags) > 0 {
			tagsLine = strings.Join(tags, " . ")
		} else {
			tagsLine = ""
		}
		title := strings.TrimSuffix(filepath.Base(noteName), ".md")
		_, err = fmt.Fprintf(f, "%s\n\n# %s\n", tagsLine, title)
		if err != nil {
			return fmt.Errorf("failed to write template: %w", err)
		}
	}

	// Remove Vim swap file if it exists to avoid annoying warnings
	swapFile := notePath + ".swp"
	if _, err := os.Stat(swapFile); err == nil {
		// Swap file exists, try to remove it
		_ = os.Remove(swapFile)
	}

	// 4. Open the note in vim, attaching to terminal IO
	cmd := exec.Command("vim", notePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
