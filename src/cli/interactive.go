package cli

import (
	"fmt"
	"strconv"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

func displayPaginatedResults(results []string, interactive bool, pageSize int, onSelect func(string)) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

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

		if page > 0 {
			fmt.Println()
		}

		for i := start; i < end; i++ {
			if interactive && i-start < len(homerow) {
				fmt.Printf("[%c] %s\n", homerow[i-start], results[i])
			} else {
				fmt.Println(results[i])
			}
		}

		if !interactive {
			break
		}

		fmt.Printf("\n(%d/%d)\n[n] next [p] prev [q] quit\n: ", page+1, totalPages)

		var input string
		fmt.Scanln(&input)
		if input == "q" {
			break
		} else if input == "n" {
			if page < totalPages-1 {
				page++
			}
			continue
		} else if input == "p" {
			if page > 0 {
				page--
			}
			continue
		}

		for i := start; i < end && i-start < len(homerow); i++ {
			if input == string(homerow[i-start]) {
				onSelect(results[i])
				return
			}
		}
		fmt.Println("Invalid input.")
	}
}

func displayPaginatedSearchResultsWithMode(results []core.SearchResult, interactive bool, deleteMode bool, pageSize int) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

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

		if page > 0 {
			fmt.Println()
		}

		for i := start; i < end; i++ {
			result := results[i]
			if interactive && i-start < len(homerow) {
				if result.Score > 1 {
					fmt.Printf("[%c] %s (matched %d tags)\n", homerow[i-start], result.Title, result.Score)
				} else {
					fmt.Printf("[%c] %s\n", homerow[i-start], result.Title)
				}
			} else {
				if result.Score > 1 {
					fmt.Printf("%s (matched %d tags)\n", result.Title, result.Score)
				} else {
					fmt.Println(result.Title)
				}
			}
		}

		if !interactive {
			break
		}

		fmt.Printf("\n(%d/%d)\n[n] next [p] prev [q] quit\n: ", page+1, totalPages)

		var input string
		fmt.Scanln(&input)
		if input == "q" {
			break
		} else if input == "n" {
			if page < totalPages-1 {
				page++
			}
			continue
		} else if input == "p" {
			if page > 0 {
				page--
			}
			continue
		}
		for i := start; i < end && i-start < len(homerow); i++ {
			if input == string(homerow[i-start]) {
				if deleteMode {
					if err := core.DeleteNote(results[i].Title); err != nil {
						fmt.Println("Error:", err)
						return
					}
					fmt.Println("Note moved to trash:", results[i].Title)
				} else {
					cfg, err := data.LoadConfig()
					if err != nil {
						fmt.Println("Error loading config:", err)
						return
					}
					data.OpenFileInEditor(results[i].FilePath, cfg.Editor)
				}
				return
			}
		}
		fmt.Println("Invalid input.")
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

	if deleteMode {
		displayPaginatedResults(titles, true, pageSize, func(title string) {
			if err := core.DeleteNote(title); err != nil {
				fmt.Println("Error:", err)
				return
			}
			fmt.Println("Note moved to trash:", title)
		})
		return
	}

	displayPaginatedResults(titles, openMode, pageSize, func(title string) {
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			return
		}
		index := data.LoadIndex()
		if meta, exists := index[title]; exists {
			data.OpenFileInEditor(meta.FilePath, cfg.Editor)
		}
	})
}

func SearchCommand(rawArgs []string, defaultOpen bool, defaultDelete bool) {
	args := ParseArgs(rawArgs)

	// Handle "search trash <query>" subcommand
	if args.First() == "trash" {
		query := strings.ToLower(strings.Join(args.Rest(), " "))
		if query == "" {
			fmt.Println("Usage: gote search trash <query>")
			return
		}
		results, err := core.SearchTrash(query)
		if err != nil {
			fmt.Println("Could not read trash:", err)
			return
		}
		if len(results) == 0 {
			fmt.Println("No matching trashed notes found.")
			return
		}
		for _, r := range results {
			fmt.Println(r)
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
			fmt.Println("Error searching by tags:", err)
			return
		}
		if len(results) == 0 {
			fmt.Println("No notes found for the given tags.")
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
		fmt.Println("Error searching notes:", err)
		return
	}
	if len(results) == 0 {
		fmt.Println("No matching note titles found.")
		return
	}
	displayPaginatedSearchResultsWithMode(results, interactive, deleteMode, pageSize)
}

