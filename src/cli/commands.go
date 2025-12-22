package cli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gote/src/core"
	"gote/src/data"
)

func NoteCommand(args []string) {
	noteName := strings.Join(args, " ")
	if err := core.CreateOrOpenNote(noteName); err != nil {
		fmt.Println("Error:", err)
	}
}

func QuickCommand() {
	NoteCommand([]string{"quick"})
}

func IndexCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	switch sub {
	case "":
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			return
		}
		if err := data.IndexNotes(cfg.NoteDir); err != nil {
			fmt.Println("Error indexing notes:", err)
		} else {
			fmt.Println("All notes indexed.")
		}
	case "edit":
		if err := data.FormatIndexFile(); err != nil {
			fmt.Println("Error trying to format index file. Got:", err.Error())
		}
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			return
		}
		if err := data.OpenFileInEditor(data.IndexPath(), cfg.Editor); err != nil {
			fmt.Println("Error opening index in editor:", err)
		}
	case "format":
		if err := data.FormatIndexFile(); err != nil {
			fmt.Println("Error formatting index:", err)
			return
		}
		fmt.Println("Index file formatted.")
	default:
		fmt.Println("Unknown subcommand:", sub)
		fmt.Println("Usage: gote index [edit|format]")
	}
}

func TagsCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	switch sub {
	case "":
		tags, err := core.ListTags()
		if err != nil {
			fmt.Println("Could not read tags file:", err)
			return
		}
		for tagName, tag := range tags {
			fmt.Printf("%s (%d)\n", tagName, tag.Count)
		}
	case "edit":
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			return
		}
		if err := data.OpenFileInEditor(data.TagsPath(), cfg.Editor); err != nil {
			fmt.Println(err)
		}
	case "format":
		if err := data.FormatTagsFile(); err != nil {
			fmt.Println("Error formatting tags:", err)
			return
		}
		fmt.Println("Tags file formatted.")
	case "popular":
		n := args.IntOr(10, "n", "limit")
		// Also support bare number: "gote tags popular 5"
		if n == 10 && len(args.Rest()) > 0 {
			if v, err := strconv.Atoi(args.Rest()[0]); err == nil && v > 0 {
				n = v
			}
		}
		tags, err := core.GetPopularTags(n)
		if err != nil {
			fmt.Println("Error getting popular tags:", err)
			return
		}
		fmt.Printf("Top %d tags by usage:\n", len(tags))
		for _, tag := range tags {
			fmt.Printf("%s (%d)\n", tag.Tag, tag.Count)
		}
	default:
		fmt.Println("Unknown subcommand:", sub)
		fmt.Println("Usage: gote tags [edit|format|popular]")
	}
}

func TagCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()
	tagsToAdd := args.List("t", "tags")

	if noteName == "" || len(tagsToAdd) == 0 {
		fmt.Println("Usage: gote tag <note name> -t <tag1> <tag2> ...")
		return
	}

	if err := core.AddTagsToNote(noteName, tagsToAdd); err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Tags updated for note:", noteName)
}

func ConfigCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	switch sub {
	case "", "show":
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err.Error())
			return
		}
		fmt.Println("Config settings:")
		data.PrettyPrintJSON(cfg)
	case "edit":
		configPath := filepath.Join(data.GoteDir(), "config.json")
		if err := data.OpenFileInEditor(configPath, "vim"); err != nil {
			fmt.Println(err)
		}
	case "format":
		if err := data.FormatConfigFile(); err != nil {
			fmt.Println("Error formatting config:", err)
			return
		}
		fmt.Println("Config file formatted.")
	default:
		fmt.Println("Unknown subcommand:", sub)
		fmt.Println("Usage: gote config [edit|format|show]")
	}
}
