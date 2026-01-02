package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gote/src/core"
	"gote/src/data"
)

func NoteCommand(args []string) {
	parsedArgs := ParseArgs(args)
	dateFlag := parsedArgs.Has("d", "date")
	datetimeFlag := parsedArgs.Has("dt", "datetime")
	noTimestampFlag := parsedArgs.Has("nt", "no-timestamp")
	templateFlag := parsedArgs.Has("t", "template")
	templateName := parsedArgs.String("t", "template")
	noteName := parsedArgs.Joined()

	if noteName == "" {
		fmt.Println("Usage: gote <note name> [-d|--date] [-dt|--datetime] [-nt|--no-timestamp] [-t|--template <name>]")
		return
	}

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	// Check if note already exists - if so, just open it
	index, err := data.LoadIndex()
	if err != nil {
		fmt.Println("Error loading index:", err)
		return
	}
	if _, exists := index[noteName]; exists {
		if err := core.CreateOrOpenNote(noteName); err != nil {
			fmt.Println("Error:", err)
		}
		return
	}

	// Note doesn't exist - apply timestamp if enabled (unless bypassed)
	if !noTimestampFlag {
		mode := cfg.TimestampNotes
		if dateFlag {
			mode = "date"
		} else if datetimeFlag {
			mode = "datetime"
		}

		if mode == "date" {
			noteName = time.Now().Format("060102") + " " + noteName
		} else if mode == "datetime" {
			noteName = time.Now().Format("060102-150405") + " " + noteName
		}
	}

	// Handle template flag
	if templateFlag {
		if templateName == "" {
			// Show template picker
			templateName = selectTemplate(cfg, ui, cfg.PageSize())
			if templateName == "" {
				return // User cancelled
			}
		}
		if err := core.CreateNoteFromTemplate(noteName, templateName); err != nil {
			ui.Error(err.Error())
		}
		return
	}

	if err := core.CreateOrOpenNote(noteName); err != nil {
		fmt.Println("Error:", err)
	}
}

func QuickCommand() {
	NoteCommand([]string{"quick"})
}

func QuickSaveCommand(args []string) {
	parsedArgs := ParseArgs(args)
	noteName := parsedArgs.Joined()

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	if noteName == "" {
		fmt.Println("Usage: gote quick save <note name>")
		fmt.Println("       gote qs <note name>")
		return
	}

	if err := core.PromoteQuickNote(noteName); err != nil {
		ui.Error(err.Error())
		return
	}
	ui.Success("Quick note saved as: " + noteName)
}

func IndexCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

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

func TagCommand(rawArgs []string, defaultOpen bool, defaultDelete bool, defaultPin bool, defaultView bool) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	openMode := defaultOpen
	deleteMode := defaultDelete
	pinMode := defaultPin
	viewMode := defaultView

	// Check for mode keywords as first positional arg (e.g., "gote tag open .work")
	if sub == "open" {
		openMode = true
		args.Positional = args.Positional[1:]
		sub = args.First()
	} else if sub == "delete" {
		deleteMode = true
		args.Positional = args.Positional[1:]
		sub = args.First()
	} else if sub == "pin" {
		pinMode = true
		args.Positional = args.Positional[1:]
		sub = args.First()
	} else if sub == "view" {
		viewMode = true
		args.Positional = args.Positional[1:]
		sub = args.First()
	}

	// If first arg starts with ".", it's a tag filter
	if strings.HasPrefix(sub, ".") {
		// Collect all tag strings from positional args
		var allTags []string
		for _, arg := range args.Positional {
			if strings.HasPrefix(arg, ".") {
				allTags = append(allTags, ParseTagString(arg)...)
			}
		}
		if len(allTags) == 0 {
			ui.Empty("No valid tags specified.")
			return
		}

		pageSize := args.IntOr(cfg.PageSize(), "n", "limit")
		results, err := core.FilterNotesByTags(allTags, -1)
		if err != nil {
			ui.Error(err.Error())
			return
		}
		if len(results) == 0 {
			ui.Empty("No notes found with all specified tags.")
			return
		}
		displayPaginatedSearchResultsWithMode(results, openMode || deleteMode || pinMode || viewMode, deleteMode, pinMode, viewMode, pageSize)
		return
	}

	// Handle subcommands
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
		// Also support bare number: "gote tag popular 5"
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
		fmt.Println("Usage: gote tag [.tag1.tag2 | edit | format | popular]")
	}
}


func ConfigCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

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
