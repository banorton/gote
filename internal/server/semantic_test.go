package server

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/embed"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/semantic"
	"github.com/banorton/gote/internal/store"
)

type countEmb struct{ n *atomic.Int32 }

func (countEmb) Model() string { return "fake" }
func (c countEmb) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		c.n.Add(1)
		out[i] = []float32{1, 0}
	}
	return out, nil
}

func TestServerBackgroundEmbedsNewFile(t *testing.T) {
	d := newDirs(t)
	var n atomic.Int32
	old := semantic.NewEmbedder
	semantic.NewEmbedder = func(config.Semantic) (embed.Embedder, error) { return countEmb{&n}, nil }
	t.Cleanup(func() { semantic.NewEmbedder = old })

	o := d.opts("v1")
	o.Open = func() (*ops.Env, func(), error) {
		st, err := store.Open(filepath.Join(d.state, "gote.db"))
		if err != nil {
			return nil, nil, err
		}
		cfg := &config.Config{NotesDir: d.notes, Semantic: config.Semantic{Provider: "command", Model: "fake", EmbedCmd: "x", BatchSize: 8}}
		return &ops.Env{Cfg: cfg, Store: st, Index: index.New(st, d.notes), Now: time.Now}, func() { st.Close() }, nil
	}
	start(t, o)

	if err := os.WriteFile(filepath.Join(d.notes, "new.md"), []byte("fresh note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3500 * time.Millisecond)
	for n.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if n.Load() != 1 {
		t.Fatalf("new file not embedded within ~3s: embeds=%d", n.Load())
	}
}
