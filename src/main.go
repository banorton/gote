package main

import (
	"os"

	"gote/src/cli"
)

func main() {
	args := os.Args

	if len(args) == 1 {
		cli.QuickCommand()
		return
	}

	switch args[1] {
	case "quick", "q":
		cli.QuickCommand()
	case "recent", "r", "ro":
		if args[1] == "ro" {
			cli.RecentCommand(append(args[2:], "--open"))
		} else {
			cli.RecentCommand(args[2:])
		}
	case "index", "idx":
		cli.IndexCommand(args[2:])
	case "tags", "ts":
		cli.TagsCommand(args[2:])
	case "tag", "t":
		cli.TagCommand(args[2:])
	case "config", "c":
		cli.ConfigCommand(args[2:])
	case "search", "s", "so":
		if args[1] == "so" {
			cli.SearchCommand(append(args[2:], "--open"))
		} else {
			cli.SearchCommand(args[2:])
		}
	case "pin", "p":
		cli.PinCommand(args[2:])
	case "unpin", "u", "up":
		cli.UnpinCommand(args[2:])
	case "pinned", "pd":
		cli.PinnedCommand(args[2:])
	case "delete", "d", "del", "trash":
		cli.DeleteCommand(args[2:])
	case "recover":
		cli.RecoverCommand(args[2:])
	case "rename", "mv", "rn":
		cli.RenameCommand(args[2:])
	case "info", "i":
		cli.InfoCommand(args[2:])
	case "help", "h", "man":
		cli.HelpCommand(args[2:])
	case "view", "v":
	case "popular", "pop":
	case "today":
	case "journal", "j":
	case "transfer":
	case "calendar", "cal":
	case "lint", "l":
	default:
		cli.NoteCommand(args[1:])
	}
}
