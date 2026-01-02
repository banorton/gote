package main

import (
	"fmt"
	"os"

	"gote/src/cli"
)

const Version = "0.1.0"

func main() {
	args := os.Args

	if len(args) == 1 {
		cli.QuickCommand()
		return
	}

	cmd := args[1]
	rest := args[2:]

	switch cmd {
	// Version
	case "-v", "--version", "version":
		fmt.Println("gote", Version)
		return

	// Notes
	case "quick", "q":
		if len(rest) > 0 && (rest[0] == "save" || rest[0] == "s") {
			cli.QuickSaveCommand(rest[1:])
		} else {
			cli.QuickCommand()
		}
	case "qs":
		cli.QuickSaveCommand(rest)

	// Recent notes
	case "recent", "r":
		cli.RecentCommand(rest, false, false, false)
	case "ro": // recent + open
		cli.RecentCommand(rest, true, false, false)
	case "rd": // recent + delete
		cli.RecentCommand(rest, false, true, false)
	case "rp": // recent + pin
		cli.RecentCommand(rest, false, false, true)

	// Search
	case "search", "s":
		cli.SearchCommand(rest, false, false, false)
	case "so": // search + open
		cli.SearchCommand(rest, true, false, false)
	case "sd": // search + delete
		cli.SearchCommand(rest, false, true, false)
	case "sp": // search + pin
		cli.SearchCommand(rest, false, false, true)

	// Index management
	case "index", "idx":
		cli.IndexCommand(rest)

	// Tags
	case "tag", "t":
		cli.TagCommand(rest, false, false, false)
	case "to": // tag + open
		cli.TagCommand(rest, true, false, false)
	case "td": // tag + delete
		cli.TagCommand(rest, false, true, false)
	case "tp": // tag + pin
		cli.TagCommand(rest, false, false, true)

	// Select (interactive flow)
	case "select", "sel":
		cli.SelectCommand()

	// Config
	case "config", "c":
		cli.ConfigCommand(rest)

	// Pins
	case "pin", "p":
		cli.PinCommand(rest)
	case "unpin", "u", "up":
		cli.UnpinCommand(rest)
	case "pinned", "pd":
		cli.PinnedCommand(rest, false)
	case "po": // pinned + open
		cli.PinnedCommand(rest, true)

	// Trash
	case "delete", "d", "del":
		cli.DeleteCommand(rest)
	case "trash":
		cli.TrashCommand(rest)
	case "recover":
		cli.RecoverCommand(rest)

	// Note operations
	case "rename", "mv", "rn":
		cli.RenameCommand(rest)
	case "info", "i":
		cli.InfoCommand(rest)
	case "preview", "view", "pv":
		cli.PreviewCommand(rest)

	// Help
	case "help", "h", "man":
		cli.HelpCommand(rest)

	// Not implemented
	case "popular", "pop":
		cli.NotImplementedCommand("popular")
	case "today":
		cli.NotImplementedCommand("today")
	case "journal", "j":
		cli.NotImplementedCommand("journal")
	case "transfer":
		cli.NotImplementedCommand("transfer")
	case "calendar", "cal":
		cli.NotImplementedCommand("calendar")
	case "lint", "l":
		cli.NotImplementedCommand("lint")

	// Default: open/create note
	default:
		cli.NoteCommand(args[1:])
	}
}
