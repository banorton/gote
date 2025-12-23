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
		cli.QuickCommand()

	// Recent notes
	case "recent", "r":
		cli.RecentCommand(rest, false, false)
	case "ro": // recent + open
		cli.RecentCommand(rest, true, false)
	case "rd": // recent + delete
		cli.RecentCommand(rest, false, true)

	// Search
	case "search", "s":
		cli.SearchCommand(rest, false, false)
	case "so": // search + open
		cli.SearchCommand(rest, true, false)
	case "sd": // search + delete
		cli.SearchCommand(rest, false, true)

	// Index management
	case "index", "idx":
		cli.IndexCommand(rest)

	// Tags
	case "tags", "ts":
		cli.TagsCommand(rest)
	case "tag", "t":
		cli.TagCommand(rest)

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

	// Help
	case "help", "h", "man":
		cli.HelpCommand(rest)

	// Not implemented
	case "view":
		cli.NotImplementedCommand("view")
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
