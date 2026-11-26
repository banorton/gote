package diagram

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/excal/render"
	"github.com/banorton/gote/internal/ops"
)

type renderResp struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Fresh  bool   `json:"fresh"`
}

func callRender(t *testing.T, env *ops.Env, id string) (renderResp, error) {
	t.Helper()
	var r renderResp
	err := client.Local(env).Call(context.Background(), "diagram.render", map[string]string{"id": id}, &r)
	return r, err
}

func stateDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("XDG_STATE_HOME", d)
	return filepath.Join(d, "gote", "renders")
}

func renders(t *testing.T, dir string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	return m
}

func TestRenderCacheMissHitStale(t *testing.T) {
	cache := stateDir(t)
	dir, env, _ := setup(t)
	writeFixture(t, dir, "d/a.excalidraw", "valid.excalidraw")

	r1, err := callRender(t, env, "d/a.excalidraw")
	if err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(filepath.Join(dir, "d", "a.excalidraw"))
	if want := filepath.Join(cache, render.Hash(src)+".png"); r1.Path != want || !r1.Fresh || r1.Width < 64 || r1.Height < 64 {
		t.Fatalf("%+v, want path %s", r1, want)
	}
	if _, err := os.Stat(r1.Path); err != nil {
		t.Fatal(err)
	}

	r2, err := callRender(t, env, "d/a.excalidraw")
	if err != nil || r2.Fresh || r2.Path != r1.Path || r2.Width != r1.Width {
		t.Fatalf("hit: %+v %v", r2, err)
	}

	os.WriteFile(filepath.Join(dir, "d", "a.excalidraw"), edited(src), 0o644)
	r3, err := callRender(t, env, "d/a.excalidraw")
	if err != nil || !r3.Fresh || r3.Path == r1.Path {
		t.Fatalf("stale: %+v %v", r3, err)
	}
}

func TestRenderRebuildsCorruptCacheEntry(t *testing.T) {
	stateDir(t)
	dir, env, _ := setup(t)
	writeFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	r1, _ := callRender(t, env, "a.excalidraw")
	os.WriteFile(r1.Path, []byte("junk"), 0o644)
	r2, err := callRender(t, env, "a.excalidraw")
	if err != nil || !r2.Fresh {
		t.Fatalf("%+v %v", r2, err)
	}
}

func TestRenderSidecarWritesNextToDrawing(t *testing.T) {
	cache := stateDir(t)
	dir, env, _ := setup(t)
	env.Cfg.DiagramPNG = "sidecar"
	writeFixture(t, dir, "d/a.excalidraw", "valid.excalidraw")
	r, err := callRender(t, env, "d/a.excalidraw")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "d", "a.png"); r.Path != want || !r.Fresh {
		t.Fatalf("%+v, want %s", r, want)
	}
	if n := len(renders(t, cache)); n != 0 {
		t.Fatalf("sidecar mode wrote %d cache entries", n)
	}
	if r2, _ := callRender(t, env, "d/a.excalidraw"); r2.Fresh {
		t.Fatal("second render should hit")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "d", "a.excalidraw"))
	os.WriteFile(filepath.Join(dir, "d", "a.excalidraw"), edited(b), 0o644)
	if r3, _ := callRender(t, env, "d/a.excalidraw"); !r3.Fresh || r3.Path != r.Path {
		t.Fatalf("stale sidecar not rebuilt: %+v", r3)
	}
}

