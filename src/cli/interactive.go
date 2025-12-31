package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

// selectKeys are the keys used for selecting items in paginated lists
// Order: homerow → top row → bottom row, then shift versions of each
var selectKeys = []rune{
	// Lowercase
	'a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l', ';', // homerow (10)
	'w', 'e', 'r', 't', 'y', 'u', 'i', 'o',           // top row (8, avoiding q, p)
	'z', 'x', 'c', 'v', 'b', 'm',                     // bottom row (6, avoiding n)
	// Shift (uppercase)
	'A', 'S', 'D', 'F', 'G', 'H', 'J', 'K', 'L', ':', // shift+homerow (10)
	'W', 'E', 'R', 'T', 'Y', 'U', 'I', 'O',           // shift+top row (8)
	'Z', 'X', 'C', 'V', 'B', 'M',                     // shift+bottom row (6)
}

const maxSelectablePageSize = 48 // len(selectKeys)

// looksLikeDate returns true if the string looks like a date input
// (digits only, optionally with a single . for time separator)
func looksLikeDate(s string) bool {
	if s == "" {
		return false
	}
	dotCount := 0
	for _, c := range s {
		if c == '.' {
			dotCount++
			if dotCount > 1 {
				return false
			}
		} else if c < '0' || c > '9' {
			return false
		}
	}
	// Valid lengths: 2, 4, 6 (no dot) or 9, 11, 13 (with dot at position 6)
	l := len(s)
	if dotCount == 0 {
		return l == 2 || l == 4 || l == 6
	}
	return (l == 9 || l == 11 || l == 13) && len(s) > 6 && s[6] == '.'
}

func displayPaginatedResults(results []string, selectable bool, pageSize int, onSelect func(string)) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

	if pageSize <= 0 {
		pageSize = 10
	}
	if selectable && pageSize > maxSelectablePageSize {
		pageSize = maxSelectablePageSize
	}

	page := 0
	totalPages := (len(results) + pageSize - 1) / pageSize

	// Single page, no navigation needed - but still allow [o]pen
	if totalPages == 1 && !selectable {
		if cfg.FancyUI {
			ui.Box("Results", results, 0)
			fmt.Printf("\n %s[o] open  [q] quit%s\n", Dim, Reset)
		} else {
			for _, r := range results {
				fmt.Println(r)
			}
			fmt.Println("[o] open [q] quit")
			fmt.Print(": ")
		}
		key, err := ReadKey(cfg.FancyUI)
		if err != nil {
			return
		}
		if key == 'o' || key == 'O' {
			selectable = true
			// Fall through to the main loop
		} else {
			if cfg.FancyUI {
				ui.Clear()
			}
			return
		}
	}

	for {
		start := page * pageSize
		end := min(start+pageSize, len(results))

		if start >= end {
			break
		}

		pageItems := results[start:end]
		var keys []rune
		if selectable {
			for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
				keys = append(keys, selectKeys[i])
			}
		}

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList("Results", pageItems, -1, keys)
			ui.NavHintWithOpen(page+1, totalPages, !selectable)
		} else {
			if page > 0 {
				fmt.Println()
			}
			for i, item := range pageItems {
				if selectable && i < len(selectKeys) {
					fmt.Printf("[%c] %s\n", selectKeys[i], item)
				} else {
					fmt.Println(item)
				}
			}
			fmt.Printf("(%d/%d)────────────────────────\n", page+1, totalPages)
			if totalPages > 1 {
				fmt.Print("[n] next [p] prev ")
			}
			if !selectable {
				fmt.Print("[o] open ")
			}
			fmt.Println("[q] quit")
			fmt.Print(": ")
		}

		// Single page non-selectable: handled above, but keep for safety
		if totalPages == 1 && !selectable {
			break
		}

		key, err := ReadKey(cfg.FancyUI)
		if err != nil {
			break
		}

		switch key {
		case 'q', 'Q':
			if cfg.FancyUI {
				ui.Clear()
			}
			return
		case 'n', 'N':
			if page < totalPages-1 {
				page++
			}
			continue
		case 'p', 'P':
			if page > 0 {
				page--
			}
			continue
		case 'o', 'O':
			if !selectable {
				selectable = true
				if !cfg.FancyUI {
					fmt.Println() // Spacing before open mode
				}
				continue
			}
		}

		if selectable {
			for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
				if key == selectKeys[i] {
					if cfg.FancyUI {
						ui.Clear()
					}
					onSelect(results[start+i])
					return
				}
			}
		}
	}
}

