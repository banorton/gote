package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/excal/render"
)

func TestServerRerendersChangedDrawing(t *testing.T) {
	d := newDirs(t)
	t.Setenv("XDG_STATE_HOME", d.state)
	cache := filepath.Join(d.state, "gote", "renders")
	path := filepath.Join(d.notes, "a.excalidraw")
	v1 := excal.NewScene()
	if err := os.WriteFile(path, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	start(t, d.opts("v1"))

	wait := func(src []byte, what string) {
		t.Helper()
		want := filepath.Join(cache, render.Hash(src)+".png")
		for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
			if _, err := os.Stat(want); err == nil {
				return
			}
		}
		t.Fatalf("%s: %s not rendered within 3s", what, want)
	}
	wait(v1, "initial")

	v2 := []byte(`{"type":"excalidraw","elements":[{"id":"r","type":"rectangle","x":0,"y":0,"width":80,"height":40}]}`)
	if err := os.WriteFile(path, v2, 0o644); err != nil {
		t.Fatal(err)
	}
	wait(v2, "after edit")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(cache, render.Hash(v1)+".png")); err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("old render not pruned")
}
