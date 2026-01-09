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

	// Last opened note
	case "-":
		cli.LastCommand()

	// Recent notes
	case "recent", "r":
		cli.RecentCommand(rest, false, false, false, false)
	case "ro": // recent + open
		cli.RecentCommand(rest, true, false, false, false)
	case "rd": // recent + delete
		cli.RecentCommand(rest, false, true, false, false)
	case "rp": // recent + pin
		cli.RecentCommand(rest, false, false, true, false)
	case "rv": // recent + view
		cli.RecentCommand(rest, false, false, false, true)

	// Search
	case "search", "s":
		cli.SearchCommand(rest, false, false, false, false)
	case "so": // search + open
		cli.SearchCommand(rest, true, false, false, false)
	case "sd": // search + delete
		cli.SearchCommand(rest, false, true, false, false)
	case "sp": // search + pin
		cli.SearchCommand(rest, false, false, true, false)
	case "sv": // search + view
		cli.SearchCommand(rest, false, false, false, true)

	// Index management
	case "index", "idx":
		cli.IndexCommand(rest)

	// Tags
	case "tag", "t":
		cli.TagCommand(rest, false, false, false, false)
	case "to": // tag + open
		cli.TagCommand(rest, true, false, false, false)
	case "td": // tag + delete
		cli.TagCommand(rest, false, true, false, false)
	case "tp": // tag + pin
		cli.TagCommand(rest, false, false, true, false)
	case "tv": // tag + view
		cli.TagCommand(rest, false, false, false, true)

	// Get (interactive flow)
	case "get", "g":
		cli.GetCommand()

	// Config
	case "config", "c":
		cli.ConfigCommand(rest)
	case "ce": // config edit shortcut
		cli.ConfigCommand([]string{"edit"})

	// Templates
	case "template", "tmpl":
		cli.TemplateCommand(rest)

	// Pins
	case "pin", "p":
		cli.PinCommand(rest)
	case "unpin", "u", "up":
		cli.UnpinCommand(rest)
	case "pinned":
		cli.PinnedCommand(rest, false, false, false, false)
	case "po": // pinned + open
		cli.PinnedCommand(rest, true, false, false, false)
	case "pv": // pinned + view
		cli.PinnedCommand(rest, false, false, false, true)
	case "pu": // pinned + unpin
		cli.PinnedCommand(rest, false, false, true, false)

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
	case "view":
		cli.ViewCommand(rest)

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
