package cli

import (
	"fmt"
	"strconv"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

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

	homerow := []rune{'a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l', ';'}
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
			for i := 0; i < len(pageItems) && i < len(homerow); i++ {
				keys = append(keys, homerow[i])
			}
		}

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList("Results", pageItems, -1, keys)
			ui.NavHint(page+1, totalPages, selectable)
		} else {
			for i, item := range pageItems {
				if selectable && i < len(homerow) {
					fmt.Printf("[%c] %s\n", homerow[i], item)
				} else {
					fmt.Println(item)
				}
			}
			fmt.Printf("\n(%d/%d)\n", page+1, totalPages)
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

		key, err := ReadKey()
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
			for i := 0; i < len(pageItems) && i < len(homerow); i++ {
				if key == homerow[i] {
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

func displayPaginatedSearchResultsWithMode(results []core.SearchResult, selectable bool, deleteMode bool, pageSize int) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if pageSize <= 0 {
		pageSize = 10
	}

	homerow := []rune{'a', 's', 'd', 'f', 'g', 'h', 'j', 'k', 'l', ';'}
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
			if selectable && i < len(homerow) {
				keys = append(keys, homerow[i])
			}
		}

		title := "Search Results"
		if deleteMode {
			title = "Search Results (delete mode)"
		}

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList(title, items, -1, keys)
			ui.NavHint(page+1, totalPages, selectable)
		} else {
			for i, item := range items {
				if selectable && i < len(homerow) {
					fmt.Printf("[%c] %s\n", homerow[i], item)
				} else {
					fmt.Println(item)
				}
			}
			fmt.Printf("\n(%d/%d)\n", page+1, totalPages)
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

		key, err := ReadKey()
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
			for i := 0; i < len(pageResults) && i < len(homerow); i++ {
				if key == homerow[i] {
					if cfg.FancyUI {
						ui.Clear()
					}
					if deleteMode {
						if err := core.DeleteNote(results[start+i].Title); err != nil {
							ui.Error(err.Error())
							return
						}
						ui.Success("Note moved to trash: " + results[start+i].Title)
					} else {
						data.OpenFileInEditor(results[start+i].FilePath, cfg.Editor)
					}
					return
				}
			}
		}
	}
}

func RecentCommand(rawArgs []string, defaultOpen bool, defaultDelete bool) {
	args := ParseArgs(rawArgs)
	openMode := defaultOpen || args.Has("o", "open")
	deleteMode := defaultDelete || args.Has("d", "delete")
	pageSize := args.IntOr(10, "n", "limit")

	// Support bare number as first positional arg (e.g., "gote r 5")
	if pageSize == 10 && args.First() != "" {
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

	cfg, _ := data.LoadConfig()

	if deleteMode {
		displayPaginatedResults(titles, true, pageSize, func(title string) {
			ui := NewUI(cfg.FancyUI)
			if err := core.DeleteNote(title); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Note moved to trash: " + title)
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

func SearchCommand(rawArgs []string, defaultOpen bool, defaultDelete bool) {
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

	openMode := defaultOpen || args.Has("o", "open")
	deleteMode := defaultDelete || args.Has("d", "delete")
	interactive := openMode || deleteMode
	pageSize := args.IntOr(10, "n", "limit")
	tags := args.List("t", "tags")

	// Tag search mode
	if len(tags) > 0 {
		results, err := core.SearchNotesByTags(tags, -1) // Get all
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No notes found for the given tags.")
			return
		}
		displayPaginatedSearchResultsWithMode(results, interactive, deleteMode, pageSize)
		return
	}

	// Title search mode
	query := strings.ToLower(args.Joined())
	if query == "" {
		fmt.Println("Usage: gote search <query> OR gote search -t <tag1> ... [-n <pageSize>]")
		return
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
	displayPaginatedSearchResultsWithMode(results, interactive, deleteMode, pageSize)
}
