package main

import (
	"fmt"
	"os"
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
