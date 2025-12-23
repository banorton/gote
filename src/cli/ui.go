package cli

import (
	"fmt"
	"strings"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Cyan      = "\033[36m"
	Green     = "\033[32m"
	Yellow    = "\033[33m"
	Blue      = "\033[34m"
	Magenta   = "\033[35m"
	BoldCyan  = "\033[1;36m"
	BoldGreen = "\033[1;32m"
)

// Box drawing characters
const (
	BoxTopLeft     = "╭"
	BoxTopRight    = "╮"
	BoxBottomLeft  = "╰"
	BoxBottomRight = "╯"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxTeeRight    = "├"
	BoxTeeLeft     = "┤"
)

// UI holds the fancy UI state
type UI struct {
	Fancy bool
}

// NewUI creates a UI instance based on config
func NewUI(fancy bool) *UI {
	return &UI{Fancy: fancy}
}

// Title prints a styled title
func (u *UI) Title(text string) {
	if u.Fancy {
		fmt.Printf("%s%s%s\n", BoldCyan, text, Reset)
	} else {
		fmt.Println(text)
	}
}

// Header prints a section header with optional box
func (u *UI) Header(text string) {
	if u.Fancy {
		width := len(text) + 4
		fmt.Printf("%s%s%s%s\n", Cyan, BoxTopLeft, strings.Repeat(BoxHorizontal, width), BoxTopRight)
		fmt.Printf("%s  %s%s%s  %s\n", BoxVertical, Bold, text, Reset+Cyan, BoxVertical)
		fmt.Printf("%s%s%s%s\n", BoxBottomLeft, strings.Repeat(BoxHorizontal, width), BoxBottomRight, Reset)
	} else {
		fmt.Println(text + ":")
	}
}

// ListItem prints a list item with optional selection key
func (u *UI) ListItem(key rune, text string, selected bool) {
	if u.Fancy {
		if key != 0 {
			fmt.Printf("  %s[%s%c%s]%s %s\n", Dim, Yellow, key, Dim, Reset, text)
		} else {
			fmt.Printf("  %s•%s %s\n", Cyan, Reset, text)
		}
	} else {
		if key != 0 {
			fmt.Printf("[%c] %s\n", key, text)
		} else {
			fmt.Println(text)
		}
	}
}

// ListItemWithMeta prints a list item with additional metadata
func (u *UI) ListItemWithMeta(key rune, text string, meta string) {
	if u.Fancy {
		if key != 0 {
			fmt.Printf("  %s[%s%c%s]%s %s %s%s%s\n", Dim, Yellow, key, Dim, Reset, text, Dim, meta, Reset)
		} else {
			fmt.Printf("  %s•%s %s %s%s%s\n", Cyan, Reset, text, Dim, meta, Reset)
		}
	} else {
		if key != 0 {
			fmt.Printf("[%c] %s %s\n", key, text, meta)
		} else {
			fmt.Printf("%s %s\n", text, meta)
		}
	}
}

// Nav prints the navigation bar
func (u *UI) Nav(page, total int) {
	if u.Fancy {
		fmt.Println()
		fmt.Printf("%s(%s%d%s/%s%d%s)%s\n",
			Dim, Yellow, page, Dim, Yellow, total, Dim, Reset)
		fmt.Printf("%s[%sn%s]%s next  %s[%sp%s]%s prev  %s[%sq%s]%s quit\n",
			Dim, Green, Dim, Reset,
			Dim, Green, Dim, Reset,
			Dim, Green, Dim, Reset)
		fmt.Printf("%s:%s ", Cyan, Reset)
	} else {
		fmt.Printf("\n(%d/%d)\n[n] next [p] prev [q] quit\n: ", page, total)
	}
}

// Success prints a success message
func (u *UI) Success(text string) {
	if u.Fancy {
		fmt.Printf("%s✓%s %s\n", Green, Reset, text)
	} else {
		fmt.Println(text)
	}
}

// Error prints an error message
func (u *UI) Error(text string) {
	if u.Fancy {
		fmt.Printf("%s✗%s %s\n", Yellow, Reset, text)
	} else {
		fmt.Println("Error:", text)
	}
}

// Info prints an info message
func (u *UI) Info(text string) {
	if u.Fancy {
		fmt.Printf("%s→%s %s\n", Cyan, Reset, text)
	} else {
		fmt.Println(text)
	}
}

// Empty prints an empty state message
func (u *UI) Empty(text string) {
	if u.Fancy {
		fmt.Printf("%s%s%s\n", Dim, text, Reset)
	} else {
		fmt.Println(text)
	}
}

// Divider prints a horizontal divider
func (u *UI) Divider() {
	if u.Fancy {
		fmt.Printf("%s%s%s\n", Dim, strings.Repeat(BoxHorizontal, 40), Reset)
	}
}

// KeyValue prints a key-value pair
func (u *UI) KeyValue(key, value string) {
	if u.Fancy {
		fmt.Printf("  %s%s:%s %s\n", Cyan, key, Reset, value)
	} else {
		fmt.Printf("%s: %s\n", key, value)
	}
}

// Tag prints a styled tag
func (u *UI) Tag(tag string) string {
	if u.Fancy {
		return fmt.Sprintf("%s#%s%s", Magenta, tag, Reset)
	}
	return tag
}

// Tags prints multiple tags
func (u *UI) Tags(tags []string) {
	if len(tags) == 0 {
		return
	}
	if u.Fancy {
		var styled []string
		for _, t := range tags {
			styled = append(styled, fmt.Sprintf("%s#%s%s", Magenta, t, Reset))
		}
		fmt.Printf("  %sTags:%s %s\n", Cyan, Reset, strings.Join(styled, " "))
	} else {
		fmt.Printf("Tags: %s\n", strings.Join(tags, ", "))
	}
}
