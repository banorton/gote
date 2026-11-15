package frontmatter

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// Quoted is written as a double-quoted YAML string.
type Quoted string

type op struct {
	key    string
	val    any
	delete bool
}

// Map records edits applied by Update.
type Map struct{ ops []op }

func (m *Map) Set(key string, val any) { m.ops = append(m.ops, op{key: key, val: val}) }
func (m *Map) Delete(key string)       { m.ops = append(m.ops, op{key: key, delete: true}) }

func bounds(src []byte) (fmStart, fmEnd, bodyStart int, ok bool) {
	if !bytes.HasPrefix(src, []byte("---\n")) && !bytes.HasPrefix(src, []byte("---\r\n")) {
		return 0, 0, 0, false
	}
	pos := bytes.IndexByte(src, '\n') + 1
	fmStart = pos
	for pos < len(src) {
		end := bytes.IndexByte(src[pos:], '\n')
		next := len(src)
		line := src[pos:]
		if end >= 0 {
			next = pos + end + 1
			line = src[pos : pos+end]
		}
		if string(bytes.TrimSuffix(line, []byte("\r"))) == "---" {
			return fmStart, pos, next, true
		}
		pos = next
	}
	return 0, 0, 0, false
}

// Split returns the raw frontmatter, the body, whether a block exists, and the
// number of file lines the block (delimiters included) occupies.
func Split(src []byte) (fm, body []byte, has bool, lines int) {
	s, e, b, ok := bounds(src)
	if !ok {
		return nil, src, false, 0
	}
	return src[s:e], src[b:], true, bytes.Count(src[:b], []byte("\n"))
}

type span struct{ start, end int } // 0-based inclusive line indexes

func topLevelKeys(fm []byte, lines []string) (map[string]span, error) {
	f, err := parser.ParseBytes(fm, 0)
	if err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	var vals []*ast.MappingValueNode
	if len(f.Docs) > 0 && f.Docs[0].Body != nil {
		switch b := f.Docs[0].Body.(type) {
		case *ast.MappingNode:
			vals = b.Values
		case *ast.MappingValueNode:
			vals = []*ast.MappingValueNode{b}
		default:
			return nil, fmt.Errorf("frontmatter: not a mapping")
		}
	}
	starts := make([]int, len(vals))
	names := make([]string, len(vals))
	for i, v := range vals {
		tk := v.Key.GetToken()
		starts[i] = tk.Position.Line - 1
		names[i] = tk.Value
	}
	out := map[string]span{}
	for i := range vals {
		end := len(lines) - 1
		if i+1 < len(vals) {
			end = starts[i+1] - 1
		}
		for end > starts[i] {
			l := strings.TrimRight(lines[end], "\r\n")
			if strings.TrimSpace(l) == "" || strings.HasPrefix(l, "#") {
				end--
				continue
			}
			break
		}
		out[names[i]] = span{starts[i], end}
	}
	return out, nil
}

func render(key string, val any) (string, error) {
	switch v := val.(type) {
	case Quoted:
		return fmt.Sprintf("%s: %q\n", key, string(v)), nil
	case []string:
		if len(v) == 0 {
			return key + ": []\n", nil
		}
		var sb strings.Builder
		sb.WriteString(key + ":\n")
		for _, s := range v {
			b, err := yaml.Marshal(s)
			if err != nil {
				return "", err
			}
			sb.WriteString("  - " + strings.TrimRight(string(b), "\n") + "\n")
		}
		return sb.String(), nil
	}
	b, err := yaml.Marshal(map[string]any{key: val})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Update applies the edits recorded by fn. Edits are spliced by line so every
// untouched byte (key order, comments, formatting) is preserved. A frontmatter
// block is added when absent.
func Update(src []byte, fn func(m *Map)) ([]byte, error) {
	var m Map
	fn(&m)
	s, e, b, has := bounds(src)
	var fm, body []byte
	if has {
		fm, body = src[s:e], src[b:]
	} else {
		body = src
	}
	lines := strings.SplitAfter(string(fm), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	keys, err := topLevelKeys(fm, lines)
	if err != nil {
		return nil, err
	}
	type repl struct {
		sp   span
		text string
	}
	var repls []repl
	var appended strings.Builder
	for _, o := range m.ops {
		sp, exists := keys[o.key]
		switch {
		case o.delete && exists:
			repls = append(repls, repl{sp, ""})
			delete(keys, o.key)
		case o.delete:
		default:
			text, err := render(o.key, o.val)
			if err != nil {
				return nil, err
			}
			if exists {
				repls = append(repls, repl{sp, text})
			} else {
				appended.WriteString(text)
			}
		}
	}
	sort.Slice(repls, func(i, j int) bool { return repls[i].sp.start > repls[j].sp.start })
	for _, r := range repls {
		tail := append([]string{r.text}, lines[r.sp.end+1:]...)
		lines = append(lines[:r.sp.start:r.sp.start], tail...)
	}
	if len(lines) > 0 && !strings.HasSuffix(lines[len(lines)-1], "\n") {
		lines[len(lines)-1] += "\n"
	}
	newFM := strings.Join(lines, "") + appended.String()
	if !has {
		return []byte("---\n" + newFM + "---\n" + string(body)), nil
	}
	var out bytes.Buffer
	out.Write(src[:s])
	out.WriteString(newFM)
	out.Write(src[e:])
	return out.Bytes(), nil
}
