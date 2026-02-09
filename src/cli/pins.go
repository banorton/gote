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
		// No args = show interactive pinned menu
		PinnedCommand(nil, false, false, false, false, false)
		return
	}

	// Otherwise, pin the note
	noteName := args.Joined()
	noteName, err := ResolveNoteName(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}
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

	noteName, err := ResolveNoteName(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	if err := core.UnpinNote(noteName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Unpinned note: " + noteName)
}

func PinnedCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultUnpin bool, defaultView bool, defaultRename bool) {
	args := ParseArgs(rawArgs)

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
	} else if first == "unpin" || defaultUnpin {
		preSelected = "unpin"
		if first == "unpin" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "view" || defaultView {
		preSelected = "view"
		if first == "view" {
			args.Positional = args.Positional[1:]
		}
	} else if first == "rename" || defaultRename {
		preSelected = "rename"
		if first == "rename" {
			args.Positional = args.Positional[1:]
		}
	}

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}
	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")

	index, indexErr := data.LoadIndex()
	if indexErr != nil {
		ui.Error("Error loading index: " + indexErr.Error())
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

	// Build paths from index
	paths := make(map[string]string)
	for _, title := range pins {
		if meta, exists := index[title]; exists {
			paths[title] = meta.FilePath
		}
	}

	result := displayMenu(MenuConfig{
		Title:             "Pinned Notes",
		Items:             pins,
		ItemPaths:         paths,
		PreSelectedAction: preSelected,
		ShowUnpin:         true,
		PageSize:          pageSize,
	}, ui, cfg.FancyUI)

	executeMenuAction(result, paths, ui)
}
