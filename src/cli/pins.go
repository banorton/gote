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

	displayPaginatedResults(pins, openMode, pageSize, func(title string) {
		if indexErr != nil {
			ui.Error("Error loading index: " + indexErr.Error())
			return
		}
		if meta, exists := index[title]; exists {
			core.OpenAndReindexNote(meta.FilePath, title)
		}
	})
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
