package semantic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/embed"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	opsearch "github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/semantic"
	"github.com/banorton/gote/internal/store"
)

type fakeEmb struct{}

func (fakeEmb) Model() string { return "f" }
func (fakeEmb) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v := []float32{0.01, 0, 0}
		if strings.Contains(t, "apple") {
			v[1] = 1
		}
		if strings.Contains(t, "pear") {
			v[2] = 1
		}
		out[i] = v
	}
	return out, nil
}

func setup(t *testing.T, sem config.Semantic) client.Client {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("apple pie\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("pear tart\n"), 0o644)
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ix := index.New(st, dir)
	if _, err := ix.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	old := semantic.NewEmbedder
	semantic.NewEmbedder = func(config.Semantic) (embed.Embedder, error) { return fakeEmb{}, nil }
	t.Cleanup(func() { semantic.NewEmbedder = old })
	return client.Local(&ops.Env{Cfg: &config.Config{NotesDir: dir, Semantic: sem}, Store: st, Index: ix, Now: time.Now})
}

var enabled = config.Semantic{Provider: "command", Model: "f", EmbedCmd: "x", BatchSize: 8}

func TestSyncStatusAndSemanticQuery(t *testing.T) {
	c := setup(t, enabled)
	ctx := context.Background()
	var st StatusResp
	if err := c.Call(ctx, "semantic.status", nil, &st); err != nil || !st.Configured || st.Chunks != 2 || st.Pending != 2 || st.Provider != "command" || st.Model != "f" {
		t.Fatalf("%v %+v", err, st)
	}
	var q opsearch.QueryResp
	err := c.Call(ctx, "search.query", opsearch.QueryReq{Q: "pear", Semantic: true}, &q)
	if err == nil || !strings.Contains(err.Error(), "run gote embed") {
		t.Fatalf("%v", err)
	}
	var sy SyncResp
	err = c.Call(ctx, "semantic.sync", SyncReq{}, &sy)
	if err == nil || !strings.Contains(err.Error(), "first embed: 2 chunks") {
		t.Fatalf("want first-embed refusal: %v", err)
	}
	if err := c.Call(ctx, "semantic.sync", SyncReq{Confirm: true}, &sy); err != nil || sy.Embedded != 2 || sy.Chunks != 2 || sy.Pending != 0 {
		t.Fatalf("%v %+v", err, sy)
	}
	if err := c.Call(ctx, "search.query", opsearch.QueryReq{Q: "pear", Semantic: true}, &q); err != nil || len(q.Results) == 0 || q.Results[0].ID != "b.md" {
		t.Fatalf("%v %+v", err, q)
	}
	if err := c.Call(ctx, "semantic.status", nil, &st); err != nil || st.Embedded != 2 || st.Pending != 0 {
		t.Fatalf("%v %+v", err, st)
	}
}

func TestUnconfigured(t *testing.T) {
	c := setup(t, config.Semantic{})
	ctx := context.Background()
	var st StatusResp
	if err := c.Call(ctx, "semantic.status", nil, &st); err != nil || st.Configured {
		t.Fatalf("%v %+v", err, st)
	}
	var sy SyncResp
	if err := c.Call(ctx, "semantic.sync", SyncReq{}, &sy); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("%v", err)
	}
	var q opsearch.QueryResp
	if err := c.Call(ctx, "search.query", opsearch.QueryReq{Q: "x", Semantic: true}, &q); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("%v", err)
	}
}
