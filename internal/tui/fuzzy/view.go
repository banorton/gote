package fuzzy

import (
	"fmt"
	"io"
	"strings"
)

const (
	styleMatch = "\x1b[1;33m"
	styleDim   = "\x1b[2m"
	styleBold  = "\x1b[1m"
	styleReset = "\x1b[0m"
)

type execAdapter struct {
	fn func() error
}

func (e *execAdapter) Run() error          { return e.fn() }
func (e *execAdapter) SetStdin(io.Reader)  {}
func (e *execAdapter) SetStdout(io.Writer) {}
func (e *execAdapter) SetStderr(io.Writer) {}

func truncate(s string, w int) string {
	r := []rune(s)
	if w <= 0 {
		return ""
	}
	if len(r) <= w {
		return s
	}
	if w == 1 {
		return "…"
	}
	return string(r[:w-1]) + "…"
}

func (m Model) row(it Item, pos []int, selected bool, w int) (string, int) {
	prefix := "  "
	if selected {
		prefix = "> "
	}
	avail := w - len([]rune(prefix))
	if avail <= 0 {
		return truncate(prefix, w), min(w, len([]rune(prefix)))
	}
	text := []rune(it.Search)
	suffix := ""
	if it.Suffix != "" {
		suffix = "  " + it.Suffix
	}
	if len(text) > avail {
		text = append(text[:avail-1], '…')
		suffix = ""
	} else if len(text)+len([]rune(suffix)) > avail {
		suffix = truncate(suffix, avail-len(text))
	}
	marked := map[int]bool{}
	for _, p := range pos {
		marked[p] = true
	}
	var b strings.Builder
	if selected {
		b.WriteString(styleBold)
	}
	b.WriteString(prefix)
	for i, r := range text {
		if marked[i] {
			b.WriteString(styleMatch + string(r) + styleReset)
			if selected {
				b.WriteString(styleBold)
			}
		} else {
			b.WriteRune(r)
		}
	}
	b.WriteString(styleReset)
	if suffix != "" {
		b.WriteString(styleDim + suffix + styleReset)
	}
	return b.String(), len([]rune(prefix)) + len(text) + len([]rune(suffix))
}

func (m Model) View() string {
	w, h := m.width, m.height
	leftW := w
	var pv []string
	if m.previewOn() {
		leftW = w / 2
		pv = strings.Split(m.preview, "\n")
	}
	var out []string
	modeName := "notes"
	if m.mode == ModeContent {
		modeName = "content"
	}
	out = append(out, truncate("> "+m.pattern, w))
	out = append(out, styleDim+truncate(fmt.Sprintf("%d/%d [%s]", len(m.results), len(m.items()), modeName), w)+styleReset)
	lh := m.listHeight()
	for i := 0; i < lh; i++ {
		idx := m.offset + i
		var left string
		plain := 0
		if idx < len(m.results) {
			r := m.results[idx]
			left, plain = m.row(m.items()[r.Index], r.Positions, idx == m.cursor, leftW)
		}
		line := left
		if pv != nil {
			pad := leftW - plain
			if pad < 0 {
				pad = 0
			}
			pl := ""
			if i < len(pv) {
				pl = truncate(strings.ReplaceAll(pv[i], "\t", "    "), w-leftW-3)
			}
			line += strings.Repeat(" ", pad) + " │ " + pl
		}
		out = append(out, line)
	}
	out = append(out, truncate(m.status, w))
	if h > 0 && len(out) > h {
		out = out[:h]
	}
	return strings.Join(out, "\n")
}
