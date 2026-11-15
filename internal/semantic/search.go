package semantic

import (
	"context"
	"path/filepath"
	"sort"
	"sync"

	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/store"
)

const snippetLen = 200

type entry struct {
	noteID string
	line   int
	text   string
	vec    []float32
}

// The cache is process-wide and keyed by store; a long-lived server reuses it
// between syncs, a one-shot CLI process loads it once.
var cache struct {
	mu      sync.Mutex
	st      *store.Store
	model   string
	entries []entry
	valid   bool
}

func invalidate() {
	cache.mu.Lock()
	cache.valid = false
	cache.entries = nil
	cache.mu.Unlock()
}

func entries(ctx context.Context, env *ops.Env, model string) ([]entry, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.valid && cache.st == env.Store && cache.model == model {
		return cache.entries, nil
	}
	rows, err := env.Store.DB.QueryContext(ctx, `SELECT c.note_id, c.start_line, c.text, e.dim, e.vec
		FROM chunks c JOIN embeddings e ON e.hash = c.hash AND e.model = ?`, model)
	if err != nil {
		return nil, internal("load embeddings: %v", err)
	}
	defer rows.Close()
	var out []entry
	for rows.Next() {
		var e entry
		var dim int
		var blob []byte
		if err := rows.Scan(&e.noteID, &e.line, &e.text, &dim, &blob); err != nil {
			return nil, internal("load embeddings: %v", err)
		}
		if e.vec, err = decodeVec(blob, dim); err != nil {
			return nil, internal("%v", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, internal("load embeddings: %v", err)
	}
	cache.st, cache.model, cache.entries, cache.valid = env.Store, model, out, true
	return out, nil
}

func snippet(s string) string {
	r := []rune(s)
	if len(r) > snippetLen {
		r = r[:snippetLen]
	}
	return string(r)
}

// Search embeds q and returns the best chunk per note, nearest first.
func Search(ctx context.Context, env *ops.Env, q string, limit int) ([]search.Result, error) {
	cfg := env.Cfg.Semantic
	if !cfg.Enabled() {
		return nil, notConfigured()
	}
	emb, err := NewEmbedder(cfg)
	if err != nil {
		return nil, ops.Errorf(ops.CodeInvalid, "%v", err)
	}
	es, err := entries(ctx, env, emb.Model())
	if err != nil {
		return nil, err
	}
	if len(es) == 0 {
		return nil, ops.Errorf(ops.CodeInvalid, "no embeddings yet; run gote embed")
	}
	qv, err := emb.Embed(ctx, []string{q})
	if err != nil {
		return nil, internal("embed query: %v", err)
	}
	if len(qv) != 1 {
		return nil, internal("embedder returned %d vectors for the query", len(qv))
	}
	if err := normalize(qv[0]); err != nil {
		return nil, internal("embed query: %v", err)
	}
	best := map[string]*scored{}
	for i := range es {
		e := &es[i]
		if len(e.vec) != len(qv[0]) {
			return nil, ops.Errorf(ops.CodeInvalid, "embedding dimension mismatch: query has %d, stored has %d; the model output changed, rerun gote embed after clearing it", len(qv[0]), len(e.vec))
		}
		var dot float32
		for j, x := range e.vec {
			dot += x * qv[0][j]
		}
		if b := best[e.noteID]; b == nil || dot > b.score {
			best[e.noteID] = &scored{e, dot}
		}
	}
	ranked := make([]*scored, 0, len(best))
	for _, b := range best {
		ranked = append(ranked, b)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].e.noteID < ranked[j].e.noteID
	})
	out := []search.Result{}
	for _, b := range ranked {
		if len(out) >= limit {
			break
		}
		var title, kind string
		err := env.Store.DB.QueryRowContext(ctx, "SELECT title, kind FROM notes WHERE id=?", b.e.noteID).Scan(&title, &kind)
		if err != nil {
			continue
		}
		out = append(out, search.Result{
			ID: b.e.noteID, Title: title, Kind: kind,
			Path:    filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(b.e.noteID)),
			Snippet: snippet(b.e.text), Line: b.e.line, Score: float64(b.score),
		})
	}
	return out, nil
}

type scored struct {
	e     *entry
	score float32
}
