package cli

import (
	"fmt"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

func PinCommand(args []string) {
	if len(args) == 1 && args[0] == "format" {
		if err := data.FormatPinsFile(); err != nil {
			fmt.Println("Error formatting pins:", err)
			return
		}
		fmt.Println("Pins file formatted.")
		return
	}

	if len(args) == 0 {
		pins, err := core.ListPinnedNotes()
		if err != nil {
			fmt.Println("Error loading pins:", err)
			return
		}
		if len(pins) == 0 {
			fmt.Println("No pinned notes.")
			return
		}
		fmt.Println("Pinned notes:")
		for _, title := range pins {
			fmt.Println(title)
		}
		return
	}

	noteName := strings.Join(args, " ")
	if err := core.PinNote(noteName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Pinned note:", noteName)
}

func UnpinCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gote unpin <note name>")
		return
	}
	noteName := strings.Join(args, " ")
	if err := core.UnpinNote(noteName); err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Unpinned note:", noteName)
}

func PinnedCommand(args []string) {
	pins, err := core.ListPinnedNotes()
	if err != nil {
		fmt.Println("Error loading pins:", err)
		return
	}
	if len(pins) == 0 {
		fmt.Println("No pinned notes.")
		return
	}
	fmt.Println("Pinned notes:")
	for _, title := range pins {
		fmt.Println(title)
	}
}