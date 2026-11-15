package excal

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/parse"
)

func init() { index.RegisterParser(".excalidraw", ParseNote) }

type Bound struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type Binding struct {
	ElementID string `json:"elementId"`
}

type Element struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	X             float64  `json:"x"`
	Y             float64  `json:"y"`
	Width         float64  `json:"width"`
	Height        float64  `json:"height"`
	Text          string   `json:"text"`
	ContainerID   string   `json:"containerId"`
	IsDeleted     bool     `json:"isDeleted"`
	Link          string   `json:"link"`
	BoundElements []Bound  `json:"boundElements"`
	StartBinding  *Binding `json:"startBinding"`
	EndBinding    *Binding `json:"endBinding"`
}

type Doc struct {
	Type     string         `json:"type"`
	Elements []Element      `json:"elements"`
	AppState map[string]any `json:"appState"`
	Files    map[string]any `json:"files"`
}

// Decode parses a native Excalidraw scene. Deleted elements stay in
// Doc.Elements; use Live to skip them.
func Decode(src []byte) (*Doc, error) {
	var d Doc
	if err := json.Unmarshal(src, &d); err != nil {
		return nil, fmt.Errorf("invalid excalidraw JSON: %w", err)
	}
	if d.Type != "excalidraw" {
		return nil, fmt.Errorf("not an excalidraw scene: type is %q", d.Type)
	}
	return &d, nil
}

// NewScene returns a minimal valid empty scene.
func NewScene() []byte {
	return []byte(`{
  "type": "excalidraw",
  "version": 2,
  "source": "gote",
  "elements": [],
  "appState": {
    "viewBackgroundColor": "#ffffff"
  },
  "files": {}
}
`)
}

func (d *Doc) live() map[string]*Element {
	m := make(map[string]*Element, len(d.Elements))
	for i := range d.Elements {
		if e := &d.Elements[i]; !e.IsDeleted {
			m[e.ID] = e
		}
	}
	return m
}

func (d *Doc) liveInOrder() []*Element {
	var out []*Element
	for i := range d.Elements {
		if !d.Elements[i].IsDeleted {
			out = append(out, &d.Elements[i])
		}
	}
	return out
}

func stem(id string) string {
	b := path.Base(id)
	return strings.TrimSuffix(b, path.Ext(b))
}

// ParseNote indexes a drawing: title is the filename stem, body is the joined
// element texts, links are wikilinks found in texts and element links.
func ParseNote(id string, src []byte) note.Note {
	name := stem(id)
	n := note.Note{
		ID:       id,
		Kind:     note.KindDiagram,
		Title:    name,
		Stem:     note.Fold(name),
		Semantic: true,
		Fields:   map[string]any{},
	}
	d, err := Decode(src)
	if err != nil {
		n.ParseErr = err.Error()
		return n
	}
	var texts, linkSrc []string
	for _, e := range d.liveInOrder() {
		if e.Type == "text" && e.Text != "" {
			texts = append(texts, e.Text)
		}
	}
	linkSrc = append(linkSrc, texts...)
	for _, e := range d.liveInOrder() {
		if e.Link != "" {
			linkSrc = append(linkSrc, e.Link)
		}
	}
	n.Body = strings.Join(texts, "\n")
	for _, s := range linkSrc {
		for _, l := range parse.Note(id, []byte(s)).Links {
			if l.Kind == note.LinkWiki || l.Kind == note.LinkEmbed {
				l.Line = 0
				n.Links = append(n.Links, l)
			}
		}
	}
	return n
}
