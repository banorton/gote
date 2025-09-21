package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"gote/src/core"
)

func RenameCommand(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: gote rename <notename> -n <newnotename>")
		return
	}
	nIdx := -1
	for i, arg := range args {
		if arg == "-n" {
			nIdx = i
			break
		}
	}
	if nIdx == -1 || nIdx == 0 || nIdx == len(args)-1 {
		fmt.Println("Usage: gote rename <notename> -n <newnotename>")
		return
	}
	oldName := strings.Join(args[:nIdx], " ")
	newName := strings.Join(args[nIdx+1:], " ")

	if err := core.RenameNote(oldName, newName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Printf("Renamed note '%s' to '%s'\n", oldName, newName)
}

func InfoCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote info <note name>")
		return
	}
	noteName := strings.Join(args, " ")
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