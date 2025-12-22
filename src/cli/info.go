package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"gote/src/core"
)

func RenameCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	oldName := args.Joined()
	newName := strings.Join(args.List("n", "name"), " ")

	if oldName == "" || newName == "" {
		fmt.Println("Usage: gote rename <note name> -n <new name>")
		return
	}

	if err := core.RenameNote(oldName, newName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Renamed note '%s' to '%s'\n", oldName, newName)
}

func InfoCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	if noteName == "" {
		fmt.Println("Usage: gote info <note name>")
		return
	}

	meta, err := core.GetNoteInfo(noteName)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling note metadata:", err)
		return
	}
	fmt.Println(string(b))
}