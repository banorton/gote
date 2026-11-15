package fuzzy

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type call struct {
	Name string
	Args []string
}

type harness struct {
	m     Model
	calls []call
	edits []string
}

func newHarness(t *testing.T, mut func(*Config)) *harness {
	t.Helper()
	h := &harness{}
	cfg := Config{
		Notes: func() ([]Item, error) {
			return []Item{
				{ID: "alpha.md", Search: "Alpha  alpha.md"},
				{ID: "beta.md", Search: "Beta  beta.md"},
				{ID: "sub/gamma.md", Search: "Gamma  sub/gamma.md  gam"},
			}, nil
		},
		Lines: func() ([]Item, error) {
			return []Item{
				{ID: "alpha.md", Line: 3, Search: "hello world", Suffix: "alpha.md:3"},
				{ID: "beta.md", Line: 9, Search: "goodbye world", Suffix: "beta.md:9"},
			}, nil
		},
		Preview: func(id string, line int) (string, error) { return fmt.Sprintf("preview %s:%d", id, line), nil },
		Invoke: func(name string, args []string) error {
			h.calls = append(h.calls, call{name, args})
			return nil
		},
		Edit: func(id string, line int) error {
			h.edits = append(h.edits, fmt.Sprintf("%s:%d", id, line))
			return nil
		},
		Editor: "nvim",
		Exec: func(fn func() error, done func(error) tea.Msg) tea.Cmd {
			return func() tea.Msg { return done(fn()) }
		},
	}
	if mut != nil {
		mut(&cfg)
	}
	h.m = New(cfg)
	h.send(tea.WindowSizeMsg{Width: 80, Height: 20})
	return h
}

func (h *harness) send(msg tea.Msg) {
	mm, cmd := h.m.Update(msg)
	h.m = mm.(Model)
	for cmd != nil {
		out := cmd()
		if _, quit := out.(tea.QuitMsg); quit {
			h.m.quit = true
			return
		}
		if out == nil {
			return
		}
		mm, cmd = h.m.Update(out)
		h.m = mm.(Model)
	}
}

