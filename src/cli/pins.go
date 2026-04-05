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
		PinnedCommand(nil, ActionDefaults{})
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

func PinnedCommand(rawArgs []string, defaults ActionDefaults) {
	args := ParseArgs(rawArgs)
	preSelected := resolvePreSelectedAction(&args, defaults)

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
	}, ui, cfg.Interface)

	executeMenuAction(result, paths, ui)
}
