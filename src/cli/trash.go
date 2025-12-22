package cli

import (
	"fmt"
	"strings"

	"gote/src/core"
)

func DeleteCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	if noteName == "" {
		fmt.Println("Usage: gote delete <note name>")
		return
	}

	if err := core.DeleteNote(noteName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Note moved to trash:", noteName)
}

func RecoverCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	if noteName == "" {
		fmt.Println("Usage: gote recover <note name>")
		return
	}

	if err := core.RecoverNote(noteName); err != nil {
		fmt.Println("Error recovering note:", err)
		return
	}
	fmt.Println("Note recovered:", noteName)
}

func TrashCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	switch sub {
	case "":
		// List trashed notes
		notes, err := core.ListTrashedNotes()
		if err != nil {
			fmt.Println("Error listing trash:", err)
			return
		}
		if len(notes) == 0 {
			fmt.Println("Trash is empty.")
			return
		}
		fmt.Println("Trashed notes:")
		for _, note := range notes {
			fmt.Println("  " + note)
		}
	case "empty":
		count, err := core.EmptyTrash()
		if err != nil {
			fmt.Println("Error emptying trash:", err)
			return
		}
		if count == 0 {
			fmt.Println("Trash was already empty.")
		} else {
			fmt.Printf("Permanently deleted %d note(s).\n", count)
		}
	case "search":
		query := strings.ToLower(strings.Join(args.Rest(), " "))
		if query == "" {
			fmt.Println("Usage: gote trash search <query>")
			return
		}
		results, err := core.SearchTrash(query)
		if err != nil {
			fmt.Println("Error searching trash:", err)
			return
		}
		if len(results) == 0 {
			fmt.Println("No matching trashed notes found.")
			return
		}
		for _, r := range results {
			fmt.Println(r)
		}
	default:
		// Treat as note name to delete
		noteName := args.Joined()
		if err := core.DeleteNote(noteName); err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Println("Note moved to trash:", noteName)
	}
}