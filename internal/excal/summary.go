package excal

import (
	"sort"
	"strings"
)

type Shape struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label"`
}

type Text struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type Edge struct {
	ID    string  `json:"id"`
	From  *string `json:"from"`
	To    *string `json:"to"`
	Label string  `json:"label"`
}

type Summary struct {
	Shapes []Shape `json:"shapes"`
	Texts  []Text  `json:"texts"`
	Edges  []Edge  `json:"edges"`
}

var shapeTypes = map[string]bool{"rectangle": true, "ellipse": true, "diamond": true, "frame": true, "image": true}

func isEdge(t string) bool { return t == "arrow" || t == "line" }

// Summarize lists shapes, standalone texts and edges in reading order
// (top-to-bottom, then left-to-right). Deleted elements are ignored.
func Summarize(d *Doc) Summary {
	live := d.live()
	els := d.liveInOrder()
	sort.SliceStable(els, func(i, j int) bool {
		a, b := els[i], els[j]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.ID < b.ID
	})

	labels := map[string][]string{}
	for _, e := range els {
		if e.Type == "text" && e.ContainerID != "" && live[e.ContainerID] != nil {
			labels[e.ContainerID] = append(labels[e.ContainerID], e.Text)
		}
	}
	label := func(id string) string { return strings.Join(labels[id], "\n") }
	endpoint := func(b *Binding) *string {
		if b == nil {
			return nil
		}
		t := live[b.ElementID]
		if t == nil {
			return nil
		}
		s := label(t.ID)
		if s == "" {
			s = t.ID
		}
		return &s
	}

	s := Summary{Shapes: []Shape{}, Texts: []Text{}, Edges: []Edge{}}
	for _, e := range els {
		switch {
		case shapeTypes[e.Type]:
			s.Shapes = append(s.Shapes, Shape{e.ID, e.Type, label(e.ID)})
		case e.Type == "text":
			if e.ContainerID == "" || live[e.ContainerID] == nil {
				s.Texts = append(s.Texts, Text{e.ID, e.Text})
			}
		case isEdge(e.Type):
			s.Edges = append(s.Edges, Edge{e.ID, endpoint(e.StartBinding), endpoint(e.EndBinding), label(e.ID)})
		}
	}
	return s
}
