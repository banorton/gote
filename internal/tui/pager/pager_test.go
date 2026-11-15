package pager

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func lines(n int) string {
	var b []string
	for i := 0; i < n; i++ {
		b = append(b, fmt.Sprintf("line %d", i))
	}
	return strings.Join(b, "\n")
}

func key(m *Model, k string) tea.Cmd {
	var msg tea.KeyMsg
	switch k {
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		msg = tea.KeyMsg{Type: tea.KeyShiftTab}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	case "space":
		msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	_, cmd := m.Update(msg)
	return cmd
}

func sized(doc Doc, load Loader) *Model {
	m := New(doc, load)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 11})
	return m
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestScrollBounds(t *testing.T) {
	m := sized(Doc{Ref: "a", Text: lines(50)}, nil)
	if m.Offset() != 0 {
		t.Fatal(m.Offset())
	}
	key(m, "k")
	key(m, "b")
	if m.Offset() != 0 {
		t.Fatalf("scrolled above top: %d", m.Offset())
	}
	key(m, "j")
	key(m, "j")
	if m.Offset() != 2 {
		t.Fatalf("j: %d", m.Offset())
	}
	key(m, "k")
	if m.Offset() != 1 {
		t.Fatalf("k: %d", m.Offset())
	}
	key(m, "space")
	if m.Offset() != 11 {
		t.Fatalf("page down by viewport height: %d", m.Offset())
	}
	key(m, "b")
	if m.Offset() != 1 {
		t.Fatalf("page up: %d", m.Offset())
	}
	for i := 0; i < 200; i++ {
		key(m, "j")
	}
	if want := 50 - 10; m.Offset() != want {
		t.Fatalf("bottom bound: %d want %d", m.Offset(), want)
	}
	key(m, "space")
	if m.Offset() != 40 {
		t.Fatalf("page past bottom: %d", m.Offset())
	}
}

func TestShortDocNeverScrolls(t *testing.T) {
	m := sized(Doc{Ref: "a", Text: lines(3)}, nil)
	for _, k := range []string{"j", "space", "j"} {
		key(m, k)
	}
	if m.Offset() != 0 {
		t.Fatal(m.Offset())
	}
}

func TestViewShowsViewportAndStatus(t *testing.T) {
	m := sized(Doc{Ref: "a", Title: "Alpha", Text: lines(50)}, nil)
	v := m.View()
	if !strings.Contains(v, "line 0") || !strings.Contains(v, "line 9") || strings.Contains(v, "line 10\n") {
		t.Fatalf("viewport:\n%s", v)
	}
	if !strings.Contains(v, "Alpha") {
		t.Fatalf("status bar lacks title:\n%s", v)
	}
}

func linked() Doc {
	var b []string
	for i := 0; i < 40; i++ {
		b = append(b, fmt.Sprintf("row %d", i))
	}
	b[2] = "see \x1b[4mfirst\x1b[0m here"
	b[20] = "and second"
	b[35] = "finally third"
	return Doc{Ref: "a", Title: "A", Text: strings.Join(b, "\n"), Links: []Link{
		{Label: "first", Target: "t1"}, {Label: "second", Target: "t2"}, {Label: "third", Target: "t3"},
	}}
}

func TestLinkCycling(t *testing.T) {
	m := sized(linked(), nil)
	if m.Selected() != -1 {
		t.Fatal(m.Selected())
	}
	var seq []int
	for i := 0; i < 4; i++ {
		key(m, "tab")
		seq = append(seq, m.Selected())
	}
	if fmt.Sprint(seq) != "[0 1 2 0]" {
		t.Fatalf("tab order %v", seq)
	}
	key(m, "shift+tab")
	if m.Selected() != 2 {
		t.Fatalf("shift+tab wraps: %d", m.Selected())
	}
	key(m, "shift+tab")
	if m.Selected() != 1 {
		t.Fatal(m.Selected())
	}
	if o := m.Offset(); o > 20 || o+10 <= 20 {
		t.Fatalf("selected link line 20 not visible at offset %d", o)
	}
	if !strings.Contains(m.View(), "t2") {
		t.Fatalf("status shows target:\n%s", m.View())
	}
}

