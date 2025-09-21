package cli

import (
	"fmt"
	"strings"

	"gote/src/core"
)

func DeleteCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote delete <note name>")
		return
	}
	noteName := strings.Join(args, " ")
	if err := core.DeleteNote(noteName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Note moved to trash:", noteName)
}

func RecoverCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote recover <note name>")
		return
	}
	noteName := strings.Join(args, " ")
	if err := core.RecoverNote(noteName); err != nil {
		fmt.Println("Error recovering note:", err)
		return
	}
	fmt.Println("Note recovered:", noteName)
}