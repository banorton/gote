package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	NoteDir string
	Editor  string
}

func saveConfig(cfg Config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(configPath())
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(cfg)
}

func loadConfig() (Config, error) {
	var cfg Config
	f, err := os.Open(configPath())
	if err != nil {
		return cfg, err
	}
	defer f.Close()
	return cfg, json.NewDecoder(f).Decode(&cfg)
}

func configPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".gote", "config.json")
}

func main() {
	args := os.Args
	if len(args) == 1 {
		// Default behavior: no arguments, just 'gote'
		note()
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
	default:
		// If not a recognized command, treat as note
		note()
	}
}

func note() {

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

func isReservedWord(arg string) bool {
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
