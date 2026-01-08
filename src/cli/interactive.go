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

// MenuConfig configures the unified menu display
type MenuConfig struct {
	Title           string
	Items           []string                   // Note titles to display
	ItemPaths       map[string]string          // Title -> FilePath mapping
	PreSelectedAction string                   // "open", "delete", etc. or "" for full menu
	ShowPin         bool                       // Show pin action
	ShowUnpin       bool                       // Show unpin action (mutually exclusive with ShowPin)
	PageSize        int
}

// MenuResult is returned from displayMenu
type MenuResult struct {
	Note   string // Selected note title
	Action string // "open", "view", "delete", "rename", "pin", "unpin", "info", or ""
}

// displayMenu shows a paginated menu with items and handles input
// In pre-selected mode: user types item letter + Enter
// In full menu mode: user types action+item combo + Enter (e.g., "oa" to open item a)
func displayMenu(cfg MenuConfig, ui *UI, fancyUI bool) MenuResult {
	if len(cfg.Items) == 0 {
		fmt.Println("No results found.")
		return MenuResult{}
	}

	pageSize := cfg.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > maxSelectablePageSize {
		pageSize = maxSelectablePageSize
	}

	page := 0
	totalPages := (len(cfg.Items) + pageSize - 1) / pageSize

	// Build action display based on config
	var actions string
	if cfg.PreSelectedAction != "" {
		actions = fmt.Sprintf("Select note to %s:", cfg.PreSelectedAction)
	} else {
		actions = "[o]pen [v]iew [d]elete [r]ename"
		if cfg.ShowPin {
			actions += " [p]in"
		}
		if cfg.ShowUnpin {
			actions += " [u]npin"
		}
		actions += " [i]nfo"
	}

	for {
		start := page * pageSize
		end := min(start+pageSize, len(cfg.Items))
		if start >= end {
			break
		}

		pageItems := cfg.Items[start:end]
		var keys []rune
		for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
			keys = append(keys, selectKeys[i])
		}

		// Display
		if fancyUI {
			ui.Clear()
			ui.SelectableList(cfg.Title, pageItems, -1, keys)
			// Navigation + actions
			fmt.Printf("\n %s(%d/%d)%s", Dim, page+1, totalPages, Reset)
			if totalPages > 1 {
				fmt.Printf(" %s[n]ext [p]rev%s", Dim, Reset)
			}
			fmt.Printf(" %s[q]uit%s\n", Dim, Reset)
			fmt.Printf(" %s%s%s\n", Dim, actions, Reset)
		} else {
			for i, item := range pageItems {
				fmt.Printf("[%c] %s\n", keys[i], item)
			}
			fmt.Printf("(%d/%d)────────────────────────\n", page+1, totalPages)
			if totalPages > 1 {
				fmt.Print("[n]ext [p]rev ")
			}
			fmt.Println("[q]uit")
			fmt.Println(actions)
			fmt.Print(": ")
		}

		// Read input
		input := ui.ReadMenuInput()
		if input == "" {
			continue
		}

		// Handle navigation
		if input == "q" {
			if fancyUI {
				ui.Clear()
			}
			return MenuResult{}
		}
		if input == "n" && page < totalPages-1 {
			page++
			continue
		}
		if input == "p" && page > 0 {
			page--
			continue
		}

		// Parse input
		var action string
		var itemKey rune

		if cfg.PreSelectedAction != "" {
			// Pre-selected mode: input is just the item letter
			if len(input) == 1 {
				action = cfg.PreSelectedAction
				itemKey = rune(input[0])
			}
		} else {
			// Full menu mode: input is action+item (e.g., "oa")
			if len(input) == 2 {
				actionKey := input[0]
				itemKey = rune(input[1])
				switch actionKey {
				case 'o':
					action = "open"
				case 'v':
					action = "view"
				case 'd':
					action = "delete"
				case 'r':
					action = "rename"
				case 'p':
					if cfg.ShowPin {
						action = "pin"
					}
				case 'u':
					if cfg.ShowUnpin {
						action = "unpin"
					}
				case 'i':
					action = "info"
				}
			}
		}

		if action == "" {
			continue // Invalid input
		}

		// Find selected item
		for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
			if itemKey == selectKeys[i] || itemKey == selectKeys[i]+32 || itemKey == selectKeys[i]-32 {
				// Handle both cases for letter keys
				if fancyUI {
					ui.Clear()
				}
				return MenuResult{Note: cfg.Items[start+i], Action: action}
			}
		}
	}
	return MenuResult{}
}

