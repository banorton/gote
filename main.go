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
		config()
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

func config() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err.Error())
		return
	}
	fmt.Println(cfg)
}

