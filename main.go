package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	args := os.Args

	if len(args) == 1 {
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
	case "config":
		config(args[2:])
	default:
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

func scratch() {

}

func config(args []string) {
	if len(args) > 0 && args[0] == "edit" {
		cfgPath := configPath()
		cfg, err := loadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			return
		}
		editor := cfg.Editor
		if editor == "" {
			editor = "vim"
		}
		cmd := exec.Command(editor, cfgPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("Error opening editor:", err)
		}
		return
	} else if len(args) > 0 && args[0] == "format" {
		err := formatConfigFile()
		if err != nil {
			fmt.Println("Error formatting config:", err)
			return
		}
		fmt.Println("Config file formatted.")
		return
	} else {
		cfg, err := loadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err.Error())
			return
		}
		fmt.Println("Config settings:")
		prettyPrintJSON(cfg)
	}
}
