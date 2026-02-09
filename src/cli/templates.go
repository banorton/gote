package cli

import (
	"fmt"
	"path/filepath"

	"gote/src/core"
	"gote/src/data"
)

// TemplateCommand handles template subcommands
func TemplateCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	switch sub {
	case "", "list":
		templateMenu(ui, cfg)
	case "delete":
		rest := args.Rest()
		if len(rest) == 0 {
			fmt.Println("Usage: gote template delete <name>")
			return
		}
		name := rest[0]
		if err := data.DeleteTemplate(name); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Deleted template: " + name)
	default:
		// Create or edit template
		if err := core.CreateOrEditTemplate(sub); err != nil {
			ui.Error(err.Error())
			return
		}
	}
}

func templateMenu(ui *UI, cfg data.Config) {
	templates, err := core.ListTemplates()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if len(templates) == 0 {
		ui.Empty("No templates. Create one with: gote template <name>")
		return
	}

	// Build paths map
	paths := make(map[string]string)
	for _, t := range templates {
		paths[t] = filepath.Join(data.TemplatesDir(), t+".md")
	}

	result := displayMenu(MenuConfig{
		Title:     "Templates",
		Items:     templates,
		ItemPaths: paths,
		HideView:  true,
		PageSize:  cfg.PageSize(),
	}, ui, cfg.FancyUI)

	executeTemplateAction(result, ui)
}

func executeTemplateAction(result MenuResult, ui *UI) {
	if result.Note == "" || result.Action == "" {
		return
	}

	switch result.Action {
	case "open":
		if err := core.CreateOrEditTemplate(result.Note); err != nil {
			ui.Error(err.Error())
		}
	case "delete":
		fmt.Printf("Delete template \"%s\"? [y/n]: ", result.Note)
		confirm := ui.ReadMenuInput()
		if confirm != "y" {
			ui.Info("Cancelled")
			return
		}
		if err := data.DeleteTemplate(result.Note); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Deleted template: " + result.Note)
	case "rename":
		newName := ui.ReadInputWithDefault("New name: ", result.Note)
		if newName == "" || newName == result.Note {
			ui.Info("Cancelled")
			return
		}
		if err := data.RenameTemplate(result.Note, newName); err != nil {
			ui.Error(err.Error())
			return
		}
		ui.Success("Renamed to: " + newName)
	case "info":
		// Show template path
		path := filepath.Join(data.TemplatesDir(), result.Note+".md")
		ui.InfoBox(result.Note, [][2]string{
			{"Path", path},
		})
	}
}

// selectTemplate shows an interactive picker and returns the selected template name
func selectTemplate(cfg data.Config, ui *UI, pageSize int) string {
	templates, err := core.ListTemplates()
	if err != nil {
		ui.Error(err.Error())
		return ""
	}
	if len(templates) == 0 {
		ui.Empty("No templates. Create one with: gote template <name>")
		return ""
	}

	paths := make(map[string]string)
	for _, t := range templates {
		paths[t] = filepath.Join(data.TemplatesDir(), t+".md")
	}

	if pageSize <= 0 {
		pageSize = cfg.PageSize()
	}

	result := displayMenu(MenuConfig{
		Title:             "Select Template",
		Items:             templates,
		ItemPaths:         paths,
		PreSelectedAction: "open",
		HideView:          true,
		PageSize:          pageSize,
	}, ui, cfg.FancyUI)

	return result.Note
}
