package cli

import (
	"fmt"
	"strconv"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

func displayPaginatedResults(results []string, openMode bool, onSelect func(string)) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	homerow := []rune{'a', 's', 'd', 'f', 'j', 'k', 'l', ';', 'g', 'h'}
	page := 0
	pageSize := 10
	totalPages := (len(results) + pageSize - 1) / pageSize

	for {
		start := page * pageSize
		end := start + pageSize
		if end > len(results) {
			end = len(results)
		}
		if start >= end {
			break
		}
		if page > 0 {
			fmt.Println()
		}
		fmt.Printf("Page (%d/%d):\n", page+1, totalPages)
		for i := start; i < end; i++ {
			if openMode && i-start < len(homerow) {
				fmt.Printf("[%c] %s\n", homerow[i-start], results[i])
			} else {
				fmt.Println(results[i])
			}
		}
		if !openMode {
			break
		}
		fmt.Print("[n] next page\n[enter] quit\n: ")
		var input string
		fmt.Scanln(&input)
		if input == "" {
			break
		}
		if input == "n" {
			page++
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

func displayPaginatedSearchResults(results []core.SearchResult, openMode bool) {
	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	homerow := []rune{'a', 's', 'd', 'f', 'j', 'k', 'l', ';', 'g', 'h'}
	page := 0
	pageSize := 10
	totalPages := (len(results) + pageSize - 1) / pageSize

	for {
		start := page * pageSize
		end := start + pageSize
		if end > len(results) {
			end = len(results)
		}
		if start >= end {
			break
		}
		if page > 0 {
			fmt.Println()
		}
		fmt.Printf("Page (%d/%d):\n", page+1, totalPages)
		for i := start; i < end; i++ {
			result := results[i]
			if openMode && i-start < len(homerow) {
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
		if !openMode {
			break
		}
		fmt.Print("(n) next page / (enter) quit: ")
		var input string
		fmt.Scanln(&input)
		if input == "" {
			break
		}
		if input == "n" {
			page++
			continue
		}
		for i := start; i < end && i-start < len(homerow); i++ {
			if input == string(homerow[i-start]) {
				cfg, err := data.LoadConfig()
				if err != nil {
					fmt.Println("Error loading config:", err)
					return
				}
				data.OpenFileInEditor(results[i].FilePath, cfg.Editor)
				return
			}
		}
		fmt.Println("Invalid input.")
	}
}

func RecentCommand(args []string) {
	n := 10
	openMode := false
	for _, arg := range args {
		if arg == "--open" || arg == "-o" {
			openMode = true
		} else if v, err := strconv.Atoi(arg); err == nil && v > 0 {
			n = v
		}
	}

	notes, err := core.GetRecentNotes(n)
	if err != nil {
		fmt.Println("Error getting recent notes:", err)
		return
	}

	var titles []string
	for _, note := range notes {
		titles = append(titles, note.Title)
	}

	displayPaginatedResults(titles, openMode, func(title string) {
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

func SearchCommand(args []string) {
	if len(args) > 0 && args[0] == "trash" {
		if len(args) < 2 {
			fmt.Println("Usage: gote search trash <query>")
			return
		}
		query := strings.ToLower(strings.Join(args[1:], " "))
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

	openMode := false
	var filteredArgs []string
	for _, arg := range args {
		if arg == "--open" || arg == "-o" {
			openMode = true
		} else {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	if len(filteredArgs) == 0 {
		fmt.Println("Usage: gote search <query> OR gote search -t <tag1> ... [-n <number>]")
		return
	}

	n := -1
	tagsMode := false
	tags := []string{}
	for i := 0; i < len(filteredArgs); i++ {
		if filteredArgs[i] == "-n" {
			if n == -1 {
				n = 10
			}
			if i+1 < len(filteredArgs) {
				if v, err := strconv.Atoi(filteredArgs[i+1]); err == nil && v > 0 {
					n = v
					i++
				}
			}
		} else if filteredArgs[i] == "-t" {
			tagsMode = true
			for j := i + 1; j < len(filteredArgs) && filteredArgs[j] != "-n"; j++ {
				tags = append(tags, filteredArgs[j])
				i = j
			}
		}
	}

	if tagsMode {
		if len(tags) == 0 {
			fmt.Println("Usage: gote search -t <tag1> ... [-n <number>]")
			return
		}
		results, err := core.SearchNotesByTags(tags, n)
		if err != nil {
			fmt.Println("Error searching by tags:", err)
			return
		}
		if len(results) == 0 {
			fmt.Println("No notes found for the given tags.")
			return
		}
		displayPaginatedSearchResults(results, openMode)
		return
	}

	query := strings.ToLower(strings.Join(filteredArgs, " "))
	results, err := core.SearchNotesByTitle(query, n)
	if err != nil {
		fmt.Println("Error searching notes:", err)
		return
	}
	if len(results) == 0 {
		fmt.Println("No matching note titles found.")
		return
	}
	displayPaginatedSearchResults(results, openMode)
}