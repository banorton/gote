package main

import (
	"fmt"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

func isReserved(arg string) bool {
	var reservedWords = map[string]struct{}{
		"delete": {}, "d": {},
		"index": {}, "x": {},
		"tags": {}, "t": {},
		"search": {}, "s": {},
		"recent": {}, "r": {},
		"pin": {}, "p": {},
		"unpin": {}, "u": {},
		"archive": {}, "a": {},
		"view": {}, "v": {},
		"lint": {}, "l": {},
		"config": {}, "c": {},
		"today": {}, "n": {},
		"links": {}, "k": {},
		"popular": {}, "z": {},
		"move": {}, "mv": {}, "m": {},
		"help": {}, "h": {},
		"pinned": {},
		"tag":    {},
		"info":   {}, "i": {},
		"trash":   {},
		"recover": {},
	}
	_, ok := reservedWords[arg]
	return ok
}

func goteDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".gote")
}

func prettyPrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(v)
		return
	}
	fmt.Println(string(data))
}

func openFileInEditor(filePath string) error {
	cfg, _ := loadConfig()
	editor := cfg.Editor
	if editor == "" {
		fmt.Println("No editor specified in config.")
	}

	cmd := exec.Command(editor, filePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error opening editor: %w", err)
	}
	return nil
}
