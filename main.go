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
	"strconv"
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

func isReservedWord(arg string) bool {
	reserved := map[string]struct{}{
		"delete": {}, "d": {},
		"index": {}, "i": {},
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
		"popular": {}, "x": {},
		"move": {}, "mv": {}, "m": {},
		"help": {}, "h": {},
		"pinned": {},
		"tag":    {},
	}
	_, ok := reserved[arg]
	return ok
}

func main() {
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
		arg = "index"
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
	case "x":
		arg = "popular"
	case "m":
		arg = "move"
	case "mv":
		arg = "move"
	case "rn":
		arg = "rename"
	case "rename":
		// already set
	}

	// gote popular [N]
	if arg == "popular" {
		N := 10
		if len(os.Args) > 2 {
			n, err := strconv.Atoi(os.Args[2])
			if err == nil && n > 0 {
				N = n
			}
		}
		idx, err := note.LoadIndex()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load index: %v\n", err)
			os.Exit(1)
		}
		accessMap, _ := note.LoadAccessLog()
		notes := note.MergeAccessCounts(idx, accessMap)
		popular := note.PopularNotes(notes, N)
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
			rel := n.Name
			title := strings.TrimSuffix(filepath.Base(rel), ".md")
			fmt.Printf("%-20s | %-20s | %3d\n", title, bar, n.AccessCount)
		}
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
		// Check if index exists
		indexExists := false
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".gote", "index.json")); err == nil {
			indexExists = true
		}
		var notes []note.NoteMetadata
		var err error
		if indexExists {
			fmt.Print("An existing index was detected. Do you want to try to salvage information from it? [y/N] ")
			reader := bufio.NewReader(os.Stdin)
			resp, _ := reader.ReadString('\n')
			resp = strings.TrimSpace(resp)
			if resp == "y" || resp == "Y" {
				notes, err = note.RefreshIndex(notesDir, false)
			} else {
				notes, err = note.IndexNotes(notesDir)
			}
		} else {
			notes, err = note.IndexNotes(notesDir)
		}
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
			fmt.Printf("- %s\n  Tags: %s\n  Created: %s  Last Modified: %s\n", rel, tags, n.CreatedStr, n.ModifiedStr)
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
			fmt.Printf("%-12s %s\n", title, n.ModifiedStr)
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

	if arg == "links" && len(os.Args) > 2 {
		noteArg := os.Args[2]
		// Outbound links
		outbound, err := note.FindOutboundLinks(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding outbound links: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Outbound links from %s:\n", noteArg)
		if len(outbound) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, l := range outbound {
				fmt.Printf("  [[%s]]\n", l)
			}
		}
		// Inbound links
		inbound, err := note.FindInboundLinks(notesDir, noteArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error finding inbound links: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Inbound links to %s:\n", noteArg)
		if len(inbound) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, l := range inbound {
				fmt.Printf("  %s\n", l)
			}
		}
		return
	}

	if arg == "help" || arg == "h" {
		fmt.Println(`gote - minimal fast note-taking

Usage: gote <command|alias> [args]

Main features:
  - Create/open notes:         gote <note_name> [tags...]
  - Tagging:                  gote tag <note> [tags...]      (alias: tag)
  - Indexing:                 gote index                     (alias: i)
  - Search:                   gote search <query>            (alias: s)
  - Search by tags:           gote search --tags <tags...>
  - List tags:                gote tags                      (alias: t)
  - Recent notes:             gote recent                    (alias: r)
  - Pin/unpin:                gote pin <note>                (alias: p)
                              gote unpin <note>              (alias: u)
  - List pinned:              gote pinned
  - Archive:                  gote archive <note>            (alias: a)
  - View note:                gote view <note>               (alias: v)
  - Lint note:                gote lint <note>               (alias: l)
  - Config dir:               gote config set-dir <path>     (alias: c)
  - Edit config:              gote config
  - Daily note:               gote today                     (alias: n)
  - Popular notes:            gote popular [N]               (alias: x)
  - Note links:               gote links <note>              (alias: k)
  - Delete note:              gote delete <note>             (alias: d)

Short aliases:
  i  index      t  tags      s  search    r  recent
  p  pin        u  unpin     a  archive   v  view
  l  lint       c  config    n  today     k  links
  x  popular    d  delete    h  help

Other details:
- Notes are markdown (.md) files, can be in subdirectories.
- Tag line is always first, lowercased, delimited by ' . '.
- [[note name]] links to other notes. Use 'gote links <note>' to see inbound/outbound links.
- Creation and modification times are tracked (yymmdd.hhmmss).
- All metadata is stored in ~/.gote/.
- All commands print clear confirmation or error messages.
`)
		return
	}

	// gote move/mv <oldnote> <newnote>
	if (arg == "move") && len(os.Args) > 3 {
		oldArg := os.Args[2]
		newArg := os.Args[3]
		oldPath, err := resolveNotePath(notesDir, oldArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid source note path: %v\n", err)
			os.Exit(1)
		}
		newPath, err := resolveNotePath(notesDir, newArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid destination note path: %v\n", err)
			os.Exit(1)
		}
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Source note does not exist: %s\n", oldArg)
			os.Exit(1)
		}
		if _, err := os.Stat(newPath); err == nil {
			fmt.Fprintf(os.Stderr, "Destination note already exists: %s\n", newArg)
			os.Exit(1)
		}
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create destination directory: %v\n", err)
			os.Exit(1)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to move note: %v\n", err)
			os.Exit(1)
		}
		// Move .created file if present
		oldCreated := oldPath + ".created"
		newCreated := newPath + ".created"
		if _, err := os.Stat(oldCreated); err == nil {
			_ = os.Rename(oldCreated, newCreated)
		}
		fmt.Printf("Moved note: %s -> %s\n", oldArg, newArg)
		return
	}

	// gote rename/rn <oldname> <newname> (rename only, no move)
	if arg == "rename" && len(os.Args) > 3 {
		oldArg := os.Args[2]
		newArg := os.Args[3]
		oldPath, err := resolveNotePath(notesDir, oldArg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid source note path: %v\n", err)
			os.Exit(1)
		}
		oldDir := filepath.Dir(oldPath)
		newBase := filepath.Base(newArg)
		if !strings.HasSuffix(newBase, ".md") {
			newBase += ".md"
		}
		newPath := filepath.Join(oldDir, newBase)
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Source note does not exist: %s\n", oldArg)
			os.Exit(1)
		}
		if _, err := os.Stat(newPath); err == nil {
			fmt.Fprintf(os.Stderr, "Destination note already exists: %s\n", newBase)
			os.Exit(1)
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to rename note: %v\n", err)
			os.Exit(1)
		}
		// Move .created file if present
		oldCreated := oldPath + ".created"
		newCreated := newPath + ".created"
		if _, err := os.Stat(oldCreated); err == nil {
			_ = os.Rename(oldCreated, newCreated)
		}
		fmt.Printf("Renamed note: %s -> %s\n", filepath.Base(oldPath), newBase)
		return
	}

	// For now, treat any argument as a note name.
	if isReservedWord(arg) {
		fmt.Fprintf(os.Stderr, "'%s' is a reserved command or alias and cannot be used as a note name.\n", arg)
		os.Exit(1)
	}
	noteArg := arg
	tags := os.Args[2:]
	// Track access count for note open/create
	relPath := noteArg
	if !strings.HasSuffix(relPath, ".md") {
		relPath += ".md"
	}
	_ = note.IncrementAccess(relPath)
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
