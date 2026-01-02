package cli

import (
	"fmt"

	"gote/src/core"
	"gote/src/data"
)

// TemplateCommand handles template subcommands
func TemplateCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	sub := args.First()

	cfg, err := data.LoadConfig()
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
	ui := NewUI(cfg.FancyUI)

	switch sub {
	case "", "list":
		listTemplates(ui, cfg)
	case "delete":
		rest := args.Rest()
		if len(rest) == 0 {
			fmt.Println("Usage: gote template delete <name>")
			return
		}
		name := rest[0]
		if name == "" {
			fmt.Println("Usage: gote template delete <name>")
			return
		}
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

func listTemplates(ui *UI, cfg data.Config) {
	templates, err := core.ListTemplates()
	if err != nil {
		ui.Error(err.Error())
		return
	}
	if len(templates) == 0 {
		ui.Empty("No templates. Create one with: gote template <name>")
		return
	}
	if cfg.FancyUI {
		ui.Box("Templates", templates, 0)
	} else {
		fmt.Println("Templates:")
		for _, t := range templates {
			fmt.Println("  " + t)
		}
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

	if pageSize <= 0 {
		pageSize = cfg.PageSize()
	}
	if pageSize > maxSelectablePageSize {
		pageSize = maxSelectablePageSize
	}

	page := 0
	totalPages := (len(templates) + pageSize - 1) / pageSize

	for {
		start := page * pageSize
		end := min(start+pageSize, len(templates))
		if start >= end {
			break
		}

		pageItems := templates[start:end]
		var keys []rune
		for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
			keys = append(keys, selectKeys[i])
		}

		if cfg.FancyUI {
			ui.Clear()
			ui.SelectableList("Select Template", pageItems, -1, keys)
			ui.NavHint(page+1, totalPages)
		} else {
			if page > 0 {
				fmt.Println()
			}
			fmt.Println("Select template:")
			for i, item := range pageItems {
				if i < len(selectKeys) {
					fmt.Printf("[%c] %s\n", selectKeys[i], item)
				} else {
					fmt.Println(item)
				}
			}
			fmt.Printf("(%d/%d) ", page+1, totalPages)
			if totalPages > 1 {
				fmt.Print("[n]ext [p]rev ")
			}
			fmt.Println("[q]uit")
			fmt.Print(": ")
		}

		key, err := ReadKey(cfg.FancyUI)
		if err != nil {
			return ""
		}

		switch key {
		case 'q', 'Q':
			if cfg.FancyUI {
				ui.Clear()
			}
			return ""
		case 'n', 'N':
			if page < totalPages-1 {
				page++
			}
			continue
		case 'p', 'P':
			if page > 0 {
				page--
			}
			continue
		}

		// Check selection keys
		for i := 0; i < len(pageItems) && i < len(selectKeys); i++ {
			if key == selectKeys[i] {
				if cfg.FancyUI {
					ui.Clear()
				}
				return templates[start+i]
			}
		}
	}
	return ""
}
