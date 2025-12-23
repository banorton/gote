package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"gote/src/core"
	"gote/src/data"
)

func NoteCommand(args []string) {
	parsedArgs := ParseArgs(args)
	dateFlag := parsedArgs.Has("d", "date")
	datetimeFlag := parsedArgs.Has("dt", "datetime")
	noteName := parsedArgs.Joined()

	if noteName == "" {
		fmt.Println("Usage: gote <note name> [-d|--date] [-dt|--datetime]")
		return
	}

	// Determine timestamp mode: flag > config > none
	cfg, _ := data.LoadConfig()
	mode := cfg.TimestampNotes
	if dateFlag {
		mode = "date"
	} else if datetimeFlag {
		mode = "datetime"
	}

	// Apply timestamp prefix
	if mode == "date" {
		noteName = time.Now().Format("060102") + " " + noteName
	} else if mode == "datetime" {
		noteName = time.Now().Format("060102-150405") + " " + noteName
	}

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

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	switch sub {
	case "":
		if err := data.IndexNotes(cfg.NoteDir); err != nil {
			ui.Error(err.Error())
		} else {
			ui.Success("All notes indexed.")
		}
	case "edit":
		if err := data.FormatIndexFile(); err != nil {
			ui.Error("Error trying to format index file: " + err.Error())
		}
		if err := data.OpenFileInEditor(data.IndexPath(), cfg.Editor); err != nil {
			ui.Error(err.Error())
		}
	case "format":
		if err := data.FormatIndexFile(); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Index file formatted.")
	case "clear":
		// Ensure terminal is in normal mode for input
		exec.Command("stty", "sane").Run()
		fmt.Print("This will delete and rebuild the index. Continue? [y/N]: ")
		var input string
		fmt.Scanln(&input)
		if input != "y" && input != "Y" {
			ui.Info("Cancelled.")
			return
		}
		os.Remove(data.IndexPath())
		os.Remove(data.TagsPath())
		if err := data.IndexNotes(cfg.NoteDir); err != nil {
			ui.Error(err.Error())
		} else {
			ui.Success("Index cleared and rebuilt.")
		}
	default:
		fmt.Println("Unknown subcommand:", sub)
		fmt.Println("Usage: gote index [edit|format|clear]")
	}
}

func TagsCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	switch sub {
	case "":
		tags, err := core.ListTags()
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(tags) == 0 {
			ui.Empty("No tags found.")
			return
		}
		var lines []string
		for tagName, tag := range tags {
			lines = append(lines, fmt.Sprintf("%s (%d)", tagName, tag.Count))
		}
		if cfg.FancyUI {
			ui.Box("Tags", lines, 0)
		} else {
			for _, line := range lines {
				fmt.Println(line)
			}
		}
	case "edit":
		if err := data.OpenFileInEditor(data.TagsPath(), cfg.Editor); err != nil {
			ui.Error(err.Error())
		}
	case "format":
		if err := data.FormatTagsFile(); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Tags file formatted.")
	case "popular":
		n := args.IntOr(cfg.PageSize(), "n", "limit")
		// Also support bare number: "gote tags popular 5"
		if n == cfg.PageSize() && len(args.Rest()) > 0 {
			if v, err := strconv.Atoi(args.Rest()[0]); err == nil && v > 0 {
				n = v
			}
		}
		tags, err := core.GetPopularTags(n)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(tags) == 0 {
			ui.Empty("No tags found.")
			return
		}
		var lines []string
		for _, tag := range tags {
			lines = append(lines, fmt.Sprintf("%s (%d)", tag.Tag, tag.Count))
		}
		if cfg.FancyUI {
			ui.Box(fmt.Sprintf("Top %d Tags", len(tags)), lines, 0)
		} else {
			fmt.Printf("Top %d tags:\n", len(tags))
			for _, line := range lines {
				fmt.Println(line)
			}
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

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	if noteName == "" || len(tagsToAdd) == 0 {
		fmt.Println("Usage: gote tag <note name> -t <tag1> <tag2> ...")
		return
	}

	if err := core.AddTagsToNote(noteName, tagsToAdd); err != nil {
		ui.Error(err.Error())
		return
	}

	ui.Success("Tags updated for note: " + noteName)
}

func ConfigCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, _ := data.LoadConfig()
	ui := NewUI(cfg.FancyUI)

	switch sub {
	case "", "show":
		if cfg.FancyUI {
			timestampVal := cfg.TimestampNotes
			if timestampVal == "" {
				timestampVal = "none"
			}
			ui.InfoBox("Config", [][2]string{
				{"Note directory", cfg.NoteDir},
				{"Editor", cfg.Editor},
				{"Fancy UI", fmt.Sprintf("%v", cfg.FancyUI)},
				{"Timestamp notes", timestampVal},
				{"Default page size", fmt.Sprintf("%d", cfg.PageSize())},
			})
		} else {
			fmt.Println("Config settings:")
			data.PrettyPrintJSON(cfg)
		}
	case "edit":
		configPath := filepath.Join(data.GoteDir(), "config.json")
		if err := data.OpenFileInEditor(configPath, "vim"); err != nil {
			ui.Error(err.Error())
		}
	case "format":
		if err := data.FormatConfigFile(); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Config file formatted.")
	case "help":
		fmt.Println(`Config file: ~/.gote/config.json

Options:
  noteDir          Directory where notes are stored
                   Default: ~/gotes

  editor           Editor to open notes with
                   Default: vim

  fancyUI          Enable TUI mode with boxes and single-keypress input
                   Values: true, false
                   Default: false

  timestampNotes   Auto-prefix new notes with timestamp
                   Values: "none", "date" (yymmdd), "datetime" (yymmdd-hhmmss)
                   Default: none (no prefix)
                   Can be overridden with -d or -dt flags

  defaultPageSize  Number of results to show by default
                   Default: 10
                   Can be overridden with -n flag`)
	default:
		fmt.Println("Unknown subcommand:", sub)
		fmt.Println("Usage: gote config [show|edit|format|help]")
	}
}
