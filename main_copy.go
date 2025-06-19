package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config holds user configuration.
type Config struct {
	NotesDir   string `json:"notesDir"`
	Editor     string `json:"editor,omitempty"`
	SafeDelete bool   `json:"safeDelete,omitempty"`
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
	// Check if the noteName is a reserved word first
	if isReservedWord(noteName) {
		return "", fmt.Errorf("cannot create note named '%s' as it is a reserved command name", noteName)
	}

	noteName = filepath.Clean(noteName)
	if !strings.HasSuffix(noteName, ".md") {
		noteName += ".md"
	}
	if filepath.IsAbs(noteName) {
		return "", fmt.Errorf("note name cannot be an absolute path")
	}
	return filepath.Join(notesDir, noteName), nil
}

func isReservedWord(arg string) bool {
	reserved := map[string]struct{}{
		"delete": {}, "d": {},
		"index": {}, "x": {},
		"tags": {}, "t": {},
		"search": {}, "s": {},
		"recent": {}, "r": {},
		"pin": {}, "p": {},
		"unpin": {}, "u": {},
		"archive": {}, "a": {},
		"view": {}, "v": {},
		"lint": {}, "l": {},
		"config": {}, "c": {},
		"today": {}, "n": {},
		"links": {}, "k": {},
		"popular": {}, "z": {},
		"move": {}, "mv": {}, "m": {},
		"help": {}, "h": {},
		"pinned": {},
		"tag":    {},
		"info":   {}, "i": {},
		"trash":   {},
		"recover": {},
	}
	_, ok := reserved[arg]
	return ok
}

// getEditor returns the configured editor, $EDITOR, or 'vim' as fallback
func getEditor() string {
	cfg, _ := loadConfig()
	if cfg.Editor != "" {
		return cfg.Editor
	}
	if ed := os.Getenv("EDITOR"); ed != "" {
		return ed
	}
	return "vim"
}

func parseNoteAndTags(args []string) (string, []string) {
	noteParts := []string{}
	tags := []string{}
	tagMode := false
	for _, arg := range args {
		if arg == "-t" || arg == "--tags" {
			tagMode = true
			continue
		}
		if tagMode {
			tags = append(tags, arg)
		} else {
			noteParts = append(noteParts, arg)
		}
	}
	noteName := strings.Join(noteParts, " ")
	return strings.TrimSpace(noteName), tags
}

// Handler for 'search' command (simple version with title truncation and date)
func handleSearch(notesDir string, args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: gote search <query>")
		return
	}
	query := strings.ToLower(args[1])
	index, err := LoadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load index: %v\n", err)
		os.Exit(1)
	}
	var matches []NoteMetadata
	for _, n := range index {
		title := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		titleLower := strings.ToLower(title)
		if strings.Contains(titleLower, query) {
			matches = append(matches, n)
		}
	}
	if len(matches) == 0 {
		fmt.Println("No notes found.")
		return
	}
	titleWidth := 20
	for _, n := range matches {
		title := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		if len(title) > titleWidth {
			title = title[:titleWidth]
		}
		fmt.Printf("%-*s %s\n", titleWidth, title, n.ModifiedStr)
	}
}

// Handler for 'recent' command
func handleRecent(notesDir string, args []string) {
	notes, err := IndexNotes(notesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Recent error: %v\n", err)
		os.Exit(1)
	}
	sort.Slice(notes, func(i, j int) bool {
		return notes[i].LastModified > notes[j].LastModified
	})
	N := len(notes)
	if len(args) > 1 {
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 && n < N {
			N = n
		}
	}
	fmt.Println("Recent notes:")
	titleWidth := 20
	dateWidth := 14
	cellWidth := titleWidth + 1 + dateWidth + 2
	termWidth := getTerminalWidth()
	cols := termWidth / cellWidth
	if cols < 1 {
		cols = 1
	}
	for i, n := range notes[:N] {
		title := strings.TrimSuffix(filepath.Base(n.Path), ".md")
		if len(title) > titleWidth {
			title = title[:titleWidth]
		}
		if i%cols == 0 {
			fmt.Print("|")
		}
		fmt.Printf(" %-*s %-*s |", titleWidth, title, dateWidth, n.ModifiedStr)
		if (i+1)%cols == 0 {
			fmt.Println()
		}
	}
	if N%cols != 0 {
		fmt.Println()
	}
	return
}

