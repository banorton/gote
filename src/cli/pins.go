package cli

import (
	"fmt"

	"gote/src/core"
	"gote/src/data"
)

func PinCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	// Handle subcommands
	switch sub {
	case "format":
		if err := data.FormatPinsFile(); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Pins file formatted.")
		return
	case "":
		// No args = list pinned notes
		listPinnedNotes()
		return
	}

	// Otherwise, pin the note
	noteName := args.Joined()
	if err := core.PinNote(noteName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Pinned note: " + noteName)
}

func UnpinCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	if noteName == "" {
		fmt.Println("Usage: gote unpin <note name>")
		return
	}

	if err := core.UnpinNote(noteName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Unpinned note: " + noteName)
}

func PinnedCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool, defaultView bool) {
	args := ParseArgs(rawArgs)
	openMode := defaultOpen
	deleteMode := defaultDelete
	pinMode := defaultPin
	viewMode := defaultView

	// Check for mode keyword as first positional arg (e.g., "gote pinned open")
	first := args.First()
	if first == "open" {
		openMode = true
		args.Positional = args.Positional[1:]
	} else if first == "delete" {
		deleteMode = true
		args.Positional = args.Positional[1:]
	} else if first == "unpin" {
		pinMode = true // unpin mode for pinned notes
		args.Positional = args.Positional[1:]
	} else if first == "view" {
		viewMode = true
		args.Positional = args.Positional[1:]
	}

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}
	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")

	// Pre-load index for callbacks
	index, indexErr := data.LoadIndex()

	pins, err := core.ListPinnedNotes()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if len(pins) == 0 {
		ui.Empty("No pinned notes.")
		return
	}

	if deleteMode {
		displayPaginatedResults(pins, true, pageSize, func(title string) {
			if err := core.DeleteNote(title); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Note moved to trash: " + title)
		})
		return
	}

	if pinMode {
		displayPaginatedResults(pins, true, pageSize, func(title string) {
			if err := core.UnpinNote(title); err != nil {
				ui.Error(err.Error())
				return
			}
			ui.Success("Unpinned: " + title)
		})
		return
	}

	if viewMode {
		displayPaginatedResults(pins, true, pageSize, func(title string) {
			if indexErr != nil {
				ui.Error("Error loading index: " + indexErr.Error())
				return
			}
			if meta, exists := index[title]; exists {
				if err := ViewNoteInBrowser(meta.FilePath, title); err != nil {
					ui.Error(err.Error())
				}
			}
		})
		return
	}

	displayPaginatedPinnedResults(pins, openMode, pageSize, cfg, ui, index, indexErr)
}

// displayPaginatedPinnedResults displays pinned notes with open/view/unpin options
func displayPaginatedPinnedResults(pins []string, selectable bool, pageSize int, cfg data.Config, ui *UI, index map[string]data.NoteMeta, indexErr error) {
	if len(pins) == 0 {
		return
	}

	if pageSize <= 0 {
		pageSize = 10
	}

	page := 0
	totalPages := (len(pins) + pageSize - 1) / pageSize
	viewMode := false
	unpinMode := false

	// Single page, show menu
	if totalPages == 1 && !selectable {
		if cfg.FancyUI {
			ui.Box("Pinned", pins, 0)
			fmt.Printf("\n %s[o]pen  [v]iew  [u]npin  [q]uit%s\n", Dim, Reset)
		} else {
			for _, p := range pins {
				fmt.Println(p)
			}
			fmt.Println("[o]pen [v]iew [u]npin [q]uit")
			fmt.Print(": ")
		}
		key, err := ReadKey(cfg.FancyUI)
		if err != nil {
			return
		}
		switch key {
		case 'o', 'O':
			selectable = true
		case 'v', 'V':
			selectable, viewMode = true, true
		case 'u', 'U':
			selectable, unpinMode = true, true
		default:
			if cfg.FancyUI {
				ui.Clear()
			}
			return
		}
	}

	for {
		start := page * pageSize
		end := min(start+pageSize, len(pins))
		if start >= end {
			break
		}

		pageItems := pins[start:end]
		var keys []rune
		if selectable {
			for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
				keys = append(keys, selectKeys[i])
			}
		}

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList("Pinned", pageItems, -1, keys)
			ui.NavHintWithModes(page+1, totalPages, !selectable, !selectable)
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
				fmt.Print("[n]ext [p]rev ")
			}
			if !selectable {
				fmt.Print("[o]pen [v]iew [u]npin ")
			}
			fmt.Println("[q]uit")
			fmt.Print(": ")
		}

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
				selectable, viewMode, unpinMode = true, false, false
				continue
			}
		case 'v', 'V':
			if !selectable {
				selectable, viewMode, unpinMode = true, true, false
				continue
			}
		case 'u', 'U':
			if !selectable {
				selectable, viewMode, unpinMode = true, false, true
				continue
			}
		}

		if selectable {
			for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
				if key == selectKeys[i] {
					if cfg.FancyUI {
						ui.Clear()
					}
					title := pins[start+i]
					if viewMode {
						if indexErr != nil {
							ui.Error("Error loading index: " + indexErr.Error())
							return
						}
						if meta, exists := index[title]; exists {
							if err := ViewNoteInBrowser(meta.FilePath, title); err != nil {
								ui.Error(err.Error())
							}
						}
					} else if unpinMode {
						if err := core.UnpinNote(title); err != nil {
							ui.Error(err.Error())
							return
						}
						ui.Success("Unpinned: " + title)
					} else {
						if indexErr != nil {
							ui.Error("Error loading index: " + indexErr.Error())
							return
						}
						if meta, exists := index[title]; exists {
							core.OpenAndReindexNote(meta.FilePath, title)
						}
					}
					return
				}
			}
		}
	}
}

func listPinnedNotes() {
	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	pins, err := core.ListPinnedNotes()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if len(pins) == 0 {
		ui.Empty("No pinned notes.")
		return
	}
	if cfg.FancyUI {
		ui.Box("Pinned Notes", pins, 0)
	} else {
		fmt.Println("Pinned notes:")
		for _, title := range pins {
			fmt.Println(title)
		}
	}
}
