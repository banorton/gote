package menu

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func items(n int) []Item {
	out := make([]Item, n)
	for i := range out {
		out[i] = Item{ID: fmt.Sprintf("id%d", i), Title: fmt.Sprintf("title%d", i)}
	}
	return out
}

var acts = []Action{"open", "delete", "pin", "view", "rename", "unpin"}

func key(m model, s string) model {
	var msg tea.KeyMsg
	switch s {
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		msg = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	next, _ := m.Update(msg)
	return next.(model)
}

func TestSelectByHomeRowKey(t *testing.T) {
	m := newModel(items(25), "open", acts)
	m = key(m, "d")
	if !m.done || m.chosen.ID != "id2" || m.action != "open" {
		t.Fatalf("%+v", m)
	}
	m = newModel(items(25), "open", acts)
	m = key(m, ";")
	if m.chosen.ID != "id9" {
		t.Fatal(m.chosen)
	}
}

func TestPaging(t *testing.T) {
	m := newModel(items(25), "open", acts)
	if got := m.View(); !strings.Contains(got, "title0") || strings.Contains(got, "title10") || !strings.Contains(got, "1/3") {
		t.Fatal(got)
	}
	m = key(m, "n")
	m = key(m, "n")
	m = key(m, "n")
	if m.page != 2 {
		t.Fatalf("page clamps at last: %d", m.page)
	}
	m = key(m, "a")
	if m.chosen.ID != "id20" {
		t.Fatal(m.chosen)
	}
	m = newModel(items(25), "open", acts)
	m = key(m, "p")
	if m.page != 0 {
		t.Fatal("page clamps at first")
	}
	m = key(m, "n")
	m = key(m, "p")
	if m.page != 0 {
		t.Fatal(m.page)
	}
	m = newModel(items(25), "open", acts)
	m = key(m, "n")
	m = key(m, "n")
	m = key(m, "s")
	if !m.done || m.chosen.ID != "id21" {
		t.Fatalf("keys index into the current page: %v", m.chosen)
	}
}

func TestSelectBeyondPageIsIgnored(t *testing.T) {
	m := newModel(items(3), "open", acts)
	m = key(m, ";")
	if m.done {
		t.Fatal("no item under ;")
	}
}

func TestActionSwitching(t *testing.T) {
	m := newModel(items(5), "open", acts)
	m = key(m, "D")
	if m.action != "delete" || !strings.Contains(m.View(), "*Delete*") {
		t.Fatalf("%q", m.action)
	}
	m = key(m, "a")
	if m.chosen.ID != "id0" || m.action != "delete" {
		t.Fatalf("%+v", m)
	}
	m = newModel(items(5), "view", acts)
	if m.action != "view" {
		t.Fatal(m.action)
	}
	m = key(m, "tab")
	if m.action != "rename" {
		t.Fatalf("tab cycles: %q", m.action)
	}
	m = key(m, "Z")
	if m.action != "rename" {
		t.Fatal("unknown mode key ignored")
	}
	m = newModel(items(5), "open", []Action{"open", "pin"})
	m = key(m, "D")
	if m.action != "open" {
		t.Fatal("mode keys limited to supplied actions")
	}
}

func TestQuit(t *testing.T) {
	for _, k := range []string{"q", "esc", "ctrl+c"} {
		m := key(newModel(items(5), "open", acts), k)
		if !m.done || !errors.Is(m.err, ErrCancelled) {
			t.Fatalf("%s: %+v", k, m)
		}
	}
}

func TestRunRequiresTTY(t *testing.T) {
	if _, _, err := Run(items(2), "open", acts); err == nil {
		t.Fatal("expected error without a TTY")
	}
	if _, _, err := Run(nil, "open", acts); err == nil {
		t.Fatal("expected error for empty list")
	}
}

func TestViewRendersMultiLineDetail(t *testing.T) {
	its := []Item{
		{ID: "a", Title: "Alpha", Detail: "a.md:3  0.82\nthe matched snippet"},
		{ID: "b", Title: "Beta", Detail: "b.md"},
		{ID: "c", Title: "Gamma"},
	}
	got := newModel(its, "open", acts).View()
	want := " [a] Alpha  a.md:3  0.82\n     the matched snippet\n [s] Beta  b.md\n [d] Gamma\n"
	if !strings.HasPrefix(got, want) {
		t.Fatalf("got %q", got)
	}
	m := key(newModel(its, "open", acts), "s")
	if m.chosen.ID != "b" {
		t.Fatalf("home-row key must still pick the entry: %+v", m.chosen)
	}
}
