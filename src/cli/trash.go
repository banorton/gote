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

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

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

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

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

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	switch sub {
	case "":
		// List trashed notes
		notes, err := data.ListTrashedNotes()
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
		count, err := data.EmptyTrash()
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
		results, err := data.SearchTrash(query)
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