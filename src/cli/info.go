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

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	if oldName == "" || newName == "" {
		ui.Info("Usage: gote rename <note name> -n <new name>")
		return
	}

	oldName, err := ResolveNoteName(oldName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	if err := core.RenameNote(oldName, newName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success(fmt.Sprintf("Renamed '%s' to '%s'", oldName, newName))
}

func DuplicateCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	if noteName == "" {
		ui.Info("Usage: gote duplicate <note name>")
		return
	}

	noteName, err := ResolveNoteName(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	newName := ui.ReadInputWithDefault("New name: ", noteName+" (copy)")
	if newName == "" {
		ui.Info("Cancelled")
		return
	}

	if err := core.DuplicateNote(noteName, newName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Duplicated to: " + newName)
}

func InfoCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	if noteName == "" {
		ui.Info("Usage: gote info <note name>")
		return
	}

	noteName, err := ResolveNoteName(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	meta, err := core.GetNoteInfo(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	if cfg.IsTUI() {
		kvPairs := [][2]string{
			{"Path", meta.FilePath},
			{"Created", meta.Created},
			{"Words", fmt.Sprintf("%d", meta.WordCount)},
			{"Chars", fmt.Sprintf("%d", meta.CharCount)},
		}
		if len(meta.Tags) > 0 {
			kvPairs = append(kvPairs, [2]string{"Tags", strings.Join(meta.Tags, ", ")})
		}
		ui.InfoBox(meta.Title, kvPairs)
	} else {
		b, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			fmt.Println("Error marshaling note metadata:", err)
			return
		}
		fmt.Println(string(b))
	}
}
