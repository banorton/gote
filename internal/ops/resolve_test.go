package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/store"
)

func resolveEnv(t *testing.T, files map[string]string) *Env {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ix := index.New(st, dir)
	if _, err := ix.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	return &Env{Cfg: &config.Config{NotesDir: dir}, Store: st, Index: ix, Now: time.Now}
}

func TestResolveNote(t *testing.T) {
	env := resolveEnv(t, map[string]string{
		"alpha.md":         "a",
		"dir/beta.md":      "b",
		"deep/dir/beta.md": "b2",
		"Gamma.md":         "---\naliases: [Gee, other/thing]\n---\n",
	})
	ctx := context.Background()
	cases := map[string]string{
		"alpha.md":      "alpha.md",
		"alpha":         "alpha.md",
		"ALPHA":         "alpha.md",
		"gamma":         "Gamma.md",
		"gee":           "Gamma.md",
		"dir/beta":      "dir/beta.md",
		"deep/dir/beta": "deep/dir/beta.md",
		"beta":          "dir/beta.md",
		"dir/beta.md":   "dir/beta.md",
		"./dir/beta":    "dir/beta.md",
	}
	for ref, want := range cases {
		got, err := ResolveNote(ctx, env, ref)
		if err != nil || got != want {
			t.Errorf("ResolveNote(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
}

func TestResolveNoteMissing(t *testing.T) {
	env := resolveEnv(t, map[string]string{"alpha.md": "a"})
	for _, ref := range []string{"nope", "x/alpha", "-"} {
		_, err := ResolveNote(context.Background(), env, ref)
		var oe *Error
		if !errors.As(err, &oe) || oe.Code != CodeNotFound {
			t.Errorf("%q: want not_found, got %v", ref, err)
		}
	}
	if _, err := ResolveNote(context.Background(), env, " "); err == nil {
		t.Error("empty ref must fail")
	}
}

func TestResolveNoteLastVisited(t *testing.T) {
	env := resolveEnv(t, map[string]string{"alpha.md": "a", "beta.md": "b"})
	ctx := context.Background()
	if _, err := env.Store.DB.Exec("INSERT INTO visits VALUES('alpha.md', 10),('beta.md', 20),('gone.md', 30)"); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveNote(ctx, env, "-")
	if err != nil || got != "beta.md" {
		t.Fatalf("got %q %v", got, err)
	}
}
