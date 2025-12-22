package cli

import (
	"fmt"

	"gote/src/core"
	"gote/src/data"
)

func PinCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	// Handle subcommands
	switch sub {
	case "format":
		if err := data.FormatPinsFile(); err != nil {
			fmt.Println("Error formatting pins:", err)
			return
		}
		fmt.Println("Pins file formatted.")
		return
	case "":
		// No args = list pinned notes
		listPinnedNotes()
		return
	}

	// Otherwise, pin the note
	noteName := args.Joined()
	if err := core.PinNote(noteName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Pinned note:", noteName)
}

func UnpinCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	if noteName == "" {
		fmt.Println("Usage: gote unpin <note name>")
		return
	}

	if err := core.UnpinNote(noteName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Unpinned note:", noteName)
}

func PinnedCommand(rawArgs []string, defaultOpen bool) {
	args := ParseArgs(rawArgs)
	openMode := defaultOpen || args.Has("o", "open")
	pageSize := args.IntOr(10, "n", "limit")

	pins, err := core.ListPinnedNotes()
	if err != nil {
		fmt.Println("Error loading pins:", err)
		return
	}
	if len(pins) == 0 {
		fmt.Println("No pinned notes.")
		return
	}

	displayPaginatedResults(pins, openMode, pageSize, func(title string) {
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

func listPinnedNotes() {
	pins, err := core.ListPinnedNotes()
	if err != nil {
		fmt.Println("Error loading pins:", err)
		return
	}
	if len(pins) == 0 {
		fmt.Println("No pinned notes.")
		return
	}
	fmt.Println("Pinned notes:")
	for _, title := range pins {
		fmt.Println(title)
	}
}
