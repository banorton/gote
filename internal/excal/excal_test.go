package excal

import (
	"bytes"
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

const withDesc = `{"type":"excalidraw","version":2,"gote":{"description":"login flow overview"},"elements":[` +
	`{"id":"t1","type":"text","x":0,"y":0,"text":"Client"}],"appState":{},"files":{},"custom":{"keep":1}}`

func TestParseNoteIndexesDescription(t *testing.T) {
	n := ParseNote("d.excalidraw", []byte(withDesc))
	if !strings.Contains(n.Body, "Client") || !strings.Contains(n.Body, "login flow overview") {
		t.Fatalf("body %q", n.Body)
	}
	plain := ParseNote("d.excalidraw", scene(txt("t1", "")))
	if plain.Body != "t1" {
		t.Fatalf("body %q", plain.Body)
	}
	only := ParseNote("d.excalidraw", []byte(`{"type":"excalidraw","gote":{"description":"just words"},"elements":[]}`))
	if only.Body != "just words" {
		t.Fatalf("body %q", only.Body)
	}
}

func TestSummarizeDescription(t *testing.T) {
	d := decode(t, []byte(withDesc))
	if got := Summarize(d).Description; got != "login flow overview" {
		t.Fatalf("%q", got)
	}
}

func TestCheckIgnoresDescription(t *testing.T) {
	if iss := Check(decode(t, []byte(withDesc))); len(iss) != 0 {
		t.Fatal(iss)
	}
}

func TestSetDescriptionKeepsEverythingElse(t *testing.T) {
	out, err := SetDescription([]byte(withDesc), "new text <b>&")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"type", "version", "elements", "appState", "files", "custom", "gote"} {
		if _, ok := m[k]; !ok {
			t.Errorf("lost key %s", k)
		}
	}
	var cb bytes.Buffer
	json.Compact(&cb, m["custom"])
	if cb.String() != `{"keep":1}` {
		t.Fatalf("custom %s", m["custom"])
	}
	if got := Summarize(decode(t, out)).Description; got != "new text <b>&" {
		t.Fatalf("%q", got)
	}
	if strings.Contains(string(out), `<`) {
		t.Fatal("html escaped")
	}
	cleared, err := SetDescription(out, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cleared), `"gote"`) || !strings.Contains(string(cleared), `"custom"`) {
		t.Fatalf("clear: %s", cleared)
	}
}

func TestSetDescriptionKeepsOtherGoteKeys(t *testing.T) {
	src := `{"type":"excalidraw","gote":{"description":"a","future":true},"elements":[]}`
	out, err := SetDescription([]byte(src), "b")
	if err != nil || !strings.Contains(string(out), `"future": true`) {
		t.Fatalf("%v %s", err, out)
	}
	out, err = SetDescription(out, "")
	if err != nil || !strings.Contains(string(out), `"future"`) {
		t.Fatalf("%v %s", err, out)
	}
}

func TestCarryGote(t *testing.T) {
	browser := scene(txt("t1", ""))
	out, err := CarryGote([]byte(withDesc), browser)
	if err != nil {
		t.Fatal(err)
	}
	if Summarize(decode(t, out)).Description != "login flow overview" {
		t.Fatalf("%s", out)
	}
	same, err := CarryGote([]byte(`{"type":"excalidraw","elements":[]}`), browser)
	if err != nil || string(same) != string(browser) {
		t.Fatalf("no-gote must pass body through byte-for-byte: %v %s", err, same)
	}
}