func TestRenderOffStillRendersOnDemandIntoCache(t *testing.T) {
	cache := stateDir(t)
	dir, env, _ := setup(t)
	env.Cfg.DiagramPNG = "off"
	writeFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	r, err := callRender(t, env, "a.excalidraw")
	if err != nil || filepath.Dir(r.Path) != cache {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestRenderErrors(t *testing.T) {
	stateDir(t)
	dir, env, _ := setup(t)
	os.WriteFile(filepath.Join(dir, "n.md"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.excalidraw"), []byte(`{"type":"excalidraw","elements":[{"id":"q","type":"rectangle","width":4,"height":4,"strokeColor":"nope"}]}`), 0o644)
	for id, code := range map[string]string{"missing.excalidraw": ops.CodeNotFound, "n.md": ops.CodeInvalid, "bad.excalidraw": ops.CodeInvalid, "../x.excalidraw": ops.CodeInvalid} {
		_, err := callRender(t, env, id)
		if oe, ok := err.(*ops.Error); !ok || oe.Code != code {
			t.Errorf("%s: %v, want %s", id, err, code)
		}
	}
}

func TestPrune(t *testing.T) {
	cache := stateDir(t)
	dir, env, _ := setup(t)
	writeFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	writeFixture(t, dir, "b.excalidraw", "frames.excalidraw")
	if _, err := env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	ra, _ := callRender(t, env, "a.excalidraw")
	rb, _ := callRender(t, env, "b.excalidraw")
	os.WriteFile(filepath.Join(cache, strings.Repeat("0", 32)+".png"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(cache, "notes.txt"), []byte("keep"), 0o644)

	if err := Prune(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	got := renders(t, cache)
	if len(got) != 2 {
		t.Fatalf("after prune: %v", got)
	}
	if _, err := os.Stat(filepath.Join(cache, "notes.txt")); err != nil {
		t.Fatal("prune touched a non-render file")
	}

	os.Remove(filepath.Join(dir, "b.excalidraw"))
	env.Index.RemoveFile("b.excalidraw")
	if err := Prune(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(rb.Path); err == nil {
		t.Fatal("render of deleted drawing survived")
	}
	if _, err := os.Stat(ra.Path); err != nil {
		t.Fatal("current render pruned")
	}

	env.Cfg.DiagramPNG = "sidecar"
	if err := Prune(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if n := len(renders(t, cache)); n != 0 {
		t.Fatalf("sidecar mode keeps %d cache entries", n)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end); time.Sleep(25 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestBackgroundRendersChangedDrawings(t *testing.T) {
	cache := stateDir(t)
	dir, env, _ := setup(t)
	writeFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	if _, err := env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	var logs []string
	stop := StartBackground(context.Background(), env, func(f string, a ...any) { logs = append(logs, f) })
	defer stop()
	waitFor(t, "initial render", func() bool { return len(renders(t, cache)) == 1 })

	b, _ := os.ReadFile(filepath.Join(dir, "a.excalidraw"))
	os.WriteFile(filepath.Join(dir, "a.excalidraw"), edited(b), 0o644)
	if err := env.Index.IndexFile("a.excalidraw"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "re-render and prune of old hash", func() bool {
		m := renders(t, cache)
		if len(m) != 1 {
			return false
		}
		return filepath.Base(m[0]) == render.Hash(edited(b))+".png"
	})
}

func TestBackgroundOffRendersNothing(t *testing.T) {
	cache := stateDir(t)
	dir, env, _ := setup(t)
	env.Cfg.DiagramPNG = "off"
	writeFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	if err := env.Index.IndexFile("a.excalidraw"); err != nil {
		t.Fatal(err)
	}
	stop := StartBackground(context.Background(), env, t.Logf)
	time.Sleep(Debounce*3 + 200*time.Millisecond)
	stop()
	if n := len(renders(t, cache)); n != 0 {
		t.Fatalf("off pre-rendered %d", n)
	}
}

func TestBackgroundSidecar(t *testing.T) {
	stateDir(t)
	dir, env, _ := setup(t)
	env.Cfg.DiagramPNG = "sidecar"
	writeFixture(t, dir, "a.excalidraw", "valid.excalidraw")
	if err := env.Index.IndexFile("a.excalidraw"); err != nil {
		t.Fatal(err)
	}
	stop := StartBackground(context.Background(), env, t.Logf)
	defer stop()
	waitFor(t, "sidecar", func() bool { _, err := os.Stat(filepath.Join(dir, "a.png")); return err == nil })
}

func edited(src []byte) []byte { return append(append([]byte{}, src...), '\n') }
