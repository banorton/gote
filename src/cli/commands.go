package cli

import (
	"fmt"
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

func IndexCommand(args []string) {
	if len(args) == 0 {
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
		return
	}

	switch args[0] {
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
		fmt.Println("Error: gote index doesn't support arg:", args[0])
	}
}

func TagsCommand(args []string) {
	if len(args) == 0 {
		tags, err := core.ListTags()
		if err != nil {
			fmt.Println("Could not read tags file:", err)
			return
		}
		for tagName, tag := range tags {
			fmt.Printf("%s (%d)\n", tagName, tag.Count)
		}
		return
	}

	switch args[0] {
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
		n := 10
		if len(args) > 1 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
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
		fmt.Println("Error: gote tags doesn't support arg:", args[0])
	}
}

func TagCommand(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: gote tag <note name> -t <tag1> <tag2> ... <tagN>")
		return
	}

	tFlag := -1
	for i, arg := range args {
		if arg == "-t" {
			tFlag = i
			break
		}
	}
	if tFlag == -1 || tFlag == 0 || tFlag == len(args)-1 {
		fmt.Println("Usage: gote tag <note name> -t <tag1> <tag2> ... <tagN>")
		return
	}

	noteName := strings.Join(args[:tFlag], " ")
	tagsToAdd := args[tFlag+1:]

	if err := core.AddTagsToNote(noteName, tagsToAdd); err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Tags updated for note:", noteName)
}

func ConfigCommand(args []string) {
	if len(args) == 0 {
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err.Error())
			return
		}
		fmt.Println("Config settings:")
		data.PrettyPrintJSON(cfg)
		return
	}

	switch args[0] {
	case "edit":
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err)
			return
		}
		configPath := data.GoteDir() + "/config.json"
		if err := data.OpenFileInEditor(configPath, cfg.Editor); err != nil {
			fmt.Println(err)
		}
	case "format":
		if err := data.FormatConfigFile(); err != nil {
			fmt.Println("Error formatting config:", err)
			return
		}
		fmt.Println("Config file formatted.")
	default:
		cfg, err := data.LoadConfig()
		if err != nil {
			fmt.Println("Error loading config:", err.Error())
			return
		}
		fmt.Println("Config settings:")
		data.PrettyPrintJSON(cfg)
	}
}
