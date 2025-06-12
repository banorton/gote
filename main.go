package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"gotes/note"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Config holds user configuration.
type Config struct {
	NotesDir string `json:"notesDir"`
}

func configFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gote", "config.json")
}

func loadConfig() (Config, error) {
	var cfg Config
	f, err := os.Open(configFilePath())
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	return cfg, json.NewDecoder(f).Decode(&cfg)
}

func saveConfig(cfg Config) error {
	dir := filepath.Dir(configFilePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(configFilePath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
}

func resolveNotesDir() string {
	cfg, err := loadConfig()
	if err == nil && cfg.NotesDir != "" {
		return cfg.NotesDir
	}
	home := os.Getenv("HOME")
	return filepath.Join(home, "gotes")
}

// Returns the full absolute path for a note, ensuring it is inside notesDir.
func resolveNotePath(notesDir, noteName string) (string, error) {
	noteName = filepath.Clean(noteName)
	if !strings.HasSuffix(noteName, ".md") {
		noteName += ".md"
	}
	fullPath := filepath.Join(notesDir, noteName)
	absNotesDir, _ := filepath.Abs(notesDir)
	absFullPath, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFullPath, absNotesDir+string(os.PathSeparator)) && absFullPath != absNotesDir {
		return "", fmt.Errorf("note path escapes notes directory")
	}
	return fullPath, nil
}

func main() {
	notesDir := resolveNotesDir()
	if len(os.Args) < 2 {
		fmt.Println("Usage: gote <command> [args]")
		os.Exit(1)
	}
	arg := os.Args[1]

	if arg == "tag" && len(os.Args) > 3 {
		noteArg := os.Args[2]
		tags := os.Args[3:]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		if _, err := os.Stat(notePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: note file does not exist: %s\n", notePath)
			os.Exit(1)
		}
		if err := note.SetTags(notePath, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting tags: %v\n", err)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		fmt.Printf("Tags for %s set to: %s\n", rel, strings.Join(tags, " . "))
		return
	}

	if arg == "tags" {
		tagCounts, err := note.TagsFrequency(notesDir)
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
		notes, err := note.IndexNotes(notesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Index error: %v\n", err)
			os.Exit(1)
		}
		if err := note.SaveIndex(notes); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save index: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Indexed notes:")
		for _, n := range notes {
			rel, _ := filepath.Rel(notesDir, n.Path)
			tags := strings.Join(n.Tags, ", ")
			fmt.Printf("- %s\n  Tags: %s\n  Last Modified: %s\n", rel, tags, time.Unix(n.LastModified, 0).Format("060102"))
		}
		return
	}

	if arg == "search" && len(os.Args) > 2 && os.Args[2] != "--tags" {
		query := strings.ToLower(os.Args[2])
		index, err := note.LoadIndex()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load index: %v\n", err)
			os.Exit(1)
		}
		var matches []note.NoteMetadata
		for _, n := range index {
			rel, _ := filepath.Rel(notesDir, n.Path)
			title := strings.TrimSuffix(rel, ".md")
			titleLower := strings.ToLower(title)
			if strings.Contains(titleLower, query) {
				matches = append(matches, n)
				continue
			}
			for _, word := range strings.FieldsFunc(titleLower, func(r rune) bool { return r == '/' || r == '_' || r == '-' || r == ' ' }) {
				if strings.Contains(word, query) {
					matches = append(matches, n)
					break
				}
			}
		}
		// Print relative paths in tabular format
		colWidth := 12
		cols := 6
		for i, n := range matches {
			rel, _ := filepath.Rel(notesDir, n.Path)
			title := strings.TrimSuffix(rel, ".md")
			if len(title) > 10 {
				title = title[:10]
			}
			fmt.Printf("%-*s", colWidth, title)
			if (i+1)%cols == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		return
	}

	if arg == "search" && len(os.Args) > 2 && os.Args[2] == "--tags" && len(os.Args) > 3 {
		tags := os.Args[3:]
		notes, err := note.IndexNotes(notesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Search error: %v\n", err)
			os.Exit(1)
		}
		matches := note.NotesWithAllTags(notes, tags)
		// Print relative paths
		colWidth := 12
		cols := 6
		for i, n := range matches {
			rel, _ := filepath.Rel(notesDir, n.Path)
			title := strings.TrimSuffix(rel, ".md")
			if len(title) > 10 {
				title = title[:10]
			}
			fmt.Printf("%-*s", colWidth, title)
			if (i+1)%cols == 0 {
				fmt.Println()
			}
		}
		fmt.Println()
		return
	}

	if arg == "today" {
		today := time.Now().Format("060102")
		dailyDir := filepath.Join(notesDir, "daily")
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
		notes, err := note.IndexNotes(notesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Recent error: %v\n", err)
			os.Exit(1)
		}
		sort.Slice(notes, func(i, j int) bool {
			return notes[i].LastModified > notes[j].LastModified
		})
		fmt.Println("Recent notes:")
		for _, n := range notes {
			rel, _ := filepath.Rel(notesDir, n.Path)
			title := strings.TrimSuffix(rel, ".md")
			if len(title) > 10 {
				title = title[:10]
			}
			fmt.Printf("%-12s %s\n", title, time.Unix(n.LastModified, 0).Format("060102"))
		}
		return
	}

	if arg == "delete" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Note does not exist: %s\n", noteArg)
			os.Exit(1)
		}
		fmt.Printf("Delete note '%s'? [y/N] ", noteArg)
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(resp)
		if resp != "y" && resp != "Y" {
			fmt.Println("Aborted.")
			return
		}
		if err := os.Remove(notePath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to delete note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Deleted note: %s\n", noteArg)
		return
	}

	if arg == "config" && len(os.Args) > 3 && os.Args[2] == "set-dir" {
		path, err := filepath.Abs(os.Args[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid path: %v\n", err)
			os.Exit(1)
		}
		cfg := Config{NotesDir: path}
		if err := saveConfig(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Notes directory set to: %s\n", path)
		return
	}

	if arg == "config" && len(os.Args) == 2 {
		cmd := exec.Command("vim", configFilePath())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		return
	}

	if arg == "pin" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Note does not exist: %s\n", noteArg)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		if err := note.PinNote(rel); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to pin note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Pinned: %s\n", rel)
		return
	}

	if arg == "unpin" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		if err := note.UnpinNote(rel); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unpin note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Unpinned: %s\n", rel)
		return
	}

	if arg == "pinned" {
		pins, err := note.ListPinned()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list pinned notes: %v\n", err)
			os.Exit(1)
		}
		if len(pins) == 0 {
			fmt.Println("No pinned notes.")
			return
		}
		fmt.Println("Pinned notes:")
		for _, rel := range pins {
			fmt.Println("-", rel)
		}
		return
	}

	if arg == "archive" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Note does not exist: %s\n", noteArg)
			os.Exit(1)
		}
		if err := note.ArchiveNote(notesDir, rel); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to archive note: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Archived: %s\n", rel)
		return
	}

	if arg == "view" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		data, err := os.ReadFile(notePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read note: %v\n", err)
			os.Exit(1)
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i == 0 {
				fmt.Printf("\033[36m%s\033[0m\n", line) // Cyan for tags
			} else if strings.HasPrefix(line, "# ") {
				fmt.Printf("\033[1;33m%s\033[0m\n", line) // Bold yellow for H1
			} else if strings.HasPrefix(line, "## ") {
				fmt.Printf("\033[1;32m%s\033[0m\n", line) // Bold green for H2
			} else if strings.HasPrefix(line, "### ") {
				fmt.Printf("\033[1;34m%s\033[0m\n", line) // Bold blue for H3
			} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "* ") {
				fmt.Printf("\033[35m%s\033[0m\n", line) // Magenta for lists
			} else if strings.HasPrefix(line, "```") {
				fmt.Printf("\033[1;37m%s\033[0m\n", line) // White for code block
			} else {
				fmt.Println(line)
			}
		}
		return
	}

	if arg == "lint" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		data, err := os.ReadFile(notePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read note: %v\n", err)
			os.Exit(1)
		}
		lines := strings.Split(string(data), "\n")
		ok := true
		if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
			fmt.Println("Lint: First line (tags) is empty.")
			ok = false
		}
		titleFound := false
		for _, line := range lines {
			if strings.HasPrefix(line, "# ") {
				titleFound = true
				break
			}
		}
		if !titleFound {
			fmt.Println("Lint: No H1 title (line starting with '# ') found.")
			ok = false
		}
		for i, line := range lines {
			if strings.HasPrefix(line, "# ") && i > 0 && strings.TrimSpace(lines[i-1]) != "" {
				fmt.Printf("Lint: Title (H1) at line %d should be preceded by a blank line.\n", i+1)
				ok = false
			}
		}
		if ok {
			fmt.Println("No lint issues found.")
		}
		return
	}

	// For now, treat any argument as a note name.
	noteArg := arg
	tags := os.Args[2:]
	if err := openOrCreateNote(notesDir, noteArg, tags); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(tags) > 0 {
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		if err := note.SetTags(notePath, tags); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting tags: %v\n", err)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		fmt.Printf("Tags for %s set to: %s\n", rel, strings.Join(tags, " . "))
	}
}

// openOrCreateNote ensures the note exists, creates it if needed, and opens it in Vim.
func openOrCreateNote(notesDir, noteName string, tags []string) error {
	fullPath, err := resolveNotePath(notesDir, noteName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create note directory: %w", err)
		}
		f, err := os.Create(fullPath)
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
		note.WriteCreatedFile(fullPath)
	}
	// Remove Vim swap file if it exists to avoid annoying warnings
	swapFile := fullPath + ".swp"
	if _, err := os.Stat(swapFile); err == nil {
		_ = os.Remove(swapFile)
	}
	cmd := exec.Command("vim", fullPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
