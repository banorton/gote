package excal

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func load(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decode(t *testing.T, src []byte) *Doc {
	t.Helper()
	d, err := Decode(src)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func golden(t *testing.T, name string, v any) {
	t.Helper()
	got, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	p := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden %s mismatch:\n%s", name, got)
	}
}

func TestDecodeRejectsBadInput(t *testing.T) {
	for name, src := range map[string]string{
		"not json":   "{nope",
		"wrong type": `{"type":"excalidraw-lib","elements":[]}`,
		"no type":    `{"elements":[]}`,
	} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestSummaryGolden(t *testing.T) {
	golden(t, "valid.summary.golden.json", Summarize(decode(t, load(t, "valid.excalidraw"))))
	golden(t, "frames.summary.golden.json", Summarize(decode(t, load(t, "frames.excalidraw"))))
}

func TestSummaryIgnoresDeleted(t *testing.T) {
	s := Summarize(decode(t, load(t, "valid.excalidraw")))
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "Ghost") || strings.Contains(string(b), "rect-del") || strings.Contains(string(b), "arrow-del") {
		t.Fatalf("deleted element leaked: %s", b)
	}
}

func TestParseNote(t *testing.T) {
	n := ParseNote("diagrams/261114203015_diagram_x.excalidraw", load(t, "valid.excalidraw"))
	if n.Kind != "diagram" || n.Title != "261114203015_diagram_x" || n.ParseErr != "" {
		t.Fatalf("%+v", n)
	}
	if n.Stem != "261114203015_diagram_x" {
		t.Fatalf("stem %q", n.Stem)
	}
	want := "Client\nServer\nrequest\nCache\nsee [[Design Doc]]"
	if n.Body != want {
		t.Fatalf("body %q want %q", n.Body, want)
	}
	var got []string
	for _, l := range n.Links {
		got = append(got, l.Kind+":"+l.Target)
	}
	if !reflect.DeepEqual(got, []string{"wiki:Design Doc", "wiki:server notes"}) {
		t.Fatalf("links %v", got)
	}
	if !n.Semantic {
		t.Fatal("semantic should default true")
	}
}

func TestParseNoteInvalidJSON(t *testing.T) {
	n := ParseNote("x.excalidraw", []byte("{broken"))
	if n.ParseErr == "" || n.Kind != "diagram" || n.Title != "x" {
		t.Fatalf("%+v", n)
	}
}

func TestCheckValidFixturesClean(t *testing.T) {
	for _, f := range []string{"valid.excalidraw", "frames.excalidraw"} {
		if iss := Check(decode(t, load(t, f))); len(iss) != 0 {
			t.Errorf("%s: %v", f, iss)
		}
	}
}

func TestNewSceneRoundTrips(t *testing.T) {
	d := decode(t, NewScene())
	if len(d.Elements) != 0 || len(Check(d)) != 0 {
		t.Fatalf("%+v", d)
	}
}

func rect(id string, bound string) string {
	return `{"id":"` + id + `","type":"rectangle","x":0,"y":0,"boundElements":` + bound + `}`
}

func txt(id, container string) string {
	c := "null"
	if container != "" {
		c = `"` + container + `"`
	}
	return `{"id":"` + id + `","type":"text","x":0,"y":0,"text":"` + id + `","containerId":` + c + `}`
}

func arr(id, start, end string) string {
	b := func(s string) string {
		if s == "" {
			return "null"
		}
		return `{"elementId":"` + s + `","focus":0,"gap":1}`
	}
	return `{"id":"` + id + `","type":"arrow","x":0,"y":0,"startBinding":` + b(start) + `,"endBinding":` + b(end) + `}`
}

func scene(elems ...string) []byte {
	return []byte(`{"type":"excalidraw","version":2,"elements":[` + strings.Join(elems, ",") + `],"appState":{},"files":{}}`)
}

func TestCheckCorruptions(t *testing.T) {
	cases := []struct {
		name  string
		src   []byte
		elem  string
		cause string
	}{
		{"arrow start missing", scene(arr("a1", "ghost", "")), "a1", "startBinding"},
		{"arrow end deleted", scene(arr("a1", "", "r1"), `{"id":"r1","type":"rectangle","isDeleted":true,"boundElements":[{"id":"a1","type":"arrow"}]}`), "a1", "endBinding"},
		{"arrow not listed by target", scene(arr("a1", "r1", ""), rect("r1", "null")), "a1", "boundElements"},
		{"boundElements points to missing", scene(rect("r1", `[{"id":"gone","type":"text"}]`)), "r1", "gone"},
		{"boundElements points to deleted", scene(rect("r1", `[{"id":"t1","type":"text"}]`), `{"id":"t1","type":"text","isDeleted":true,"containerId":"r1"}`), "r1", "t1"},
		{"bound text does not point back", scene(rect("r1", `[{"id":"t1","type":"text"}]`), txt("t1", "")), "r1", "t1"},
		{"bound arrow does not point back", scene(rect("r1", `[{"id":"a1","type":"arrow"}]`), arr("a1", "", "")), "r1", "a1"},
		{"text container missing", scene(txt("t1", "nowhere")), "t1", "containerId"},
		{"text container does not list it", scene(rect("r1", "null"), txt("t1", "r1")), "t1", "containerId"},
		{"text container deleted", scene(`{"id":"r1","type":"rectangle","isDeleted":true,"boundElements":[{"id":"t1","type":"text"}]}`, txt("t1", "r1")), "t1", "containerId"},
	}
	for _, c := range cases {
		iss := Check(decode(t, c.src))
		found := false
		for _, i := range iss {
			if i.Element == c.elem && strings.Contains(i.Problem, c.cause) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want issue on %s mentioning %q, got %v", c.name, c.elem, c.cause, iss)
		}
	}
}

func TestCheckIgnoresDeletedSources(t *testing.T) {
	src := scene(`{"id":"r1","type":"rectangle","isDeleted":true,"boundElements":[{"id":"gone","type":"text"}]}`)
	if iss := Check(decode(t, src)); len(iss) != 0 {
		t.Fatal(iss)
	}
}
