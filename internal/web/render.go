package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/wikilink"

	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/parse"
)

var imageExts = map[string]bool{
	".apng": true, ".avif": true, ".gif": true, ".jpg": true, ".jpeg": true,
	".jfif": true, ".png": true, ".svg": true, ".webp": true,
}

func escPath(id string) string {
	segs := strings.Split(id, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}

func routeFor(id, kind string) string {
	if kind == note.KindDiagram {
		return "/d/" + escPath(id)
	}
	return "/n/" + escPath(id)
}

// slug approximates goldmark's auto heading ids so [[note#Heading]] lands on
// the heading.
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == ' ' || r == '-':
			b.WriteRune('-')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

type page struct {
	ID       string
	Title    string
	Tags     []string
	Body     template.HTML
	Drawings bool
	Events   template.JS
}

type wikiRenderer struct {
	s        *Server
	ctx      context.Context
	drawings bool
	err      error
	open     map[*wikilink.Node]bool
}

func (r *wikiRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(wikilink.Kind, r.render)
}

func (r *wikiRenderer) resolve(ref string) (id, kind string, ok bool) {
	if ref == "" || ref == "-" {
		return "", "", false
	}
	id, err := ops.ResolveNote(r.ctx, r.s.env, ref)
	if err != nil {
		var oe *ops.Error
		if !errors.As(err, &oe) || oe.Code != ops.CodeNotFound {
			r.err = err
		}
		return "", "", false
	}
	kind, ok = r.s.indexedKind(id)
	return id, kind, ok
}

func (r *wikiRenderer) render(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*wikilink.Node)
	if !entering {
		if r.open[n] {
			w.WriteString("</a>")
		}
		return ast.WalkContinue, nil
	}
	target := string(n.Target)
	label := func() string {
		if c := n.FirstChild(); c != nil {
			if t, ok := c.(*ast.Text); ok {
				return string(t.Segment.Value(src))
			}
		}
		return target
	}
	if n.Embed {
		if imageExts[strings.ToLower(path.Ext(target))] {
			if rel, ok := r.s.lookupFile(target); ok {
				w.WriteString(`<img src="/f/` + escPath(rel) + `" alt="` + html.EscapeString(target) + `">`)
			} else {
				w.WriteString(`<span class="broken">` + html.EscapeString(label()) + `</span>`)
			}
			return ast.WalkSkipChildren, nil
		}
		id, kind, ok := r.resolve(target)
		switch {
		case ok && kind == note.KindDiagram:
			r.drawings = true
			w.WriteString(`<div class="gote-drawing" data-id="` + html.EscapeString(id) + `"><div class="gote-drawing-canvas"></div>` +
				`<a class="gote-edit" href="` + routeFor(id, kind) + `">edit</a></div>`)
			return ast.WalkSkipChildren, nil
		case !ok:
			w.WriteString(`<span class="broken">` + html.EscapeString(label()) + `</span>`)
			return ast.WalkSkipChildren, nil
		}
	}
	if target == "" {
		w.WriteString(`<a href="#` + html.EscapeString(slug(string(n.Fragment))) + `">`)
	} else if id, kind, ok := r.resolve(target); ok {
		href := routeFor(id, kind)
		if len(n.Fragment) > 0 {
			href += "#" + url.PathEscape(slug(string(n.Fragment)))
		}
		w.WriteString(`<a href="` + href + `">`)
	} else {
		w.WriteString(`<a class="broken" title="unresolved link">`)
	}
	if r.open == nil {
		r.open = map[*wikilink.Node]bool{}
	}
	r.open[n] = true
	return ast.WalkContinue, nil
}

func hasScheme(dest string) bool {
	u, err := url.Parse(dest)
	return err != nil || u.Scheme != "" || u.Host != ""
}

// rewriteDest points a relative markdown link or image at a served route.
func (r *wikiRenderer) rewriteDest(srcID string, dest []byte, image bool) []byte {
	d := string(dest)
	if d == "" || strings.HasPrefix(d, "#") || hasScheme(d) {
		return dest
	}
	u, err := url.Parse(d)
	if err != nil {
		return dest
	}
	var rel string
	if strings.HasPrefix(u.Path, "/") {
		rel = path.Clean(strings.TrimPrefix(u.Path, "/"))
	} else {
		rel = path.Join(path.Dir(srcID), u.Path)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return []byte("#")
	}
	frag := ""
	if u.Fragment != "" {
		frag = "#" + url.PathEscape(u.Fragment)
	}
	if !image {
		if kind, ok := r.s.indexedKind(rel); ok {
			return []byte(routeFor(rel, kind) + frag)
		}
	}
	if _, ok := r.s.vaultPath(rel); ok {
		return []byte("/f/" + escPath(rel))
	}
	if image {
		return dest
	}
	return []byte("/n/" + escPath(rel) + frag)
}

func (s *Server) markdown(ctx context.Context, id string, body []byte) (template.HTML, bool, error) {
	wr := &wikiRenderer{s: s, ctx: ctx}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithInlineParsers(util.Prioritized(&wikilink.Parser{}, 199)),
		),
		goldmark.WithRendererOptions(renderer.WithNodeRenderers(util.Prioritized(wr, 199))),
	)
	doc := md.Parser().Parse(text.NewReader(body))
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Link:
			v.Destination = wr.rewriteDest(id, v.Destination, false)
		case *ast.Image:
			v.Destination = wr.rewriteDest(id, v.Destination, true)
		}
		return ast.WalkContinue, nil
	})
	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, body, doc); err != nil {
		return "", false, err
	}
	if wr.err != nil {
		return "", false, wr.err
	}
	return template.HTML(buf.String()), wr.drawings, nil
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	kind, ok := s.kindOf(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if kind == note.KindDiagram {
		http.Redirect(w, r, "/d/"+escPath(id), http.StatusFound)
		return
	}
	abs, ok := s.vaultPath(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		fail(w, http.StatusInternalServerError, "read %s: %v", id, err)
		return
	}
	n := parse.Note(id, src)
	body, drawings, err := s.markdown(r.Context(), id, []byte(n.Body))
	if err != nil {
		failErr(w, err)
		return
	}
	events, _ := json.Marshal("/events?id=" + url.QueryEscape(id))
	p := page{ID: id, Title: n.Title, Tags: n.Tags, Body: body, Drawings: drawings, Events: template.JS(events)}
	renderPage(w, notePage, p)
}

func renderPage(w http.ResponseWriter, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		fail(w, http.StatusInternalServerError, "render: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.Copy(w, &buf)
}
