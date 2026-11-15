package semantic

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"

	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/parse/frontmatter"
)

const maxChunk = 2000

type Chunk struct {
	Ord         int
	HeadingPath string
	StartLine   int
	Text        string
	// Embed is the exact text sent to the embedder; Hash identifies it.
	Embed string
	Hash  string
}

var headingRe = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)\s*$`)

func frontmatterLines(src string) int {
	_, _, _, n := frontmatter.Split([]byte(src))
	return n
}

func eligible(n note.Note, excludeTags []string) bool {
	if !n.Semantic {
		return false
	}
	if n.Kind != note.KindNote {
		return n.Kind == note.KindDiagram
	}
	for _, t := range n.Tags {
		for _, x := range excludeTags {
			if strings.EqualFold(t, x) {
				return false
			}
		}
	}
	return true
}

type piece struct {
	text string
	line int
}

// ChunkNote splits a note into embeddable chunks. fmLines is the number of file
// lines before n.Body so start lines refer to the file. Ineligible notes yield none.
func ChunkNote(n note.Note, fmLines int, excludeTags []string) []Chunk {
	if !eligible(n, excludeTags) {
		return nil
	}
	var pieces []sectionPiece
	if n.Kind == note.KindDiagram {
		if t := strings.TrimSpace(n.Body); t != "" {
			pieces = append(pieces, sectionPiece{piece: piece{t, 0}})
		}
	} else {
		pieces = sections(n.Body, fmLines)
	}
	var out []Chunk
	for _, p := range pieces {
		c := Chunk{Ord: len(out), HeadingPath: p.path, StartLine: p.line, Text: p.text}
		c.Embed = header(n, p.path) + "\n\n" + p.text
		c.Hash = strconv.FormatUint(xxhash.Sum64String(c.Embed), 16)
		out = append(out, c)
	}
	return out
}

func header(n note.Note, path string) string {
	lines := []string{"Title: " + n.Title}
	if path != "" {
		lines = append(lines, "Section: "+path)
	}
	if len(n.Tags) > 0 {
		lines = append(lines, "Tags: "+strings.Join(n.Tags, ", "))
	}
	if len(n.Aliases) > 0 {
		lines = append(lines, "Aliases: "+strings.Join(n.Aliases, ", "))
	}
	return strings.Join(lines, "\n")
}

type sectionPiece struct {
	piece
	path string
}

type section struct {
	path        string
	headingLine int
	firstLine   int
	lines       []string
}

func sections(body string, fmLines int) []sectionPiece {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	cur := &section{firstLine: fmLines + 1}
	var secs []*section
	type lvl struct {
		level int
		text  string
	}
	var stack []lvl
	fence := ""
	for i, l := range lines {
		lineNo := fmLines + 1 + i
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			if fence == "" {
				fence = t[:3]
			} else if strings.HasPrefix(t, fence) {
				fence = ""
			}
		}
		if m := headingRe.FindStringSubmatch(l); m != nil && fence == "" {
			secs = append(secs, cur)
			for len(stack) > 0 && stack[len(stack)-1].level >= len(m[1]) {
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, lvl{len(m[1]), m[2]})
			var names []string
			for _, s := range stack {
				names = append(names, s.text)
			}
			cur = &section{path: strings.Join(names, " > "), headingLine: lineNo, firstLine: lineNo + 1}
			continue
		}
		cur.lines = append(cur.lines, l)
	}
	secs = append(secs, cur)

	var out []sectionPiece
	for _, s := range secs {
		ps := splitSection(s)
		for i, p := range ps {
			if i == 0 && s.headingLine > 0 {
				p.line = s.headingLine
			}
			out = append(out, sectionPiece{p, s.path})
		}
	}
	return out
}

func splitSection(s *section) []piece {
	text := strings.TrimSpace(strings.Join(s.lines, "\n"))
	if text == "" {
		return nil
	}
	if len([]rune(text)) <= maxChunk {
		first := 0
		for first < len(s.lines) && strings.TrimSpace(s.lines[first]) == "" {
			first++
		}
		return []piece{{text, s.firstLine + first}}
	}
	type para struct {
		text string
		line int
	}
	var paras []para
	start := -1
	flush := func(end int) {
		if start >= 0 {
			paras = append(paras, para{strings.Join(s.lines[start:end], "\n"), s.firstLine + start})
			start = -1
		}
	}
	for i, l := range s.lines {
		if strings.TrimSpace(l) == "" {
			flush(i)
		} else if start < 0 {
			start = i
		}
	}
	flush(len(s.lines))

	var out []piece
	var cur *piece
	emit := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, p := range paras {
		r := []rune(p.text)
		if len(r) > maxChunk {
			emit()
			for off := 0; off < len(r); off += maxChunk {
				end := min(off+maxChunk, len(r))
				seg := string(r[off:end])
				if strings.TrimSpace(seg) == "" {
					continue
				}
				out = append(out, piece{strings.TrimSpace(seg), p.line + strings.Count(string(r[:off]), "\n")})
			}
			continue
		}
		if cur != nil && len([]rune(cur.text))+2+len(r) <= maxChunk {
			cur.text += "\n\n" + p.text
			continue
		}
		emit()
		cur = &piece{p.text, p.line}
	}
	emit()
	return out
}
