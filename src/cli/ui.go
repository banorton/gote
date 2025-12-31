package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// ANSI escape codes
const (
	Reset    = "\033[0m"
	Bold     = "\033[1m"
	Dim      = "\033[2m"
	Cyan     = "\033[36m"
	White    = "\033[37m"
	BoldCyan = "\033[1;36m"
	Reverse  = "\033[7m"
)

// Screen control
const (
	ClearScreen = "\033[2J"
	CursorHome  = "\033[H"
	CursorHide  = "\033[?25l"
	CursorShow  = "\033[?25h"
	ClearLine   = "\033[K"
)

// Box drawing characters
const (
	BoxTopLeft     = "╭"
	BoxTopRight    = "╮"
	BoxBottomLeft  = "╰"
	BoxBottomRight = "╯"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visibleLen returns the visible length of a string (excluding ANSI codes)
func visibleLen(s string) int {
	return len(ansiRegex.ReplaceAllString(s, ""))
}

// UI holds the UI state
type UI struct {
	Fancy bool
}

// NewUI creates a UI instance
func NewUI(fancy bool) *UI {
	return &UI{Fancy: fancy}
}

// Clear clears the screen and moves cursor home
func (u *UI) Clear() {
	if u.Fancy {
		fmt.Print(ClearScreen + CursorHome)
	}
}

// HideCursor hides the cursor
func (u *UI) HideCursor() {
	if u.Fancy {
		fmt.Print(CursorHide)
	}
}

// ShowCursor shows the cursor
func (u *UI) ShowCursor() {
	if u.Fancy {
		fmt.Print(CursorShow)
	}
}

// Box draws a box with title and content
func (u *UI) Box(title string, lines []string, width int) {
	if !u.Fancy {
		if title != "" {
			fmt.Println(title + ":")
		}
		for _, line := range lines {
			fmt.Println(line)
		}
		return
	}

	// Calculate width based on visible content
	if width == 0 {
		width = len(title) + 4
		for _, line := range lines {
			visible := visibleLen(line) + 4
			if visible > width {
				width = visible
			}
		}
	}
	if width < 20 {
		width = 20
	}

	// Top border with title
	if title != "" {
		titlePart := fmt.Sprintf(" %s ", title)
		remaining := width - len(titlePart) - 2
		left := remaining / 2
		right := remaining - left
		fmt.Printf("%s%s%s%s%s%s%s\n",
			Cyan, BoxTopLeft,
			strings.Repeat(BoxHorizontal, left),
			Reset+Bold+titlePart+Reset+Cyan,
			strings.Repeat(BoxHorizontal, right),
			BoxTopRight, Reset)
	} else {
		fmt.Printf("%s%s%s%s%s\n",
			Cyan, BoxTopLeft,
			strings.Repeat(BoxHorizontal, width-2),
			BoxTopRight, Reset)
	}

	// Content - pad based on visible width
	for _, line := range lines {
		visible := visibleLen(line)
		padding := width - visible - 4
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("%s%s%s %s%s %s%s%s\n",
			Cyan, BoxVertical, Reset,
			line, strings.Repeat(" ", padding),
			Cyan, BoxVertical, Reset)
	}

	// Bottom border
	fmt.Printf("%s%s%s%s%s\n",
		Cyan, BoxBottomLeft,
		strings.Repeat(BoxHorizontal, width-2),
		BoxBottomRight, Reset)
}

// SelectableList renders an interactive selectable list with box
func (u *UI) SelectableList(title string, items []string, selected int, keys []rune) {
	if !u.Fancy {
		if title != "" {
			fmt.Println(title + ":")
		}
		for i, item := range items {
			if i < len(keys) && keys[i] != 0 {
				fmt.Printf("[%c] %s\n", keys[i], item)
			} else {
				fmt.Println(item)
			}
		}
		return
	}

	// Calculate width based on actual item lengths
	width := len(title) + 4
	for _, item := range items {
		itemWidth := len(item) + 8 // account for " [x] " prefix and padding
		if itemWidth > width {
			width = itemWidth
		}
	}
	if width < 30 {
		width = 30
	}

	var lines []string
	for i, item := range items {
		key := rune(0)
		if i < len(keys) {
			key = keys[i]
		}

		var line string
		if i == selected {
			// Highlighted selection
			if key != 0 {
				line = fmt.Sprintf("%s%s [%c] %s %s", Reverse, White, key, item, Reset)
			} else {
				line = fmt.Sprintf("%s%s  *  %s %s", Reverse, White, item, Reset)
			}
		} else {
			if key != 0 {
				line = fmt.Sprintf(" %s[%c]%s %s", Dim, key, Reset, item)
			} else {
				line = fmt.Sprintf("  *  %s", item)
			}
		}
		lines = append(lines, line)
	}

	u.Box(title, lines, width+4)
}

// NavHint shows navigation hints
func (u *UI) NavHint(page, total int) {
	u.NavHintWithOpen(page, total, false)
}

// NavHintWithOpen shows navigation hints with optional [o]pen option
func (u *UI) NavHintWithOpen(page, total int, showOpen bool) {
	if !u.Fancy {
		fmt.Printf("(%d/%d) ", page, total)
		if total > 1 {
			fmt.Print("[n]ext [p]rev ")
		}
		if showOpen {
			fmt.Print("[o]pen ")
		}
		fmt.Println("[q]uit")
		return
	}

	var hints []string
	hints = append(hints, fmt.Sprintf("(%d/%d)", page, total))
	if total > 1 {
		hints = append(hints, "[n] next")
		hints = append(hints, "[p] prev")
	}
	if showOpen {
		hints = append(hints, "[o] open")
	}
	hints = append(hints, "[q] quit")

	fmt.Printf("\n %s%s%s\n", Dim, strings.Join(hints, "  "), Reset)
}

// Success prints a success message
func (u *UI) Success(text string) {
	if u.Fancy {
		fmt.Printf("%s->%s %s\n", Cyan, Reset, text)
	} else {
		fmt.Println(text)
	}
}

// Error prints an error message
func (u *UI) Error(text string) {
	if u.Fancy {
		fmt.Printf("%s!%s %s\n", Bold, Reset, text)
	} else {
		fmt.Println("Error:", text)
	}
}

// Info prints an info message
func (u *UI) Info(text string) {
	if u.Fancy {
		fmt.Printf("%s->%s %s\n", Cyan, Reset, text)
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

// Title prints a styled title
func (u *UI) Title(text string) {
	if u.Fancy {
		fmt.Printf("%s%s%s\n", BoldCyan, text, Reset)
	} else {
		fmt.Println(text)
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

// Tags prints multiple tags
func (u *UI) Tags(tags []string) {
	if len(tags) == 0 {
		return
	}
	if u.Fancy {
		fmt.Printf("  %sTags:%s %s\n", Cyan, Reset, strings.Join(tags, ", "))
	} else {
		fmt.Printf("Tags: %s\n", strings.Join(tags, ", "))
	}
}

// ListItem prints a list item (non-interactive)
func (u *UI) ListItem(key rune, text string, selected bool) {
	if u.Fancy {
		if key != 0 {
			fmt.Printf("  %s[%c]%s %s\n", Dim, key, Reset, text)
		} else {
			fmt.Printf("  %s*%s %s\n", Dim, Reset, text)
		}
	} else {
		if key != 0 {
			fmt.Printf("[%c] %s\n", key, text)
		} else {
			fmt.Println(text)
		}
	}
}

// ListItemWithMeta prints a list item with metadata
func (u *UI) ListItemWithMeta(key rune, text string, meta string) {
	if u.Fancy {
		if key != 0 {
			fmt.Printf("  %s[%c]%s %s %s%s%s\n", Dim, key, Reset, text, Dim, meta, Reset)
		} else {
			fmt.Printf("  %s*%s %s %s%s%s\n", Dim, Reset, text, Dim, meta, Reset)
		}
	} else {
		if key != 0 {
			fmt.Printf("[%c] %s %s\n", key, text, meta)
		} else {
			fmt.Printf("%s %s\n", text, meta)
		}
	}
}

// drainStdin discards any pending bytes in stdin (used to clear garbage from terminal init)
func drainStdin() {
	// Set stdin to non-blocking temporarily to drain any pending bytes
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	saved, err := cmd.Output()
	if err != nil {
		return
	}

	// Set non-blocking with 0 timeout
	cmd = exec.Command("stty", "-icanon", "min", "0", "time", "0")
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return
	}

	// Read and discard any pending bytes
	buf := make([]byte, 256)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			break
		}
	}

	// Restore original state
	cmd = exec.Command("stty", strings.TrimSpace(string(saved)))
	cmd.Stdin = os.Stdin
	cmd.Run()
}

// ReadKey reads a single keypress. In fancy mode, uses raw terminal (no Enter needed).
// In non-fancy mode, uses buffered input (requires Enter).
func ReadKey(fancy bool) (rune, error) {
	if !fancy {
		// Buffered input - requires Enter
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return 0, nil
		}
		return rune(input[0]), nil
	}

	// Drain any garbage bytes before reading (helps with WSL terminal quirks)
	drainStdin()

	// Raw mode - single keypress, no Enter needed
	if err := setRawMode(); err != nil {
		// Fallback to buffered if raw mode fails
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			return 0, nil
		}
		return rune(input[0]), nil
	}
	defer restoreTerminal()

	var buf [1]byte
	_, err := os.Stdin.Read(buf[:])
	if err != nil {
		return 0, err
	}
	return rune(buf[0]), nil
}

