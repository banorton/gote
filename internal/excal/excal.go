package excal

import (
	"bytes"
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

// Gote holds gote's own top-level metadata in a drawing file. Excalidraw
// ignores unknown top-level keys on load but drops them on save, so the web
// handler carries this object across editor saves.
type Gote struct {
	Description string `json:"description,omitempty"`
}

type Doc struct {
	Type     string         `json:"type"`
	Gote     Gote           `json:"gote"`
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
	if desc := strings.TrimSpace(d.Gote.Description); desc != "" {
		texts = append(texts, desc)
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

const goteKey = "gote"

func marshalTop(m map[string]json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeTop(src []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(src, &m); err != nil {
		return nil, fmt.Errorf("invalid excalidraw JSON: %w", err)
	}
	return m, nil
}

func goteObject(m map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	g := map[string]json.RawMessage{}
	if raw, ok := m[goteKey]; ok {
		if err := json.Unmarshal(raw, &g); err != nil {
			return nil, fmt.Errorf("invalid %q object: %w", goteKey, err)
		}
	}
	return g, nil
}

// SetDescription returns src with the gote description set; empty clears it.
// Every other top-level key is kept; the file is re-marshalled (key order and
// whitespace may change).
func SetDescription(src []byte, desc string) ([]byte, error) {
	if _, err := Decode(src); err != nil {
		return nil, err
	}
	m, err := decodeTop(src)
	if err != nil {
		return nil, err
	}
	g, err := goteObject(m)
	if err != nil {
		return nil, err
	}
	if desc == "" {
		delete(g, "description")
	} else {
		b, err := json.Marshal(desc)
		if err != nil {
			return nil, err
		}
		g["description"] = b
	}
	if len(g) == 0 {
		delete(m, goteKey)
	} else {
		b, err := json.Marshal(g)
		if err != nil {
			return nil, err
		}
		m[goteKey] = b
	}
	return marshalTop(m)
}

// CarryGote returns incoming with the gote object from existing copied in.
// If existing has none, incoming is returned unchanged byte-for-byte.
func CarryGote(existing, incoming []byte) ([]byte, error) {
	em, err := decodeTop(existing)
	if err != nil {
		return nil, err
	}
	raw, ok := em[goteKey]
	if !ok {
		return incoming, nil
	}
	im, err := decodeTop(incoming)
	if err != nil {
		return nil, err
	}
	im[goteKey] = raw
	return marshalTop(im)
}