func (h *harness) typ(s string) {
	for _, r := range s {
		h.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func key(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func (h *harness) visible() []string {
	var out []string
	for _, it := range h.m.Visible() {
		out = append(out, it.ID)
	}
	return out
}

func TestFilterTyping(t *testing.T) {
	h := newHarness(t, nil)
	if got := h.visible(); len(got) != 3 {
		t.Fatalf("%v", got)
	}
	h.typ("gam")
	if got := h.visible(); !reflect.DeepEqual(got, []string{"sub/gamma.md"}) {
		t.Fatalf("%v", got)
	}
	h.send(key(tea.KeyBackspace))
	h.send(key(tea.KeyBackspace))
	h.send(key(tea.KeyBackspace))
	if len(h.visible()) != 3 || h.m.Pattern() != "" {
		t.Fatal("backspace should restore list")
	}
	h.typ("zzzz")
	if len(h.visible()) != 0 {
		t.Fatal("no match should be empty")
	}
	h.send(key(tea.KeyCtrlU))
	if h.m.Pattern() != "" || len(h.visible()) != 3 {
		t.Fatal("ctrl-u clears")
	}
}

func TestMovement(t *testing.T) {
	h := newHarness(t, nil)
	if h.m.Cursor() != 0 {
		t.Fatal()
	}
	h.send(key(tea.KeyDown))
	h.send(key(tea.KeyCtrlJ))
	if h.m.Cursor() != 2 {
		t.Fatalf("cursor %d", h.m.Cursor())
	}
	h.send(key(tea.KeyDown))
	if h.m.Cursor() != 2 {
		t.Fatal("clamped at bottom")
	}
	h.send(key(tea.KeyUp))
	h.send(key(tea.KeyCtrlK))
	h.send(key(tea.KeyUp))
	if h.m.Cursor() != 0 {
		t.Fatal("clamped at top")
	}
}

func TestCursorResetsOnFilter(t *testing.T) {
	h := newHarness(t, nil)
	h.send(key(tea.KeyDown))
	h.typ("a")
	if h.m.Cursor() != 0 {
		t.Fatal()
	}
}

func TestModeToggle(t *testing.T) {
	h := newHarness(t, nil)
	h.typ("world")
	if len(h.visible()) != 0 {
		t.Fatal("note mode has no world")
	}
	h.send(key(tea.KeyCtrlT))
	if h.m.Mode() != ModeContent || len(h.visible()) != 2 {
		t.Fatalf("mode %v visible %v", h.m.Mode(), h.visible())
	}
	h.send(key(tea.KeyCtrlT))
	if h.m.Mode() != ModeNotes || h.m.Pattern() != "world" {
		t.Fatal("toggle back keeps pattern")
	}
}

func TestEnterOpensNote(t *testing.T) {
	h := newHarness(t, nil)
	h.send(key(tea.KeyDown))
	h.send(key(tea.KeyEnter))
	want := []call{{"open", []string{"beta.md"}}}
	if !reflect.DeepEqual(h.calls, want) || len(h.edits) != 0 {
		t.Fatalf("%+v %v", h.calls, h.edits)
	}
	if h.m.quit {
		t.Fatal("returns to the finder after open")
	}
}

func TestEnterContentModeOpensAtLineForLineEditors(t *testing.T) {
	h := newHarness(t, nil)
	h.send(key(tea.KeyCtrlT))
	h.send(key(tea.KeyDown))
	h.send(key(tea.KeyEnter))
	if !reflect.DeepEqual(h.edits, []string{"beta.md:9"}) || len(h.calls) != 0 {
		t.Fatalf("%+v %v", h.calls, h.edits)
	}
}

func TestEnterContentModeOtherEditorJustOpens(t *testing.T) {
	h := newHarness(t, func(c *Config) { c.Editor = "code -w" })
	h.send(key(tea.KeyCtrlT))
	h.send(key(tea.KeyEnter))
	if !reflect.DeepEqual(h.calls, []call{{"open", []string{"alpha.md"}}}) || len(h.edits) != 0 {
		t.Fatalf("%+v %v", h.calls, h.edits)
	}
}

func TestSupportsLine(t *testing.T) {
	for ed, want := range map[string]bool{
		"nvim": true, "vim": true, "vi": true, "nano": true, "/usr/bin/nvim": true, "nvim -u NONE": true,
		"code -w": false, "emacs": false, "": false,
	} {
		if got := SupportsLine(ed); got != want {
			t.Errorf("%q: %v", ed, got)
		}
	}
}

func TestViewAndPinKeys(t *testing.T) {
	h := newHarness(t, nil)
	h.send(key(tea.KeyCtrlV))
	h.send(key(tea.KeyCtrlP))
	want := []call{{"view", []string{"alpha.md"}}, {"pin", []string{"alpha.md"}}}
	if !reflect.DeepEqual(h.calls, want) {
		t.Fatalf("%+v", h.calls)
	}
	if !strings.Contains(h.m.Status(), "pin") {
		t.Fatalf("status %q", h.m.Status())
	}
}

func TestDeleteConfirm(t *testing.T) {
	h := newHarness(t, nil)
	h.send(key(tea.KeyCtrlD))
	if len(h.calls) != 0 || !strings.Contains(h.m.Status(), "y/n") {
		t.Fatalf("%+v %q", h.calls, h.m.Status())
	}
	h.typ("n")
	if len(h.calls) != 0 || len(h.visible()) != 3 || h.m.Pattern() != "" {
		t.Fatal("n cancels and is not typed into the pattern")
	}
	h.send(key(tea.KeyCtrlD))
	h.typ("y")
	if !reflect.DeepEqual(h.calls, []call{{"delete", []string{"alpha.md"}}}) {
		t.Fatalf("%+v", h.calls)
	}
	if got := h.visible(); !reflect.DeepEqual(got, []string{"beta.md", "sub/gamma.md"}) {
		t.Fatalf("deleted note leaves list: %v", got)
	}
}

func TestDeleteFailureKeepsItem(t *testing.T) {
	h := newHarness(t, func(c *Config) {
		c.Invoke = func(name string, args []string) error { return errors.New("boom") }
	})
	h.send(key(tea.KeyCtrlD))
	h.typ("y")
	if len(h.visible()) != 3 || !strings.Contains(h.m.Status(), "boom") {
		t.Fatalf("%v %q", h.visible(), h.m.Status())
	}
}

func TestMissingCommandShowsStatusError(t *testing.T) {
	h := newHarness(t, func(c *Config) {
		c.Invoke = func(name string, args []string) error { return fmt.Errorf("%s: not available", name) }
	})
	h.send(key(tea.KeyCtrlV))
	if h.m.Status() != "view: not available" {
		t.Fatalf("%q", h.m.Status())
	}
	h.send(key(tea.KeyCtrlP))
	if h.m.Status() != "pin: not available" {
		t.Fatalf("%q", h.m.Status())
	}
}

func TestNoSelectionIsNoop(t *testing.T) {
	h := newHarness(t, nil)
	h.typ("zzzz")
	for _, k := range []tea.KeyType{tea.KeyEnter, tea.KeyCtrlV, tea.KeyCtrlD, tea.KeyCtrlP} {
		h.send(key(k))
	}
	if len(h.calls) != 0 {
		t.Fatalf("%+v", h.calls)
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		h := newHarness(t, nil)
		h.send(key(k))
		if !h.m.quit {
			t.Errorf("%v did not quit", k)
		}
	}
}

func TestPreviewOnlyWhenWide(t *testing.T) {
	h := newHarness(t, nil)
	if strings.Contains(h.m.View(), "preview alpha.md:0") {
		t.Fatal("hidden below 100 cols")
	}
	h.send(tea.WindowSizeMsg{Width: 120, Height: 20})
	if !strings.Contains(h.m.View(), "preview alpha.md:0") {
		t.Fatalf("shown at 120 cols:\n%s", h.m.View())
	}
	h.send(key(tea.KeyDown))
	if !strings.Contains(h.m.View(), "preview beta.md:0") {
		t.Fatal("preview follows cursor")
	}
	h.send(key(tea.KeyCtrlT))
	if !strings.Contains(h.m.View(), "preview alpha.md:3") {
		t.Fatalf("content mode previews at line:\n%s", h.m.View())
	}
}

func TestViewHighlightsAndDoesNotPanic(t *testing.T) {
	h := newHarness(t, nil)
	h.typ("gam")
	v := h.m.View()
	if !strings.Contains(v, "\x1b[") || !strings.Contains(v, "amma") {
		t.Fatalf("%q", v)
	}
	h.send(tea.WindowSizeMsg{Width: 5, Height: 3})
	_ = h.m.View()
}

func TestCorpusLoadErrorShownInStatus(t *testing.T) {
	h := newHarness(t, func(c *Config) {
		c.Notes = func() ([]Item, error) { return nil, errors.New("no corpus") }
	})
	if !strings.Contains(h.m.Status(), "no corpus") {
		t.Fatalf("%q", h.m.Status())
	}
}
