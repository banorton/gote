package fuzzy

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type Mode int

const (
	ModeNotes Mode = iota
	ModeContent
)

const previewMinWidth = 100

// Item is one selectable row. Search is the string that is matched and
// highlighted; Suffix is shown dim after it. Line is 0 for note rows.
type Item struct {
	ID     string
	Line   int
	Search string
	Suffix string
}

type Config struct {
	Notes   func() ([]Item, error)
	Lines   func() ([]Item, error)
	Preview func(id string, line int) (string, error)
	// Invoke runs a registered command by name ("open", "view", "delete", "pin").
	Invoke func(name string, args []string) error
	// Edit opens id at a line in the configured editor; only used when Editor supports it.
	Edit   func(id string, line int) error
	Editor string
	// Exec runs fn with the terminal released to it. Defaults to tea.Exec.
	Exec func(fn func() error, done func(error) tea.Msg) tea.Cmd
}

type doneMsg struct {
	label    string
	err      error
	removeID string
}

type Model struct {
	cfg     Config
	notes   []Item
	lines   []Item
	linesOK bool
	mode    Mode
	pattern string
	results []Result
	cursor  int
	offset  int
	width   int
	height  int
	status  string
	confirm *Item
	preview string
	pvKey   string
	quit    bool
}

func New(cfg Config) Model {
	if cfg.Exec == nil {
		cfg.Exec = defaultExec
	}
	m := Model{cfg: cfg, width: 80, height: 24}
	if cfg.Notes != nil {
		items, err := cfg.Notes()
		if err != nil {
			m.status = "load notes: " + err.Error()
		}
		m.notes = items
	}
	m.refilter()
	m.syncPreview()
	return m
}

func defaultExec(fn func() error, done func(error) tea.Msg) tea.Cmd {
	return tea.Exec(&execAdapter{fn: fn}, func(err error) tea.Msg { return done(err) })
}

func Run(cfg Config) error {
	_, err := tea.NewProgram(New(cfg), tea.WithAltScreen()).Run()
	return err
}

// SupportsLine reports whether the editor command takes a +<line> argument.
func SupportsLine(editor string) bool {
	f := strings.Fields(editor)
	if len(f) == 0 {
		return false
	}
	switch filepath.Base(f[0]) {
	case "vi", "vim", "nvim", "nano":
		return true
	}
	return false
}

func (m Model) Pattern() string { return m.pattern }
func (m Model) Cursor() int     { return m.cursor }
func (m Model) Mode() Mode      { return m.mode }
func (m Model) Status() string  { return m.status }

func (m Model) items() []Item {
	if m.mode == ModeContent {
		return m.lines
	}
	return m.notes
}

func (m Model) Visible() []Item {
	src := m.items()
	out := make([]Item, len(m.results))
	for i, r := range m.results {
		out[i] = src[r.Index]
	}
	return out
}

func (m *Model) refilter() {
	src := m.items()
	strs := make([]string, len(src))
	for i, it := range src {
		strs[i] = it.Search
	}
	m.results = Match(m.pattern, strs)
	m.cursor, m.offset = 0, 0
}

func (m Model) selected() (Item, bool) {
	if m.cursor < 0 || m.cursor >= len(m.results) {
		return Item{}, false
	}
	return m.items()[m.results[m.cursor].Index], true
}

func (m Model) previewOn() bool { return m.width >= previewMinWidth && m.cfg.Preview != nil }

func (m *Model) syncPreview() {
	it, ok := m.selected()
	if !ok || !m.previewOn() {
		m.preview, m.pvKey = "", ""
		return
	}
	key := fmt.Sprintf("%s:%d", it.ID, it.Line)
	if key == m.pvKey {
		return
	}
	text, err := m.cfg.Preview(it.ID, it.Line)
	if err != nil {
		text = "preview: " + err.Error()
	}
	m.preview, m.pvKey = text, key
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) listHeight() int {
	h := m.height - 3
	if h < 0 {
		return 0
	}
	return h
}