func displayPaginatedSearchResultsWithMode(results []core.SearchResult, selectable bool, deleteMode bool, pinMode bool, pageSize int) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

	if pageSize <= 0 {
		pageSize = 10
	}
	if selectable && pageSize > maxSelectablePageSize {
		pageSize = maxSelectablePageSize
	}

	// Track if we're in "open mode" (not delete or pin)
	openMode := selectable && !deleteMode && !pinMode

	page := 0
	totalPages := (len(results) + pageSize - 1) / pageSize

	for {
		start := page * pageSize
		end := min(start+pageSize, len(results))

		if start >= end {
			break
		}

		pageResults := results[start:end]

		// Build display items
		var items []string
		var keys []rune
		for i, result := range pageResults {
			item := result.Title
			if result.Score > 1 {
				item = fmt.Sprintf("%s (matched %d tags)", result.Title, result.Score)
			}
			items = append(items, item)
			if selectable && i < len(selectKeys) {
				keys = append(keys, selectKeys[i])
			}
		}

		title := "Search Results"
		if deleteMode {
			title = "Search Results (delete mode)"
		} else if pinMode {
			title = "Search Results (pin mode)"
		}

		// Show [o]pen option if not already in a mode
		showOpen := !selectable && !deleteMode && !pinMode

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList(title, items, -1, keys)
			ui.NavHintWithOpen(page+1, totalPages, showOpen)
		} else {
			if page > 0 {
				fmt.Println()
			}
			for i, item := range items {
				if selectable && i < len(selectKeys) {
					fmt.Printf("[%c] %s\n", selectKeys[i], item)
				} else {
					fmt.Println(item)
				}
			}
			fmt.Printf("(%d/%d)────────────────────────\n", page+1, totalPages)
			if totalPages > 1 {
				fmt.Print("[n] next [p] prev ")
			}
			if showOpen {
				fmt.Print("[o] open ")
			}
			fmt.Println("[q] quit")
			fmt.Print(": ")
		}

		key, err := ReadKey(cfg.FancyUI)
		if err != nil {
			break
		}

		switch key {
		case 'q', 'Q':
			if cfg.FancyUI {
				ui.Clear()
			}
			return
		case 'n', 'N':
			if page < totalPages-1 {
				page++
			}
			continue
		case 'p', 'P':
			if page > 0 {
				page--
			}
			continue
		case 'o', 'O':
			if !selectable && !deleteMode && !pinMode {
				selectable = true
				openMode = true
				if !cfg.FancyUI {
					fmt.Println() // Spacing before open mode
				}
				continue
			}
		}

		if selectable {
			for i := 0; i < len(pageResults) && i < len(selectKeys); i++ {
				if key == selectKeys[i] {
					if cfg.FancyUI {
						ui.Clear()
					}
					if deleteMode {
						if err := core.DeleteNote(results[start+i].Title); err != nil {
							ui.Error(err.Error())
							return
						}
						ui.Success("Note moved to trash: " + results[start+i].Title)
					} else if pinMode {
						if err := core.PinNote(results[start+i].Title); err != nil {
							ui.Error(err.Error())
							return
						}
						ui.Success("Pinned: " + results[start+i].Title)
					} else {
						core.OpenAndReindexNote(results[start+i].FilePath, results[start+i].Title)
					}
					return
				}
			}
		}
	}
	// Suppress unused variable warning - openMode tracks state for the else branch in selection
	_ = openMode
}

func RecentCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool) {
	args := ParseArgs(rawArgs)
	openMode := defaultOpen
	deleteMode := defaultDelete
	pinMode := defaultPin

	// Check for mode keywords as first positional arg (e.g., "gote recent open")
	first := args.First()
	if first == "open" {
		openMode = true
		args.Positional = args.Positional[1:]
	} else if first == "delete" {
		deleteMode = true
		args.Positional = args.Positional[1:]
	} else if first == "pin" {
		pinMode = true
		args.Positional = args.Positional[1:]
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")

	// Support bare number as first positional arg (e.g., "gote r 5")
	if pageSize == cfg.PageSize() && args.First() != "" {
		if v, err := strconv.Atoi(args.First()); err == nil && v > 0 {
			pageSize = v
		}
	}

	notes, err := core.GetRecentNotes(-1) // Get all
	if err != nil {
		fmt.Println("Error getting recent notes:", err)
		return
	}

	var titles []string
	for _, note := range notes {
		titles = append(titles, note.Title)
	}

	ui := NewUI(cfg.FancyUI)

	if deleteMode {
		displayPaginatedResults(titles, true, pageSize, func(title string) {
			if err := core.DeleteNote(title); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Note moved to trash: " + title)
		})
		return
	}

	if pinMode {
		displayPaginatedResults(titles, true, pageSize, func(title string) {
			if err := core.PinNote(title); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Pinned: " + title)
		})
		return
	}

	displayPaginatedResults(titles, openMode, pageSize, func(title string) {
		index, err := data.LoadIndex()
		if err != nil {
			ui.Error("Error loading index: " + err.Error())
			return
		}
		if meta, exists := index[title]; exists {
			core.OpenAndReindexNote(meta.FilePath, title)
		}
	})
}

func SearchCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool) {
	args := ParseArgs(rawArgs)

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

	// Handle "search trash <query>" subcommand
	if args.First() == "trash" {
		query := strings.ToLower(strings.Join(args.Rest(), " "))
		if query == "" {
			fmt.Println("Usage: gote search trash <query>")
			return
		}
		results, err := core.SearchTrash(query)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No matching trashed notes found.")
			return
		}
		if cfg.FancyUI {
			ui.Box("Trash Search Results", results, 0)
		} else {
			for _, r := range results {
				fmt.Println(r)
			}
		}
		return
	}

	openMode := defaultOpen
	deleteMode := defaultDelete
	pinMode := defaultPin

	// Check for mode keywords as first positional arg (e.g., "gote search open -w 2512")
	first := args.First()
	if first == "open" {
		openMode = true
		args.Positional = args.Positional[1:]
	} else if first == "delete" {
		deleteMode = true
		args.Positional = args.Positional[1:]
	} else if first == "pin" {
		pinMode = true
		args.Positional = args.Positional[1:]
	}

	interactive := openMode || deleteMode || pinMode
	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")
	tags := args.TagList("t", "tags")
	dateValues := args.List("w", "when")

	// Date search mode: -w <date> [<date>] [--modified]
	if len(dateValues) > 0 && looksLikeDate(dateValues[0]) {
		useCreated := !args.Has("modified", "m") // default to created
		results, err := core.SearchNotesByDate(dateValues, useCreated, -1)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No notes found in that date range.")
			return
		}
		displayPaginatedSearchResultsWithMode(results, interactive, deleteMode, pinMode, pageSize)
		return
	}

	// Tag search mode
	if args.Has("t", "tags") {
		if len(tags) == 0 {
			fmt.Print("Tags: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			tags = ParseTagString(strings.TrimSpace(input))
			if len(tags) == 0 {
				return
			}
		}
		results, err := core.SearchNotesByTags(tags, -1) // Get all
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No notes found for the given tags.")
			return
		}
		displayPaginatedSearchResultsWithMode(results, interactive, deleteMode, pinMode, pageSize)
		return
	}

	// Title search mode
	query := strings.ToLower(args.Joined())
	if query == "" {
		fmt.Print("Search: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		query = strings.ToLower(strings.TrimSpace(input))
		if query == "" {
			return
		}
	}

	results, err := core.SearchNotesByTitle(query, -1) // Get all
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if len(results) == 0 {
		ui.Empty("No matching note titles found.")
		return
	}
	displayPaginatedSearchResultsWithMode(results, interactive, deleteMode, pinMode, pageSize)
}

