package main

import (
	"os"

	"github.com/banorton/gote/internal/cli"
)

func main() { os.Exit(cli.Main(os.Args[1:])) }
