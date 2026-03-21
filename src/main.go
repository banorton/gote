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
		cli.RecentCommand(rest, cli.ActionDefaults{})
	case "ro": // recent + open
		cli.RecentCommand(rest, cli.ActionDefaults{Open: true})
	case "rd": // recent + delete
		cli.RecentCommand(rest, cli.ActionDefaults{Delete: true})
	case "rp": // recent + pin
		cli.RecentCommand(rest, cli.ActionDefaults{Pin: true})
	case "rv": // recent + view
		cli.RecentCommand(rest, cli.ActionDefaults{View: true})
	case "rr": // recent + rename
		cli.RecentCommand(rest, cli.ActionDefaults{Rename: true})

	// Search
	case "search", "s":
		cli.SearchCommand(rest, cli.ActionDefaults{})
	case "so": // search + open
		cli.SearchCommand(rest, cli.ActionDefaults{Open: true})
	case "sd": // search + delete
		cli.SearchCommand(rest, cli.ActionDefaults{Delete: true})
	case "sp": // search + pin
		cli.SearchCommand(rest, cli.ActionDefaults{Pin: true})
	case "sv": // search + view
		cli.SearchCommand(rest, cli.ActionDefaults{View: true})
	case "sr": // search + rename
		cli.SearchCommand(rest, cli.ActionDefaults{Rename: true})

	// Index management
	case "index", "idx":
		cli.IndexCommand(rest)

	// Tags
	case "tag", "t":
		cli.TagCommand(rest, cli.ActionDefaults{})
	case "to": // tag + open
		cli.TagCommand(rest, cli.ActionDefaults{Open: true})
	case "td": // tag + delete
		cli.TagCommand(rest, cli.ActionDefaults{Delete: true})
	case "tp": // tag + pin
		cli.TagCommand(rest, cli.ActionDefaults{Pin: true})
	case "tv": // tag + view
		cli.TagCommand(rest, cli.ActionDefaults{View: true})
	case "tr": // tag + rename
		cli.TagCommand(rest, cli.ActionDefaults{Rename: true})

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
		cli.PinnedCommand(rest, cli.ActionDefaults{})
	case "po": // pinned + open
		cli.PinnedCommand(rest, cli.ActionDefaults{Open: true})
	case "pd": // pinned + delete
		cli.PinnedCommand(rest, cli.ActionDefaults{Delete: true})
	case "pv": // pinned + view
		cli.PinnedCommand(rest, cli.ActionDefaults{View: true})
	case "pu": // pinned + unpin
		cli.PinnedCommand(rest, cli.ActionDefaults{Unpin: true})
	case "pr": // pinned + rename
		cli.PinnedCommand(rest, cli.ActionDefaults{Rename: true})

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
	case "duplicate", "dup", "cp":
		cli.DuplicateCommand(rest)
	case "info", "i":
		cli.InfoCommand(rest)
	case "view", "v":
		cli.ViewCommand(rest)

	// Help
	case "help", "h", "man":
		cli.HelpCommand(rest)

	// Default: open/create note
	default:
		cli.NoteCommand(args[1:])
	}
}