func TestShiftTabFromNoneSelectsLast(t *testing.T) {
	m := sized(linked(), nil)
	key(m, "shift+tab")
	if m.Selected() != 2 {
		t.Fatal(m.Selected())
	}
}

func TestNoLinksTabIsNoop(t *testing.T) {
	m := sized(Doc{Ref: "a", Text: lines(5)}, nil)
	key(m, "tab")
	key(m, "enter")
	if m.Selected() != -1 || m.Doc().Ref != "a" {
		t.Fatal("tab on linkless doc changed state")
	}
}

func TestFollowAndBack(t *testing.T) {
	docs := map[string]Doc{
		"t1": {Ref: "b", Title: "B", Text: lines(30), Links: []Link{{Label: "line 5", Target: "t3"}}},
		"t3": {Ref: "c", Title: "C", Text: "leaf"},
	}
	var loaded []string
	load := func(target string) (Doc, error) {
		loaded = append(loaded, target)
		d, ok := docs[target]
		if !ok {
			return Doc{}, errors.New("nope " + target)
		}
		return d, nil
	}
	m := sized(linked(), load)

	key(m, "enter")
	if len(loaded) != 0 {
		t.Fatal("enter with no selection loaded a doc")
	}
	key(m, "j")
	key(m, "j")
	key(m, "j")
	key(m, "tab")
	key(m, "enter")
	if m.Doc().Ref != "b" || m.Offset() != 0 || m.Selected() != -1 || m.Depth() != 1 {
		t.Fatalf("after follow: ref %s off %d sel %d depth %d", m.Doc().Ref, m.Offset(), m.Selected(), m.Depth())
	}
	key(m, "tab")
	key(m, "enter")
	if m.Doc().Ref != "c" || m.Depth() != 2 {
		t.Fatalf("second follow: %s %d", m.Doc().Ref, m.Depth())
	}
	key(m, "backspace")
	if m.Doc().Ref != "b" || m.Depth() != 1 {
		t.Fatalf("back: %s %d", m.Doc().Ref, m.Depth())
	}
	key(m, "backspace")
	if m.Doc().Ref != "a" || m.Depth() != 0 || m.Selected() != 0 {
		t.Fatalf("back to root: %s depth %d sel %d", m.Doc().Ref, m.Depth(), m.Selected())
	}
	key(m, "backspace")
	if m.Doc().Ref != "a" || m.Depth() != 0 {
		t.Fatal("back past root")
	}
}

func TestFollowErrorKeepsDocAndReports(t *testing.T) {
	m := sized(linked(), func(string) (Doc, error) { return Doc{}, errors.New("note gone") })
	key(m, "tab")
	key(m, "enter")
	if m.Doc().Ref != "a" || m.Depth() != 0 {
		t.Fatal("failed follow changed doc")
	}
	if !strings.Contains(m.View(), "note gone") {
		t.Fatalf("error not shown:\n%s", m.View())
	}
	key(m, "j")
	if strings.Contains(m.View(), "note gone") {
		t.Fatal("error not cleared on next key")
	}
}

func TestQuitAndEdit(t *testing.T) {
	m := sized(linked(), nil)
	if !isQuit(key(m, "q")) {
		t.Fatal("q should quit")
	}
	if m.EditRequest() != "" {
		t.Fatal("edit request set by quit")
	}
	m = sized(linked(), nil)
	if !isQuit(key(m, "e")) || m.EditRequest() != "a" {
		t.Fatalf("e: edit request %q", m.EditRequest())
	}
}

func TestEditAfterFollowTargetsCurrentDoc(t *testing.T) {
	m := sized(linked(), func(string) (Doc, error) { return Doc{Ref: "b", Text: "x"}, nil })
	key(m, "tab")
	key(m, "enter")
	key(m, "e")
	if m.EditRequest() != "b" {
		t.Fatal(m.EditRequest())
	}
}
