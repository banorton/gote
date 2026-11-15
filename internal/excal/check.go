package excal

import "fmt"

type Issue struct {
	Element string `json:"element"`
	Problem string `json:"problem"`
}

// Check validates the two-sided bindings between elements. Deleted elements
// are neither checked nor valid targets.
func Check(d *Doc) []Issue {
	live := d.live()
	var out []Issue
	add := func(e *Element, f string, args ...any) {
		out = append(out, Issue{e.ID, fmt.Sprintf(f, args...)})
	}
	lists := func(container *Element, id string) bool {
		for _, b := range container.BoundElements {
			if b.ID == id {
				return true
			}
		}
		return false
	}

	for _, e := range d.liveInOrder() {
		for _, side := range []struct {
			name string
			b    *Binding
		}{{"startBinding", e.StartBinding}, {"endBinding", e.EndBinding}} {
			if side.b == nil {
				continue
			}
			t := live[side.b.ElementID]
			switch {
			case t == nil:
				add(e, "%s points to missing or deleted element %q", side.name, side.b.ElementID)
			case !lists(t, e.ID):
				add(e, "%s target %q does not list this element in boundElements", side.name, t.ID)
			}
		}

		for _, b := range e.BoundElements {
			t := live[b.ID]
			switch {
			case t == nil:
				add(e, "boundElements entry %q is missing or deleted", b.ID)
			case !pointsBack(t, e.ID):
				add(e, "boundElements entry %q does not point back to this element", b.ID)
			}
		}

		if e.Type == "text" && e.ContainerID != "" {
			c := live[e.ContainerID]
			switch {
			case c == nil:
				add(e, "containerId %q is missing or deleted", e.ContainerID)
			case !lists(c, e.ID):
				add(e, "containerId %q does not list this text in boundElements", e.ContainerID)
			}
		}
	}
	return out
}

func pointsBack(t *Element, id string) bool {
	if t.Type == "text" {
		return t.ContainerID == id
	}
	return (t.StartBinding != nil && t.StartBinding.ElementID == id) ||
		(t.EndBinding != nil && t.EndBinding.ElementID == id)
}