// executeMenuAction performs the action from a menu result
func executeMenuAction(result MenuResult, paths map[string]string, ui *UI) {
	if result.Note == "" || result.Action == "" {
		return
	}

	filePath := paths[result.Note]

	switch result.Action {
	case "open":
		core.OpenAndReindexNote(filePath, result.Note)
	case "view":
		if err := ViewNoteInBrowser(filePath, result.Note); err != nil {
			ui.Error(err.Error())
		}
	case "delete":
		// Confirm deletion
		fmt.Printf("Delete \"%s\"? [y/n]: ", result.Note)
		confirm := ui.ReadMenuInput()
		if confirm != "y" {
			ui.Info("Cancelled")
			return
		}
		if err := core.DeleteNote(result.Note); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Moved to trash: " + result.Note)
	case "rename":
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
		if err := core.RenameNote(result.Note, newName); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Renamed to: " + newName)
	case "pin":
		if err := core.PinNote(result.Note); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Pinned: " + result.Note)
	case "unpin":
		if err := core.UnpinNote(result.Note); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Unpinned: " + result.Note)
	case "info":
		info, err := core.GetNoteInfo(result.Note)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		ui.InfoBox(result.Note, [][2]string{
			{"Created", info.Created},
			{"Modified", info.Modified},
			{"Words", fmt.Sprintf("%d", info.WordCount)},
			{"Chars", fmt.Sprintf("%d", info.CharCount)},
			{"Tags", strings.Join(info.Tags, ", ")},
		})
	}
}

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

func RecentCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool, defaultView bool) {
	args := ParseArgs(rawArgs)

	// Determine pre-selected action from flags or keywords
	var preSelected string
	first := args.First()
	if first == "open" || defaultOpen {
		preSelected = "open"
		if first == "open" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "delete" || defaultDelete {
		preSelected = "delete"
		if first == "delete" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "pin" || defaultPin {
		preSelected = "pin"
		if first == "pin" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "view" || defaultView {
		preSelected = "view"
		if first == "view" {
			args.Positional = args.Positional[1:]
		}
	}

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}
	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")

	// Support bare number as first positional arg (e.g., "gote r 5")
	if pageSize == cfg.PageSize() && args.First() != "" {
		if v, err := strconv.Atoi(args.First()); err == nil && v > 0 {
			pageSize = v
		}
	}

	notes, err := core.GetRecentNotes(-1)
	if err != nil {
		fmt.Println("Error getting recent notes:", err)
		return
	}

	// Build items and paths
	var titles []string
	paths := make(map[string]string)
	for _, note := range notes {
		titles = append(titles, note.Title)
		paths[note.Title] = note.FilePath
	}

	result := displayMenu(MenuConfig{
		Title:             "Recent Notes",
		Items:             titles,
		ItemPaths:         paths,
		PreSelectedAction: preSelected,
		ShowPin:           true,
		PageSize:          pageSize,
	}, ui, cfg.FancyUI)

	executeMenuAction(result, paths, ui)
}

// searchResultsToMenu converts SearchResults to menu items and paths
func searchResultsToMenu(results []core.SearchResult) ([]string, map[string]string) {
	var titles []string
	paths := make(map[string]string)
	for _, r := range results {
		titles = append(titles, r.Title)
		paths[r.Title] = r.FilePath
	}
	return titles, paths
}

func SearchCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool, defaultView bool) {
	args := ParseArgs(rawArgs)

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

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

	// Determine pre-selected action
	var preSelected string
	first := args.First()
	if first == "open" || defaultOpen {
		preSelected = "open"
		if first == "open" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "delete" || defaultDelete {
		preSelected = "delete"
		if first == "delete" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "pin" || defaultPin {
		preSelected = "pin"
		if first == "pin" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "view" || defaultView {
		preSelected = "view"
		if first == "view" {
			args.Positional = args.Positional[1:]
		}
	}

	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")
	tags := args.TagList("t", "tags")
	dateValues := args.List("w", "when")

	var results []core.SearchResult
	var err error

	// Date search mode: -w <date> [<date>] [--modified]
	if len(dateValues) > 0 && looksLikeDate(dateValues[0]) {
		useCreated := !args.Has("modified", "m")
		results, err = core.SearchNotesByDate(dateValues, useCreated, -1)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No notes found in that date range.")
			return
		}
	} else if args.Has("t", "tags") {
		// Tag search mode
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
		results, err = core.SearchNotesByTags(tags, -1)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No notes found for the given tags.")
			return
		}
	} else {
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
		results, err = core.SearchNotesByTitle(query, -1)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No matching note titles found.")
			return
		}
	}

	titles, paths := searchResultsToMenu(results)
	result := displayMenu(MenuConfig{
		Title:             "Search Results",
		Items:             titles,
		ItemPaths:         paths,
		PreSelectedAction: preSelected,
		ShowPin:           true,
		PageSize:          pageSize,
	}, ui, cfg.FancyUI)

	executeMenuAction(result, paths, ui)
}

// GetCommand provides an interactive flow: choose source -> select note with actions
func GetCommand() {
	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}
	pageSize := cfg.PageSize()

	// Step 1: Choose source
	var results []core.SearchResult
	var title string
sourceLoop:
	for {
		if cfg.FancyUI {
			ui.Clear()
		}
		fmt.Println("Select source:")
		fmt.Println("[r] Recent")
		fmt.Println("[s] Search")
		fmt.Println("[p] Pinned")
		fmt.Println("[t] Tag")
		fmt.Println("[q] Quit")
		fmt.Print(": ")

		input := ui.ReadMenuInput()
		switch input {
		case "q":
			return
		case "r":
			notes, err := core.GetRecentNotes(-1)
			if err != nil {
				ui.Error(err.Error())
				return
			}
			for _, n := range notes {
				results = append(results, core.SearchResult{Title: n.Title, FilePath: n.FilePath})
			}
			title = "Recent Notes"
			break sourceLoop
		case "s":
			fmt.Print("Search: ")
			reader := bufio.NewReader(os.Stdin)
			query, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			query = strings.TrimSpace(query)
			if query == "" {
				return
			}
			results, err = core.SearchNotesByTitle(query, -1)
			if err != nil {
				ui.Error(err.Error())
				return
			}
			title = "Search Results"
			break sourceLoop
		case "p":
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
			for t := range pins {
				if meta, exists := index[t]; exists {
					results = append(results, core.SearchResult{Title: t, FilePath: meta.FilePath})
				}
			}
			title = "Pinned Notes"
			break sourceLoop
		case "t":
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
			title = "Tagged Notes"
			break sourceLoop
		}
	}

	if len(results) == 0 {
		ui.Empty("No notes found.")
		return
	}

	// Step 2: Display notes with full menu
	titles, paths := searchResultsToMenu(results)
	result := displayMenu(MenuConfig{
		Title:    title,
		Items:    titles,
		ItemPaths: paths,
		ShowPin:  true,
		PageSize: pageSize,
	}, ui, cfg.FancyUI)

	executeMenuAction(result, paths, ui)
}
