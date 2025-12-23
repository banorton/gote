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
// Homerow + bottom row (avoiding n for next) + top row (avoiding q for quit, p for prev)
var selectKeys = []rune{
	'a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l', ';', // homerow (10)
	'z', 'x', 'c', 'v', 'b', 'm',                     // bottom row (6, avoiding n)
	'w', 'e', 'r', 't', 'y', 'u', 'i', 'o',           // top row (8, avoiding q, p)
}

const maxSelectablePageSize = 24 // len(selectKeys)

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

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if pageSize <= 0 {
		pageSize = 10
	}
	if selectable && pageSize > maxSelectablePageSize {
		pageSize = maxSelectablePageSize
	}

	page := 0
	totalPages := (len(results) + pageSize - 1) / pageSize

	// Single page, no navigation needed
	if totalPages == 1 && !selectable {
		if cfg.FancyUI {
			ui.Box("Results", results, 0)
		} else {
			for _, r := range results {
				fmt.Println(r)
			}
		}
		return
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
			ui.NavHint(page+1, totalPages)
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
			fmt.Println("[q] quit")
			fmt.Print(": ")
		}

		// Single page non-selectable: just show and exit
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

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if pageSize <= 0 {
		pageSize = 10
	}
	if selectable && pageSize > maxSelectablePageSize {
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

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList(title, items, -1, keys)
			ui.NavHint(page+1, totalPages)
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
			fmt.Println("[q] quit")
			fmt.Print(": ")
		}

		// Single page non-selectable: just show and exit
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
						data.OpenFileInEditor(results[start+i].FilePath, cfg.Editor)
					}
					return
				}
			}
		}
	}
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

	cfg, _ := data.LoadConfig()
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
		index := data.LoadIndex()
		if meta, exists := index[title]; exists {
			data.OpenFileInEditor(meta.FilePath, cfg.Editor)
		}
	})
}

func SearchCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool) {
	args := ParseArgs(rawArgs)

	cfg, _ := data.LoadConfig()
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