var originalSttyState string

func setRawMode() error {
	// Save current state
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	originalSttyState = strings.TrimSpace(string(out))

	// Set raw mode: -echo (no echo), -icanon (no line buffering), min 1 (read at least 1 char)
	cmd = exec.Command("stty", "-echo", "-icanon", "min", "1")
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func restoreTerminal() {
	if originalSttyState != "" {
		cmd := exec.Command("stty", originalSttyState)
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: failed to restore terminal:", err)
		}
	}
}

// InfoBox displays key-value info in a box
func (u *UI) InfoBox(title string, kvPairs [][2]string) {
	if !u.Fancy {
		if title != "" {
			fmt.Println(title)
		}
		for _, kv := range kvPairs {
			fmt.Printf("%s: %s\n", kv[0], kv[1])
		}
		return
	}

	// Find max key length for alignment
	maxKey := 0
	for _, kv := range kvPairs {
		if len(kv[0]) > maxKey {
			maxKey = len(kv[0])
		}
	}

	var lines []string
	for _, kv := range kvPairs {
		padding := strings.Repeat(" ", maxKey-len(kv[0]))
		line := fmt.Sprintf("%s%s%s:%s %s", Cyan, kv[0], padding, Reset, kv[1])
		lines = append(lines, line)
	}

	u.Box(title, lines, 0)
}