func (m *Model) move(d int) {
	m.cursor += d
	if m.cursor >= len(m.results) {
		m.cursor = len(m.results) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	h := m.listHeight()
	if h > 0 {
		if m.cursor < m.offset {
			m.offset = m.cursor
		}
		if m.cursor >= m.offset+h {
			m.offset = m.cursor - h + 1
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.move(0)
	case doneMsg:
		m = m.handleDone(msg)
	case tea.KeyMsg:
		m, cmd = m.handleKey(msg)
	}
	m.syncPreview()
	return m, cmd
}

func (m Model) handleDone(d doneMsg) Model {
	if d.err != nil {
		m.status = d.err.Error()
		return m
	}
	m.status = ""
	if d.label == "pin" {
		m.status = "pin: done"
	}
	if d.removeID != "" {
		m.status = d.label + ": done"
		m.notes = without(m.notes, d.removeID)
		m.lines = without(m.lines, d.removeID)
		cur := m.cursor
		m.refilter()
		m.move(cur)
	}
	return m
}

func without(items []Item, id string) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		if it.ID != id {
			out = append(out, it)
		}
	}
	return out
}

func (m Model) handleKey(k tea.KeyMsg) (Model, tea.Cmd) {
	if m.confirm != nil {
		it := *m.confirm
		m.confirm = nil
		m.status = ""
		if k.String() == "y" {
			return m, m.run("delete", []string{it.ID}, false, it.ID)
		}
		return m, nil
	}
	switch k.String() {
	case "esc", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	case "up", "ctrl+k":
		m.move(-1)
	case "down", "ctrl+j":
		m.move(1)
	case "ctrl+t":
		m = m.toggleMode()
	case "enter":
		return m.open()
	case "ctrl+v":
		if it, ok := m.selected(); ok {
			return m, m.run("view", []string{it.ID}, true, "")
		}
	case "ctrl+p":
		if it, ok := m.selected(); ok {
			return m, m.run("pin", []string{it.ID}, false, "")
		}
	case "ctrl+d":
		if it, ok := m.selected(); ok {
			m.confirm = &it
			m.status = fmt.Sprintf("delete %s? y/n", it.ID)
		}
	case "backspace":
		if r := []rune(m.pattern); len(r) > 0 {
			m.pattern = string(r[:len(r)-1])
			m.refilter()
		}
	case "ctrl+u":
		m.pattern = ""
		m.refilter()
	case "ctrl+w":
		m.pattern = strings.TrimRight(m.pattern, " ")
		m.pattern = m.pattern[:strings.LastIndex(m.pattern, " ")+1]
		m.refilter()
	default:
		switch {
		case k.Type == tea.KeySpace:
			m.pattern += " "
			m.refilter()
		case k.Type == tea.KeyRunes && !k.Alt:
			m.pattern += string(k.Runes)
			m.refilter()
		}
	}
	return m, nil
}

func (m Model) toggleMode() Model {
	if m.mode == ModeNotes {
		if !m.linesOK {
			if m.cfg.Lines == nil {
				m.status = "content mode not available"
				return m
			}
			items, err := m.cfg.Lines()
			if err != nil {
				m.status = "load content: " + err.Error()
				return m
			}
			m.lines, m.linesOK = items, true
		}
		m.mode = ModeContent
	} else {
		m.mode = ModeNotes
	}
	m.status = ""
	m.refilter()
	return m
}

func (m Model) open() (Model, tea.Cmd) {
	it, ok := m.selected()
	if !ok {
		return m, nil
	}
	if m.mode == ModeContent && it.Line > 0 && m.cfg.Edit != nil && SupportsLine(m.cfg.Editor) {
		fn := func() error { return m.cfg.Edit(it.ID, it.Line) }
		return m, m.cfg.Exec(fn, func(err error) tea.Msg { return doneMsg{label: "open", err: err} })
	}
	return m, m.run("open", []string{it.ID}, true, "")
}

func (m Model) run(name string, args []string, interactive bool, removeID string) tea.Cmd {
	invoke := m.cfg.Invoke
	if invoke == nil {
		invoke = func(name string, _ []string) error { return fmt.Errorf("%s: not available", name) }
	}
	fn := func() error { return invoke(name, args) }
	done := func(err error) tea.Msg { return doneMsg{label: name, err: err, removeID: removeID} }
	if interactive {
		return m.cfg.Exec(fn, done)
	}
	return func() tea.Msg { return done(fn()) }
}
