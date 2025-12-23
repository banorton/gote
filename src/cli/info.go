package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

func RenameCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	oldName := args.Joined()
	newName := strings.Join(args.List("n", "name"), " ")

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if oldName == "" || newName == "" {
		fmt.Println("Usage: gote rename <note name> -n <new name>")
		return
	}

	if err := core.RenameNote(oldName, newName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success(fmt.Sprintf("Renamed '%s' to '%s'", oldName, newName))
}

func InfoCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if noteName == "" {
		fmt.Println("Usage: gote info <note name>")
		return
	}

	meta, err := core.GetNoteInfo(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	if cfg.FancyUI {
		ui.Title(meta.Title)
		fmt.Println()
		ui.KeyValue("Path", meta.FilePath)
		ui.KeyValue("Created", meta.Created)
		ui.KeyValue("Words", fmt.Sprintf("%d", meta.WordCount))
		ui.KeyValue("Chars", fmt.Sprintf("%d", meta.CharCount))
		if len(meta.Tags) > 0 {
			ui.Tags(meta.Tags)
		}
	} else {
		b, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			fmt.Println("Error marshaling note metadata:", err)
			return
		}
		fmt.Println(string(b))
	}
}