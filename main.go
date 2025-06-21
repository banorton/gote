package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	args := os.Args

	if len(args) == 1 {
		note(nil)
		return
	}

	switch args[1] {
	case "recent":
		recent()
	case "popular":
		popular()
	case "index":
		index()
	case "tags":
		tags()
	case "tag":
		tag()
	case "config":
		config(args[2:])
	default:
		note(args[1:])
	}
}

func note(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: gote <note-name>")
		return
	}

	noteName := args[0]
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	noteDir := cfg.NoteDir
	if noteDir == "" {
		noteDir = defaultConfig().NoteDir
	}

	if err := os.MkdirAll(noteDir, 0755); err != nil {
		fmt.Println("Error creating notes directory:", err)
		return
	}

	notePath := filepath.Join(noteDir, noteName+".md")
	if _, err := os.Stat(notePath); os.IsNotExist(err) {
		f, err := os.Create(notePath)
		if err != nil {
			fmt.Println("Error creating note:", err)
			return
		}
		f.Close()
	}

	editor := cfg.Editor
	if editor == "" {
		fmt.Println("No editor specified in config.")
	}

	if err := openFileInEditor(editor, notePath), err != nil {
		fmt.Println(err)
	}
}

func index() {

}

func recent() {

}

func popular() {

}

func tags() {

}

func tag() {

}

func scratch() {

}

func config(args []string) {
	if len(args) == 0 {
		printConfig()
		return
	}

	switch args[0] {
	case "edit":
		openConfigInEditor()
	case "format":
		err := formatConfigFile()
		if err != nil {
			fmt.Println("Error formatting config:", err)
			return
		}
		fmt.Println("Config file formatted.")
	default:
		printConfig()
	}
}
