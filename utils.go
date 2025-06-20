package main

import (
	"fmt"
	"encoding/json"
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

func prettyPrintJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Println(v)
		return
	}
	fmt.Println(string(data))
}