// Handler for 'popular' command
func handlePopular(notesDir string, args []string) {
	N := 10
	if len(args) > 1 {
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 {
			N = n
		}
	}
	idx, err := LoadIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load index: %v\n", err)
		os.Exit(1)
	}
	accessMap, _ := LoadAccessLog()
	notes := MergeAccessCounts(idx, accessMap)
	popular := PopularNotes(notes, N)
	maxAccess := 1
	for _, n := range popular {
		if n.AccessCount > maxAccess {
			maxAccess = n.AccessCount
		}
	}
	fmt.Println("Popular notes:")
	for _, n := range popular {
		barLen := 0
		if maxAccess > 0 {
			barLen = n.AccessCount * 20 / maxAccess
		}
		bar := strings.Repeat("█", barLen)
		title := strings.TrimSuffix(filepath.Base(n.Name), ".md")
		if len(title) > 20 {
			title = title[:20]
		}
		fmt.Printf("%-20s | %-20s\n", title, bar)
	}
	return
}

func main() {
	// Quick note behavior: no args
	if len(os.Args) == 1 {
		notesDir := resolveNotesDir()
		quickPath := filepath.Join(notesDir, "quick.md")
		if _, err := os.Stat(quickPath); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(quickPath), 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create note directory: %v\n", err)
				os.Exit(1)
			}
			f, err := os.Create(quickPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create quick note: %v\n", err)
				os.Exit(1)
			}
			defer f.Close()
			_, err = fmt.Fprintf(f, ".quick\n\n# Quick\n")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write quick note template: %v\n", err)
				os.Exit(1)
			}
		}
		// Open in editor, position cursor consistently with other notes
		editor := getEditor()
		var cmd *exec.Cmd
		if editor == "vim" || editor == "nvim" {
			// Position cursor at start of title
			cmd = exec.Command(editor, "+normal gg/^# \\<CR>n2l", quickPath)
		} else {
			cmd = exec.Command(editor, quickPath)
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		os.Exit(0)
	}

	notesDir := resolveNotesDir()
	if len(os.Args) < 2 {
		fmt.Println("Usage: gote <command|alias> [args]")
		os.Exit(1)
	}
	arg := os.Args[1]

	// Aliases for commands
	switch arg {
	case "d":
		arg = "delete"
	case "i":
		arg = "info"
	case "t":
		arg = "tags"
	case "s":
		arg = "search"
	case "r":
		arg = "recent"
	case "p":
		arg = "pin"
	case "u":
		arg = "unpin"
	case "a":
		arg = "archive"
	case "v":
		arg = "view"
	case "l":
		arg = "lint"
	case "c":
		arg = "config"
	case "n":
		arg = "today"
	case "k":
		arg = "links"
	case "z":
		arg = "popular"
	case "m":
		arg = "move"
	case "mv":
		arg = "move"
	case "rn":
		arg = "rename"
	case "rename":
		// already set
	case "ii":
		arg = "info"
	}

	// --- INFO COMMAND HANDLER ---
	if arg == "info" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
			os.Exit(1)
		}
		index, err := LoadIndex()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load index: %v\n", err)
			os.Exit(1)
		}
		var found *NoteMetadata
		for _, n := range index {
			if n.Path == notePath {
				found = &n
				break
			}
		}
		if found == nil {
			fmt.Fprintf(os.Stderr, "Note not found in index: %s\n", noteArg)
			os.Exit(1)
		}
		fmt.Printf("Note: %s\n", noteArg)
		fmt.Printf("  Path: %s\n", found.Path)
		fmt.Printf("  Tags: %s\n", strings.Join(found.Tags, ", "))
		fmt.Printf("  Created: %s\n", found.CreatedStr)
		fmt.Printf("  Last Modified: %s\n", found.ModifiedStr)
		fmt.Printf("  Access Count: %d\n", found.AccessCount)
		fmt.Printf("  Word Count: %d\n", found.WordCount)
		fmt.Printf("  Char Count: %d\n", found.CharCount)
		return
	}

	// gote popular [N]
	if arg == "popular" {
		handlePopular(notesDir, os.Args[1:])
		return
	}

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
		// Load current tags
		currentTags, _ := ParseTagsFromFile(notePath)
		tagSet := make(map[string]struct{})
		for _, t := range currentTags {
			tagSet[t] = struct{}{}
		}
		added := false
		for _, t := range tags {
			t = strings.ToLower(strings.TrimSpace(t))
			if t == "" {
				continue
			}
			if _, exists := tagSet[t]; !exists {
				currentTags = append(currentTags, t)
				tagSet[t] = struct{}{}
				added = true
			}
		}
		if !added {
			fmt.Printf("No new tags to add for %s\n", filepath.Base(notePath))
			return
		}
		if err := SetTags(notePath, currentTags); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting tags: %v\n", err)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		fmt.Printf("Tags for %s set to: %s\n", rel, strings.Join(currentTags, " . "))
		return
	}

	if arg == "tags" {
		sortByPopular := false
		if len(os.Args) > 2 && os.Args[2] == "--sort" && len(os.Args) > 3 && os.Args[3] == "popular" {
			sortByPopular = true
		}
		tagCounts, err := TagsFrequency(notesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error indexing tags: %v\n", err)
			os.Exit(1)
		}
		var tags []string
		for tag := range tagCounts {
			tags = append(tags, tag)
		}
		if sortByPopular {
			sort.Slice(tags, func(i, j int) bool {
				if tagCounts[tags[i]] == tagCounts[tags[j]] {
					return tags[i] < tags[j]
				}
				return tagCounts[tags[i]] > tagCounts[tags[j]]
			})
			fmt.Println("Tags by popularity:")
		} else {
			sort.Strings(tags)
			fmt.Println("Tags (alphabetical):")
		}
		for _, tag := range tags {
			fmt.Printf("%s (%d)\n", tag, tagCounts[tag])
		}
		return
	}

	if arg == "index" {
		if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
			noteArg := os.Args[2]
			// notePath is not needed, just validate
			if _, err := resolveNotePath(notesDir, noteArg); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid note path: %v\n", err)
				os.Exit(1)
			}
			if err := UpdateNoteInIndex(notesDir, noteArg, false); err != nil {
				fmt.Fprintf(os.Stderr, "Index error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Index updated for note: %s\n", noteArg)
			return
		}
		notes, err := IndexNotes(notesDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Index error: %v\n", err)
			os.Exit(1)
		}
		if err := SaveIndex(notes); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save index: %v\n", err)
			os.Exit(1)
		}
		// Only print indexed notes if --verbose or -v is present
		verbose := false
		for _, a := range os.Args[2:] {
			if a == "--verbose" || a == "-v" {
				verbose = true
				break
			}
		}
		if verbose {
			fmt.Println("Indexed notes:")
			for _, n := range notes {
				rel, _ := filepath.Rel(notesDir, n.Path)
				tags := strings.Join(n.Tags, ", ")
				fmt.Printf("- %s\n  Tags: %s\n  Created: %s  Last Modified: %s\n", rel, tags, n.CreatedStr, n.ModifiedStr)
			}
		}
		return
	}

	// gote search -d <dirname> or --dir <dirname>
	if arg == "search" {
		handleSearch(notesDir, os.Args[1:])
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
		handleRecent(notesDir, os.Args[1:])
		return
	}

	if arg == "delete" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		cfg, _ := loadConfig()
		if cfg.SafeDelete {
			fmt.Print("delete? [y/n] ")
			reader := bufio.NewReader(os.Stdin)
			resp, _ := reader.ReadString('\n')
			resp = strings.TrimSpace(resp)
			if resp != "y" {
				fmt.Println("aborted", noteArg)
				return
			}
		}
		home, _ := os.UserHomeDir()
		trashDir := filepath.Join(home, ".gote", "trash")
		if err := os.MkdirAll(trashDir, 0755); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		trashPath := filepath.Join(trashDir, filepath.Base(notePath))
		if err := os.Rename(notePath, trashPath); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		fmt.Println("deleted", noteArg)
		return
	}

	if arg == "config" && len(os.Args) > 3 && os.Args[2] == "set-dir" {
		path, err := filepath.Abs(os.Args[3])
		if err != nil {
			fmt.Println("error", os.Args[3])
			os.Exit(1)
		}
		cfg := Config{NotesDir: path}
		if err := saveConfig(cfg); err != nil {
			fmt.Println("error", path)
			os.Exit(1)
		}
		fmt.Println("ok", path)
		return
	}

	if arg == "config" && len(os.Args) > 3 && os.Args[2] == "set-editor" {
		editor := os.Args[3]
		cfg, _ := loadConfig()
		cfg.Editor = editor
		if err := saveConfig(cfg); err != nil {
			fmt.Println("error", editor)
			os.Exit(1)
		}
		fmt.Println("ok", editor)
		return
	}

	if arg == "config" && len(os.Args) > 2 && os.Args[2] == "safe-delete" {
		cfg, _ := loadConfig()
		if len(os.Args) > 3 {
			val := strings.ToLower(os.Args[3])
			if val == "on" || val == "true" {
				cfg.SafeDelete = true
			} else if val == "off" || val == "false" {
				cfg.SafeDelete = false
			} else {
				fmt.Println("usage: gote config safe-delete [on|off]")
				os.Exit(1)
			}
			if err := saveConfig(cfg); err != nil {
				fmt.Println("error", os.Args[3])
				os.Exit(1)
			}
			fmt.Println("ok", os.Args[3])
			return
		}
		if cfg.SafeDelete {
			fmt.Println("on")
		} else {
			fmt.Println("off")
		}
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
		if err != nil || os.IsNotExist(err) {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		if err := PinNote(rel); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		fmt.Println("pinned", noteArg)
		return
	}

	if arg == "unpin" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		if err := UnpinNote(rel); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		fmt.Println("unpinned", noteArg)
		return
	}

	if arg == "archive" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		notePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		rel, _ := filepath.Rel(notesDir, notePath)
		if _, err := os.Stat(notePath); os.IsNotExist(err) {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if err := ArchiveNote(notesDir, rel); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		fmt.Println("archived", noteArg)
		return
	}

	if arg == "recover" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		if !strings.HasSuffix(noteArg, ".md") {
			noteArg += ".md"
		}
		home, _ := os.UserHomeDir()
		trashDir := filepath.Join(home, ".gote", "trash")
		trashPath := filepath.Join(trashDir, noteArg)
		if _, err := os.Stat(trashPath); os.IsNotExist(err) {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		notesDir := resolveNotesDir()
		restorePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if _, err := os.Stat(restorePath); err == nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(restorePath), 0755); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if err := os.Rename(trashPath, restorePath); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		fmt.Println("recovered", noteArg)
		return
	}

	if arg == "move" && len(os.Args) > 3 {
		oldArg := os.Args[2]
		newArg := os.Args[3]
		oldPath, err := resolveNotePath(notesDir, oldArg)
		if err != nil {
			fmt.Println("error", oldArg)
			os.Exit(1)
		}
		newPath, err := resolveNotePath(notesDir, newArg)
		if err != nil {
			fmt.Println("error", newArg)
			os.Exit(1)
		}
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Println("error", oldArg)
			os.Exit(1)
		}
		if _, err := os.Stat(newPath); err == nil {
			fmt.Println("error", newArg)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			fmt.Println("error", newArg)
			os.Exit(1)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Println("error", oldArg)
			os.Exit(1)
		}
		oldCreated := oldPath + ".created"
		newCreated := newPath + ".created"
		if _, err := os.Stat(oldCreated); err == nil {
			_ = os.Rename(oldCreated, newCreated)
		}
		fmt.Println("moved", oldArg, newArg)
		return
	}

	if arg == "rename" && len(os.Args) > 3 {
		oldArg := os.Args[2]
		newArg := os.Args[3]
		oldPath, err := resolveNotePath(notesDir, oldArg)
		if err != nil {
			fmt.Println("error", oldArg)
			os.Exit(1)
		}
		oldDir := filepath.Dir(oldPath)
		newBase := filepath.Base(newArg)
		if !strings.HasSuffix(newBase, ".md") {
			newBase += ".md"
		}
		newPath := filepath.Join(oldDir, newBase)
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Println("error", oldArg)
			os.Exit(1)
		}
		if _, err := os.Stat(newPath); err == nil {
			fmt.Println("error", newBase)
			os.Exit(1)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Println("error", oldArg)
			os.Exit(1)
		}
		oldCreated := oldPath + ".created"
		newCreated := newPath + ".created"
		if _, err := os.Stat(oldCreated); err == nil {
			_ = os.Rename(oldCreated, newCreated)
		}
		fmt.Println("renamed", oldArg, newBase)
		return
	}

	if arg == "trash" {
		home, _ := os.UserHomeDir()
		trashDir := filepath.Join(home, ".gote", "trash")
		entries, err := os.ReadDir(trashDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to read trash: %v\n", err)
			os.Exit(1)
		}
		type trashEntry struct {
			Name    string
			ModTime time.Time
		}
		var files []trashEntry
		for _, entry := range entries {
			if !entry.IsDir() {
				info, err := entry.Info()
				if err == nil {
					files = append(files, trashEntry{entry.Name(), info.ModTime()})
				}
			}
		}
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.After(files[j].ModTime)
		})
		if len(files) == 0 {
			fmt.Println("Trash is empty.")
			return
		}
		fmt.Println("Notes in trash (most recent first):")
		for _, f := range files {
			fmt.Printf("- %s (deleted: %s)\n", f.Name, f.ModTime.Format("2006-01-02 15:04:05"))
		}
		return
	}

	if arg == "recover" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		if !strings.HasSuffix(noteArg, ".md") {
			noteArg += ".md"
		}
		home, _ := os.UserHomeDir()
		trashDir := filepath.Join(home, ".gote", "trash")
		trashPath := filepath.Join(trashDir, noteArg)
		if _, err := os.Stat(trashPath); os.IsNotExist(err) {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		notesDir := resolveNotesDir()
		restorePath, err := resolveNotePath(notesDir, noteArg)
		if err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if _, err := os.Stat(restorePath); err == nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(restorePath), 0755); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		if err := os.Rename(trashPath, restorePath); err != nil {
			fmt.Println("error", noteArg)
			os.Exit(1)
		}
		fmt.Println("recovered", noteArg)
		return
	}

	if arg == "pack" {
		notesDir := resolveNotesDir()
		home, _ := os.UserHomeDir()
		packDir := filepath.Join(home, ".gote")
		zipPath := filepath.Join(packDir, "notes_pack.zip")
		if err := os.MkdirAll(packDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create pack dir: %v\n", err)
			os.Exit(1)
		}
		zipCmd := exec.Command("zip", "-r", zipPath, ".", "--include", "*.md", "index.json", "access.json", "pinned.json")
		zipCmd.Dir = notesDir
		zipCmd.Stdout = os.Stdout
		zipCmd.Stderr = os.Stderr
		if err := zipCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create zip: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Packed notes and metadata to %s\n", zipPath)
		return
	}

	if arg == "unpack" && len(os.Args) > 3 {
		zipFile := os.Args[2]
		destDir := os.Args[3]
		if err := os.MkdirAll(destDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create destination dir: %v\n", err)
			os.Exit(1)
		}
		unzipCmd := exec.Command("unzip", zipFile, "-d", destDir)
		unzipCmd.Stdout = os.Stdout
		unzipCmd.Stderr = os.Stderr
		if err := unzipCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to unzip: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Unpacked notes and metadata to %s\n", destDir)
		return
	}

	// --- VIEW COMMAND HANDLER ---
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
		fmt.Print(string(data))
		return
	}

	// IMPORTANT: All command handlers must be placed before this reserved word check!
	// Only check for reserved word if the user is trying to create/open a note (not running a command)
	// That is, if the arg is not a known command/alias, treat it as a note name and check
	knownCommands := map[string]struct{}{
		"delete": {}, "d": {}, "index": {}, "x": {}, "tags": {}, "t": {}, "search": {}, "s": {}, "recent": {}, "r": {}, "pin": {}, "p": {}, "unpin": {}, "u": {}, "archive": {}, "a": {}, "view": {}, "v": {}, "lint": {}, "l": {}, "config": {}, "c": {}, "today": {}, "n": {}, "links": {}, "k": {}, "popular": {}, "z": {}, "move": {}, "mv": {}, "m": {}, "help": {}, "h": {}, "pinned": {}, "tag": {}, "info": {}, "i": {}, "trash": {}, "recover": {},
	}
	if _, isCmd := knownCommands[arg]; !isCmd {
		if isReservedWord(arg) {
			fmt.Fprintf(os.Stderr, "'%s' is a reserved command or alias and cannot be used as a note name.\n", arg)
			os.Exit(1)
		}
	}
	// Support note names with spaces and tags via -t/--tags
	noteName, tags := parseNoteAndTags(os.Args[1:])
	if noteName == "" {
		fmt.Println("Usage: gote <note name> [-t|--tags <tags...>]")
		os.Exit(1)
	}
	// Track access count for note open/create
	relPath := noteName
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	_ = IncrementAccess(relPath)
	if err := openOrCreateNote(notesDir, noteName, tags); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	UpdateNoteInIndex(notesDir, noteName, false)
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
		WriteCreatedFile(fullPath)
	}
	// Remove Vim swap file if it exists to avoid annoying warnings
	swapFile := fullPath + ".swp"
	if _, err := os.Stat(swapFile); err == nil {
		_ = os.Remove(swapFile)
	}
	editor := getEditor()

	// For Vim/Neovim, position cursor at start of title (after "# ")
	var cmd *exec.Cmd
	if editor == "vim" || editor == "nvim" {
		// +normal commands:
		// gg - go to start of file
		// /^#<space> - search for "# " at start of line
		// n - go to first match
		// 2l - move 2 characters right (after "# ")
		cmd = exec.Command(editor, "+normal gg/^# \\<CR>n2l", fullPath)
	} else {
		cmd = exec.Command(editor, fullPath)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getTerminalWidth returns the width of the terminal in columns, or 80 if unknown
func getTerminalWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if n, err := strconv.Atoi(cols); err == nil && n > 0 {
			return n
		}
	}
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err == nil {
		parts := strings.Fields(string(out))
		if len(parts) == 2 {
			if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
				return n
			}
		}
	}
	return 80 // default fallback
}
