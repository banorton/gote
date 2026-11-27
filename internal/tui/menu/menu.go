// Package menu is the paginated home-row selection menu. Items are picked with
// a single key; an uppercase letter (the first letter of an action's name) or
// Tab switches the action applied to the pick.
package menu

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	ErrCancelled = errors.New("cancelled")
	ErrNotTTY    = errors.New("interactive menu needs a terminal")
)

// Item is one menu entry. Detail follows the title on the same row; any lines
// after its first are shown indented beneath the entry.
type Item struct {
	ID, Title, Detail string
}

// Action names what happens to the picked item, e.g. "open" or "delete".
type Action string

const pageSize = 10

var homeRow = []rune("asdfghjkl;")

type model struct {
	items   []Item
	actions []Action
	action  Action
	page    int
	done    bool
	chosen  Item
	err     error
}

func newModel(items []Item, initial Action, actions []Action) model {
	return model{items: items, actions: actions, action: initial}
}

func (m model) pages() int { return (len(m.items) + pageSize - 1) / pageSize }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyEsc, tea.KeyCtrlC:
		return m.finish(ErrCancelled)
	case tea.KeyTab:
		for i, a := range m.actions {
			if a == m.action {
				m.action = m.actions[(i+1)%len(m.actions)]
				break
			}
		}
		return m, nil
	case tea.KeyRunes:
		if len(km.Runes) != 1 {
			return m, nil
		}
		r := km.Runes[0]
		switch {
		case r == 'q':
			return m.finish(ErrCancelled)
		case r == 'n':
			if m.page < m.pages()-1 {
				m.page++
			}
			return m, nil
		case r == 'p':
			if m.page > 0 {
				m.page--
			}
			return m, nil
		case unicode.IsUpper(r):
			for _, a := range m.actions {
				if unicode.ToUpper([]rune(string(a))[0]) == r {
					m.action = a
				}
			}
			return m, nil
		}
		for i, k := range homeRow {
			if k != r {
				continue
			}
			idx := m.page*pageSize + i
			if idx < len(m.items) && i < pageSize {
				m.chosen, m.done = m.items[idx], true
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m model) finish(err error) (tea.Model, tea.Cmd) {
	m.done, m.err = true, err
	return m, tea.Quit
}

func (m model) View() string {
	var sb strings.Builder
	start := m.page * pageSize
	end := min(start+pageSize, len(m.items))
	for i, it := range m.items[start:end] {
		fmt.Fprintf(&sb, " [%c] %s", homeRow[i], it.Title)
		first, rest, _ := strings.Cut(it.Detail, "\n")
		if first != "" {
			fmt.Fprintf(&sb, "  %s", first)
		}
		sb.WriteByte('\n')
		if rest != "" {
			for _, l := range strings.Split(rest, "\n") {
				fmt.Fprintf(&sb, "     %s\n", l)
			}
		}
	}
	fmt.Fprintf(&sb, "\n (%d/%d) [n]ext [p]rev [q]uit\n", m.page+1, m.pages())
	var modes []string
	for _, a := range m.actions {
		name := string(a)
		label := strings.ToUpper(name[:1]) + name[1:]
		if a == m.action {
			label = "*" + label + "*"
		}
		modes = append(modes, label)
	}
	fmt.Fprintf(&sb, " action: %s  (shift+letter or tab to switch)\n", strings.Join(modes, " "))
	return sb.String()
}

// Run shows the menu and returns the picked item with the action in effect.
func Run(items []Item, initial Action, actions []Action) (Item, Action, error) {
	if len(items) == 0 {
		return Item{}, "", errors.New("menu: no items")
	}
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		st, err := f.Stat()
		if err != nil || st.Mode()&os.ModeCharDevice == 0 {
			return Item{}, "", ErrNotTTY
		}
	}
	final, err := tea.NewProgram(newModel(items, initial, actions)).Run()
	if err != nil {
		return Item{}, "", fmt.Errorf("menu: %w", err)
	}
	m := final.(model)
	if m.err != nil {
		return Item{}, "", m.err
	}
	return m.chosen, m.action, nil
}
