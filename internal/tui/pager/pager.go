// Package pager is a scrollable terminal reader with link navigation.
package pager

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type Link struct {
	Label  string
	Target string
}

// Doc is one rendered page: Ref identifies it to the editor, Text is
// ANSI-styled output, and Links are in reading order.
type Doc struct {
	Ref   string
	Title string
	Text  string
	Links []Link
}

// Loader resolves a link target to the document it points at.
type Loader func(target string) (Doc, error)

type frame struct {
	doc    Doc
	offset int
	sel    int
}

type Model struct {
	doc       Doc
	lines     []string
	linkLines []int
	offset    int
	sel       int
	history   []frame
	load      Loader
	width     int
	height    int
	msg       string
	edit      string
}

func New(start Doc, load Loader) *Model {
	m := &Model{load: load, sel: -1, height: 24, width: 80}
	m.setDoc(start)
	return m
}

func (m *Model) setDoc(d Doc) {
	m.doc = d
	m.lines = strings.Split(strings.TrimRight(d.Text, "\n"), "\n")
	m.linkLines = make([]int, len(d.Links))
	at := 0
	for i, l := range d.Links {
		for j := at; j < len(m.lines); j++ {
			if strings.Contains(ansi.Strip(m.lines[j]), l.Label) {
				at = j
				break
			}
		}
		m.linkLines[i] = at
	}
}

func (m *Model) Doc() Doc            { return m.doc }
func (m *Model) Offset() int         { return m.offset }
func (m *Model) Selected() int       { return m.sel }
func (m *Model) Depth() int          { return len(m.history) }
func (m *Model) EditRequest() string { return m.edit }

func (m *Model) viewport() int {
	if m.height < 2 {
		return 1
	}
	return m.height - 1
}

func (m *Model) clamp() {
	if max := len(m.lines) - m.viewport(); m.offset > max {
		m.offset = max
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *Model) reveal(line int) {
	vp := m.viewport()
	if line >= m.offset && line < m.offset+vp {
		return
	}
	m.offset = line - vp/3
	m.clamp()
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
	case tea.KeyMsg:
		m.msg = ""
		return m, m.key(msg)
	}
	return m, nil
}

func (m *Model) key(k tea.KeyMsg) tea.Cmd {
	switch k.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "e":
		m.edit = m.doc.Ref
		return tea.Quit
	case "j", "down":
		m.offset++
	case "k", "up":
		m.offset--
	case " ", "pgdown", "f":
		m.offset += m.viewport()
	case "b", "pgup":
		m.offset -= m.viewport()
	case "tab":
		m.cycle(1)
	case "shift+tab":
		m.cycle(-1)
	case "enter":
		m.follow()
	case "backspace":
		m.back()
	}
	m.clamp()
	return nil
}

func (m *Model) cycle(dir int) {
	n := len(m.doc.Links)
	if n == 0 {
		return
	}
	switch {
	case m.sel < 0 && dir < 0:
		m.sel = n - 1
	default:
		m.sel = (m.sel + dir + n) % n
	}
	m.reveal(m.linkLines[m.sel])
}

func (m *Model) follow() {
	if m.sel < 0 {
		return
	}
	if m.load == nil {
		m.msg = "no loader: cannot follow links"
		return
	}
	l := m.doc.Links[m.sel]
	d, err := m.load(l.Target)
	if err != nil {
		m.msg = err.Error()
		return
	}
	m.history = append(m.history, frame{m.doc, m.offset, m.sel})
	m.setDoc(d)
	m.offset, m.sel = 0, -1
}

func (m *Model) back() {
	if len(m.history) == 0 {
		return
	}
	f := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.setDoc(f.doc)
	m.offset, m.sel = f.offset, f.sel
}

func (m *Model) View() string {
	end := m.offset + m.viewport()
	if end > len(m.lines) {
		end = len(m.lines)
	}
	var b strings.Builder
	for _, l := range m.lines[m.offset:end] {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	for i := end - m.offset; i < m.viewport(); i++ {
		b.WriteByte('\n')
	}
	b.WriteString(m.status())
	return b.String()
}

func (m *Model) status() string {
	title := m.doc.Title
	if title == "" {
		title = m.doc.Ref
	}
	s := title
	switch {
	case m.msg != "":
		s += "  !! " + m.msg
	case m.sel >= 0:
		l := m.doc.Links[m.sel]
		s += fmt.Sprintf("  [%d/%d] %s -> %s  (enter follows)", m.sel+1, len(m.doc.Links), l.Label, l.Target)
	case len(m.doc.Links) > 0:
		s += fmt.Sprintf("  %d links (tab)", len(m.doc.Links))
	}
	if len(m.history) > 0 {
		s += "  (backspace: back)"
	}
	return ansi.Truncate("\x1b[7m"+s+"\x1b[0m", m.width, "…")
}
