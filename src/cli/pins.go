package cli

import (
	"fmt"

	"gote/src/core"
	"gote/src/data"
)

func PinCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

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

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

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

func PinnedCommand(rawArgs []string, defaultOpen bool) {
	args := ParseArgs(rawArgs)
	openMode := defaultOpen

	// Check for mode keyword as first positional arg (e.g., "gote pinned open")
	if args.First() == "open" {
		openMode = true
		args.Positional = args.Positional[1:]
	}

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	pageSize := args.IntOr(cfg.PageSize(), "n", "limit")
	ui := NewUI(cfg.FancyUI)

	pins, err := core.ListPinnedNotes()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if len(pins) == 0 {
		ui.Empty("No pinned notes.")
		return
	}

	displayPaginatedResults(pins, openMode, pageSize, func(title string) {
		index, err := data.LoadIndex()
		if err != nil {
			ui.Error("Error loading index: " + err.Error())
			return
		}
		if meta, exists := index[title]; exists {
			data.OpenFileInEditor(meta.FilePath, cfg.Editor)
		}
	})
}

func listPinnedNotes() {
	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

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
