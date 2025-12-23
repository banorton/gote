package cli

import (
	"fmt"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

func DeleteCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if noteName == "" {
		fmt.Println("Usage: gote delete <note name>")
		return
	}

	if err := core.DeleteNote(noteName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Note moved to trash: " + noteName)
}

func RecoverCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if noteName == "" {
		fmt.Println("Usage: gote recover <note name>")
		return
	}

	if err := core.RecoverNote(noteName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Note recovered: " + noteName)
}

func TrashCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	switch sub {
	case "":
		// List trashed notes
		notes, err := core.ListTrashedNotes()
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(notes) == 0 {
			ui.Empty("Trash is empty.")
			return
		}
		if cfg.FancyUI {
			ui.Box("Trash", notes, 0)
		} else {
			fmt.Println("Trashed notes:")
			for _, note := range notes {
				fmt.Println(note)
			}
		}
	case "empty":
		count, err := core.EmptyTrash()
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if count == 0 {
			ui.Empty("Trash was already empty.")
		} else {
			ui.Success(fmt.Sprintf("Permanently deleted %d note(s).", count))
		}
	case "search":
		query := strings.ToLower(strings.Join(args.Rest(), " "))
		if query == "" {
			fmt.Println("Usage: gote trash search <query>")
			return
		}
		results, err := core.SearchTrash(query)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No matching trashed notes found.")
			return
		}
		if cfg.FancyUI {
			ui.Box("Trash Search", results, 0)
		} else {
			for _, r := range results {
				fmt.Println(r)
			}
		}
	default:
		// Treat as note name to delete
		noteName := args.Joined()
		if err := core.DeleteNote(noteName); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Note moved to trash: " + noteName)
	}
}