package parse

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"go.abhg.dev/goldmark/wikilink"

	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/parse/frontmatter"
)

var taskPrefix = regexp.MustCompile(`^\s*\[[ xX]\]\s*`)

var md = goldmark.New(
	goldmark.WithExtensions(extension.TaskList, &wikilink.Extender{}),
)

// Note parses a markdown file. It never fails: invalid frontmatter is reported
// in ParseErr and the body is still indexed.
func Note(id string, src []byte) note.Note {
	base := path.Base(id)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	n := note.Note{
		ID:       id,
		Kind:     note.KindNote,
		Title:    name,
		Stem:     strings.ToLower(name),
		Semantic: true,
		Fields:   map[string]any{},
	}

	fm, body, has, fmLines := frontmatter.Split(src)
	n.Body = string(body)
	if has && len(bytes.TrimSpace(fm)) > 0 {
		var fields map[string]any
		if err := yaml.Unmarshal(fm, &fields); err != nil {
			n.ParseErr = err.Error()
		} else if fields != nil {
			n.Fields = fields
			applyFields(&n)
		}
	}

	walkBody(&n, body, fmLines)
	return n
}

func applyFields(n *note.Note) {
	f := n.Fields
	n.Tags = stringList(f["tags"])
	n.Aliases = stringList(f["aliases"])
	if t, ok := f["title"].(string); ok && strings.TrimSpace(t) != "" {
		n.Title = t
	}
	switch c := f["created"].(type) {
	case string:
		n.Created = c
	case uint64:
		n.Created = fmt.Sprintf("%012d", c)
	case int64:
		n.Created = fmt.Sprintf("%012d", c)
	case int:
		n.Created = fmt.Sprintf("%012d", c)
	}
	if s, ok := f["semantic"].(bool); ok {
		n.Semantic = s
	}
	for _, k := range []string{"related", "parent"} {
		for _, v := range stringList(f[k]) {
			v = strings.TrimSpace(v)
			v = strings.TrimSuffix(strings.TrimPrefix(v, "[["), "]]")
			if v != "" {
				n.Links = append(n.Links, note.Link{Target: v, Kind: k})
			}
		}
	}
}

// stringList flattens scalars and nested lists: YAML reads an unquoted
// [[x]] as a list containing a list.
func stringList(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, stringList(e)...)
		}
		return out
	default:
		return []string{fmt.Sprint(x)}
	}
}

func walkBody(n *note.Note, body []byte, lineOffset int) {
	doc := md.Parser().Parse(text.NewReader(body))
	lineAt := func(off int) int {
		if off > len(body) {
			off = len(body)
		}
		return lineOffset + 1 + bytes.Count(body[:off], []byte("\n"))
	}
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := node.(type) {
		case *ast.Heading:
			if v.Lines().Len() > 0 {
				seg := v.Lines().At(0)
				n.Headings = append(n.Headings, note.Heading{
					Level: v.Level,
					Text:  strings.TrimSpace(string(seg.Value(body))),
					Line:  lineAt(seg.Start),
				})
			}
		case *wikilink.Node:
			l := note.Link{Target: string(v.Target), Fragment: string(v.Fragment), Kind: note.LinkWiki, Line: lineOf(node, lineAt)}
			if v.Embed {
				l.Kind = note.LinkEmbed
			}
			if t, ok := v.FirstChild().(*ast.Text); ok && t.Segment.Start > 0 && body[t.Segment.Start-1] == '|' {
				l.Alias = string(t.Segment.Value(body))
			}
			n.Links = append(n.Links, l)
		case *ast.Link:
			if t, frag, ok := relativeLink(string(v.Destination)); ok {
				n.Links = append(n.Links, note.Link{Target: t, Fragment: frag, Kind: note.LinkMD, Line: lineOf(node, lineAt)})
			}
		case *extast.TaskCheckBox:
			li := node.Parent()
			for li != nil {
				if _, ok := li.(*ast.ListItem); ok {
					break
				}
				li = li.Parent()
			}
			if li != nil && li.FirstChild() != nil && li.FirstChild().Lines().Len() > 0 {
				seg := li.FirstChild().Lines().At(0)
				n.Tasks = append(n.Tasks, note.Task{
					Line: lineAt(seg.Start),
					Text: strings.TrimSpace(taskPrefix.ReplaceAllString(string(seg.Value(body)), "")),
					Done: v.IsChecked,
				})
			}
		}
		return ast.WalkContinue, nil
	})
}

// lineOf finds the line of an inline node via its first text descendant, else
// the enclosing block's first line.
func lineOf(node ast.Node, lineAt func(int) int) int {
	var first *ast.Text
	_ = ast.Walk(node, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := c.(*ast.Text); ok && entering {
			first = t
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if first != nil {
		return lineAt(first.Segment.Start)
	}
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Type() == ast.TypeBlock && p.Lines().Len() > 0 {
			return lineAt(p.Lines().At(0).Start)
		}
	}
	return lineAt(0)
}

func relativeLink(dest string) (target, frag string, ok bool) {
	if dest == "" || strings.HasPrefix(dest, "#") || strings.HasPrefix(dest, "/") {
		return "", "", false
	}
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return "", "", false
	}
	p := u.Path
	if e := strings.ToLower(path.Ext(p)); e != ".md" && e != ".excalidraw" {
		return "", "", false
	}
	return p, u.Fragment, true
}