// SelectCommand provides an interactive flow: choose source -> select note -> choose action
func SelectCommand() {
	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)
	pageSize := cfg.PageSize()

	// Step 1: Choose source (loop until valid input)
	var results []core.SearchResult
sourceLoop:
	for {
		if cfg.FancyUI {
			ui.Clear()
		}
		sourceItems := []string{"Recent", "Search", "Pinned", "Tag"}
		sourceKeys := []rune{'r', 's', 'p', 't'}
		if cfg.FancyUI {
			ui.SelectableList("Select Source", sourceItems, -1, sourceKeys)
		} else {
			fmt.Println("Select source:")
			for i, item := range sourceItems {
				fmt.Printf("[%c] %s\n", sourceKeys[i], item)
			}
			fmt.Print(": ")
		}

		sourceKey, err := ReadKey(cfg.FancyUI)
		if err != nil {
			return
		}

		// Step 2: Get notes based on source
		switch sourceKey {
		case 'q', 'Q':
			if cfg.FancyUI {
				ui.Clear()
			}
			return
		case 'r', 'R':
			notes, err := core.GetRecentNotes(-1)
			if err != nil {
				ui.Error(err.Error())
				return
			}
			for _, n := range notes {
				results = append(results, core.SearchResult{Title: n.Title, FilePath: n.FilePath})
			}
			break sourceLoop
		case 's', 'S':
			if cfg.FancyUI {
				ui.Clear()
			}
			fmt.Print("Search: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			query := strings.TrimSpace(input)
			if query == "" {
				return
			}
			results, err = core.SearchNotesByTitle(query, -1)
			if err != nil {
				ui.Error(err.Error())
				return
			}
			break sourceLoop
		case 'p', 'P':
			pins, err := data.LoadPins()
			if err != nil {
				ui.Error(err.Error())
				return
			}
			index, err := data.LoadIndex()
			if err != nil {
				ui.Error(err.Error())
				return
			}
			for title := range pins {
				if meta, exists := index[title]; exists {
					results = append(results, core.SearchResult{Title: title, FilePath: meta.FilePath})
				}
			}
			break sourceLoop
		case 't', 'T':
			if cfg.FancyUI {
				ui.Clear()
			}
			fmt.Print("Tags: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			tags := ParseTagString(strings.TrimSpace(input))
			if len(tags) == 0 {
				return
			}
			results, err = core.FilterNotesByTags(tags, -1)
			if err != nil {
				ui.Error(err.Error())
				return
			}
			break sourceLoop
		default:
			// Invalid or unexpected input (e.g., escape sequences on WSL) - reprompt
			continue
		}
	}
	if cfg.FancyUI {
		ui.Clear()
	} else {
		fmt.Println() // Spacing between sections in non-fancy mode
	}

	if len(results) == 0 {
		ui.Empty("No notes found.")
		return
	}

	// Step 3: Select a note
	selectedNote := selectNoteFromResults(results, cfg, ui, pageSize)
	if selectedNote == nil {
		return
	}

	// Step 4: Choose action (loop until valid input)
	if !cfg.FancyUI {
		fmt.Println() // Spacing between sections in non-fancy mode
	}
actionLoop:
	for {
		if cfg.FancyUI {
			ui.Clear()
		}
		actionItems := []string{"Open", "Rename", "Delete", "Pin/Unpin", "Info"}
		actionKeys := []rune{'o', 'r', 'd', 'p', 'i'}
		if cfg.FancyUI {
			ui.SelectableList("Action: "+selectedNote.Title, actionItems, -1, actionKeys)
		} else {
			fmt.Printf("Action for '%s':\n", selectedNote.Title)
			for i, item := range actionItems {
				fmt.Printf("[%c] %s\n", actionKeys[i], item)
			}
			fmt.Print(": ")
		}

		actionKey, err := ReadKey(cfg.FancyUI)
		if err != nil {
			return
		}

		// Step 5: Execute action
		switch actionKey {
		case 'q', 'Q':
			if cfg.FancyUI {
				ui.Clear()
			}
			return
		case 'o', 'O':
			if cfg.FancyUI {
				ui.Clear()
			}
			core.OpenAndReindexNote(selectedNote.FilePath, selectedNote.Title)
			break actionLoop
		case 'r', 'R':
			if cfg.FancyUI {
				ui.Clear()
			}
			fmt.Print("New name: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			newName := strings.TrimSpace(input)
			if newName == "" {
				ui.Info("Cancelled")
				return
			}
			if err := core.RenameNote(selectedNote.Title, newName); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Renamed to: " + newName)
			break actionLoop
		case 'd', 'D':
			if cfg.FancyUI {
				ui.Clear()
			}
			if err := core.DeleteNote(selectedNote.Title); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Moved to trash: " + selectedNote.Title)
			break actionLoop
		case 'p', 'P':
			if cfg.FancyUI {
				ui.Clear()
			}
			pins, _ := data.LoadPins()
			if _, isPinned := pins[selectedNote.Title]; isPinned {
				if err := core.UnpinNote(selectedNote.Title); err != nil {
					ui.Error(err.Error())
					return
				}
				ui.Success("Unpinned: " + selectedNote.Title)
			} else {
				if err := core.PinNote(selectedNote.Title); err != nil {
					ui.Error(err.Error())
					return
				}
				ui.Success("Pinned: " + selectedNote.Title)
			}
			break actionLoop
		case 'i', 'I':
			if cfg.FancyUI {
				ui.Clear()
			}
			info, err := core.GetNoteInfo(selectedNote.Title)
			if err != nil {
				ui.Error(err.Error())
				return
			}
			ui.InfoBox(selectedNote.Title, [][2]string{
				{"Created", info.Created},
				{"Modified", info.Modified},
				{"Words", fmt.Sprintf("%d", info.WordCount)},
				{"Chars", fmt.Sprintf("%d", info.CharCount)},
				{"Tags", strings.Join(info.Tags, ", ")},
			})
			break actionLoop
		default:
			// Invalid or unexpected input - reprompt
			continue
		}
	}
}

// selectNoteFromResults displays paginated results and returns the selected note
func selectNoteFromResults(results []core.SearchResult, cfg data.Config, ui *UI, pageSize int) *core.SearchResult {
	if pageSize > maxSelectablePageSize {
		pageSize = maxSelectablePageSize
	}

	page := 0
	totalPages := (len(results) + pageSize - 1) / pageSize

	for {
		start := page * pageSize
		end := min(start+pageSize, len(results))
		if start >= end {
			break
		}

		pageResults := results[start:end]
		var items []string
		var keys []rune
		for i, r := range pageResults {
			items = append(items, r.Title)
			if i < len(selectKeys) {
				keys = append(keys, selectKeys[i])
			}
		}

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList("Select Note", items, -1, keys)
			ui.NavHint(page+1, totalPages)
		} else {
			for i, item := range items {
				if i < len(keys) {
					fmt.Printf("[%c] %s\n", keys[i], item)
				} else {
					fmt.Println(item)
				}
			}
			fmt.Printf("(%d/%d) ", page+1, totalPages)
			if totalPages > 1 {
				fmt.Print("[n]ext [p]rev ")
			}
			fmt.Println("[q]uit")
			fmt.Print(": ")
		}

		key, err := ReadKey(cfg.FancyUI)
		if err != nil {
			return nil
		}

		switch key {
		case 0: // Empty input - reprompt
			continue
		case 'q', 'Q':
			if cfg.FancyUI {
				ui.Clear()
			}
			return nil
		case 'n', 'N':
			if page < totalPages-1 {
				page++
			}
			continue
		case 'p', 'P':
			if page > 0 {
				page--
			}
			continue
		}

		// Check if it's a selection key
		for i := 0; i < len(pageResults) && i < len(selectKeys); i++ {
			if key == selectKeys[i] || key == selectKeys[i+24] { // lowercase or uppercase
				if cfg.FancyUI {
					ui.Clear()
				}
				return &results[start+i]
			}
		}
	}
	return nil
}
