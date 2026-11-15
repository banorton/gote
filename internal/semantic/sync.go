package semantic

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/embed"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/parse"
)

// NewEmbedder builds the configured embedder; tests replace it with a fake.
var NewEmbedder = embed.New

type Stats struct {
	Chunks, Embedded, Pending, Tokens int
	Elapsed                           time.Duration
}

var syncMu sync.Mutex

func estTokens(s string) int { return utf8.RuneCountInString(s) / 4 }

// Cost estimates USD for tokens; ok is false when price_per_mtok is unset.
func Cost(cfg config.Semantic, tokens int) (usd float64, ok bool) {
	if cfg.PricePerMTok <= 0 {
		return 0, false
	}
	return float64(tokens) / 1e6 * cfg.PricePerMTok, true
}

func notConfigured() error {
	return ops.Errorf(ops.CodeInvalid, "semantic search not configured: add a semantic: block to the config")
}

func encodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func decodeVec(b []byte, dim int) ([]float32, error) {
	if len(b) != 4*dim {
		return nil, fmt.Errorf("corrupt embedding: %d bytes for dim %d", len(b), dim)
	}
	v := make([]float32, dim)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v, nil
}

func normalize(v []float32) error {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 || math.IsNaN(sum) || math.IsInf(sum, 0) {
		return errors.New("embedder returned a zero or non-finite vector")
	}
	n := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= n
	}
	return nil
}

// refreshChunks recomputes the chunks cache for every eligible note and
// returns hash -> embedded text.
func refreshChunks(ctx context.Context, env *ops.Env) (map[string]string, error) {
	rows, err := env.Store.DB.QueryContext(ctx, "SELECT id, kind FROM notes ORDER BY id")
	if err != nil {
		return nil, err
	}
	type nk struct{ id, kind string }
	var all []nk
	for rows.Next() {
		var x nk
		if err := rows.Scan(&x.id, &x.kind); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type noteChunks struct {
		id string
		cs []Chunk
	}
	var found []noteChunks
	texts := map[string]string{}
	for _, x := range all {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		src, err := os.ReadFile(filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(x.id)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", x.id, err)
		}
		var n note.Note
		fm := 0
		if x.kind == note.KindDiagram {
			n = excal.ParseNote(x.id, src)
		} else {
			n = parse.Note(x.id, src)
			fm = frontmatterLines(string(src))
		}
		cs := ChunkNote(n, fm, env.Cfg.Semantic.ExcludeTags)
		if len(cs) == 0 {
			continue
		}
		for _, c := range cs {
			texts[c.Hash] = c.Embed
		}
		found = append(found, noteChunks{x.id, cs})
	}

	tx, err := env.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM chunks"); err != nil {
		return nil, err
	}
	for _, nc := range found {
		for _, c := range nc.cs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO chunks(note_id, ord, heading_path, start_line, text, hash)
				SELECT ?,?,?,?,?,? WHERE EXISTS (SELECT 1 FROM notes WHERE id=?)`,
				nc.id, c.Ord, c.HeadingPath, c.StartLine, c.Text, c.Hash, nc.id); err != nil {
				return nil, err
			}
		}
	}
	return texts, tx.Commit()
}

func missingHashes(ctx context.Context, env *ops.Env, model string) ([]string, error) {
	rows, err := env.Store.DB.QueryContext(ctx, `SELECT hash FROM chunks
		WHERE hash NOT IN (SELECT hash FROM embeddings WHERE model=?) GROUP BY hash ORDER BY min(id)`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func scalarInt(ctx context.Context, env *ops.Env, q string, args ...any) (int, error) {
	var n int
	err := env.Store.DB.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

func getMeta(ctx context.Context, env *ops.Env, key string) (string, error) {
	var v string
	err := env.Store.DB.QueryRowContext(ctx, "SELECT value FROM meta WHERE key=?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func internal(format string, args ...any) error { return ops.Errorf(ops.CodeInternal, format, args...) }

// Sync brings the embeddings of the current model in line with the chunks.
// An interrupted run keeps every completed batch and resumes where it stopped.
func Sync(ctx context.Context, env *ops.Env, confirm bool) (Stats, error) {
	syncMu.Lock()
	defer syncMu.Unlock()
	defer invalidate()
	start := time.Now()
	cfg := env.Cfg.Semantic
	if !cfg.Enabled() {
		return Stats{}, notConfigured()
	}
	emb, err := NewEmbedder(cfg)
	if err != nil {
		return Stats{}, ops.Errorf(ops.CodeInvalid, "%v", err)
	}
	model := emb.Model()

	texts, err := refreshChunks(ctx, env)
	if err != nil {
		return Stats{}, internal("refresh chunks: %v", err)
	}
	total, err := scalarInt(ctx, env, "SELECT count(*) FROM chunks")
	if err != nil {
		return Stats{}, internal("count chunks: %v", err)
	}
	pending, err := missingHashes(ctx, env, model)
	if err != nil {
		return Stats{}, internal("find missing embeddings: %v", err)
	}
	st := Stats{Chunks: total, Pending: len(pending)}
	tokens := 0
	for _, h := range pending {
		tokens += estTokens(texts[h])
	}

	if len(pending) > 0 && !confirm {
		prev, err := getMeta(ctx, env, "embed_model")
		if err != nil {
			return st, internal("read meta: %v", err)
		}
		have, err := scalarInt(ctx, env, "SELECT count(*) FROM embeddings WHERE model=?", model)
		if err != nil {
			return st, internal("count embeddings: %v", err)
		}
		reason := ""
		switch {
		case prev == "" && have == 0:
			reason = "first embed"
		case prev != "" && prev != model && have == 0:
			reason = "model changed"
		}
		if reason != "" {
			cost := ""
			if usd, ok := Cost(cfg, tokens); ok {
				cost = fmt.Sprintf(", ~$%.4f", usd)
			}
			return st, ops.Errorf(ops.CodeInvalid, "%s: %d chunks (~%d tokens%s); rerun with --yes", reason, len(pending), tokens, cost)
		}
	}

	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 64
	}
	for i := 0; i < len(pending); i += batch {
		if err := ctx.Err(); err != nil {
			return finish(st, start), err
		}
		hs := pending[i:min(i+batch, len(pending))]
		in := make([]string, len(hs))
		for j, h := range hs {
			in[j] = texts[h]
		}
		vecs, err := emb.Embed(ctx, in)
		if err != nil {
			return finish(st, start), internal("embed batch %d: %v", i/batch+1, err)
		}
		if len(vecs) != len(hs) {
			return finish(st, start), internal("embedder returned %d vectors for %d texts", len(vecs), len(hs))
		}
		if err := storeBatch(ctx, env, model, hs, vecs); err != nil {
			return finish(st, start), internal("store batch %d: %v", i/batch+1, err)
		}
		st.Embedded += len(hs)
		st.Pending -= len(hs)
		for _, t := range in {
			st.Tokens += estTokens(t)
		}
	}

	if _, err := env.Store.DB.ExecContext(ctx, "DELETE FROM embeddings WHERE model=? AND hash NOT IN (SELECT hash FROM chunks)", model); err != nil {
		return finish(st, start), internal("prune embeddings: %v", err)
	}
	var dim int
	err = env.Store.DB.QueryRowContext(ctx, "SELECT dim FROM embeddings WHERE model=? LIMIT 1", model).Scan(&dim)
	if err != nil && err != sql.ErrNoRows {
		return finish(st, start), internal("read dim: %v", err)
	}
	if err == nil {
		for k, v := range map[string]string{"embed_model": model, "embed_dim": strconv.Itoa(dim)} {
			if _, err := env.Store.DB.ExecContext(ctx, "INSERT OR REPLACE INTO meta VALUES(?,?)", k, v); err != nil {
				return finish(st, start), internal("write meta: %v", err)
			}
		}
	}
	return finish(st, start), nil
}

func finish(st Stats, start time.Time) Stats {
	st.Elapsed = time.Since(start)
	return st
}

func storeBatch(ctx context.Context, env *ops.Env, model string, hashes []string, vecs [][]float32) error {
	tx, err := env.Store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, v := range vecs {
		if len(v) == 0 {
			return errors.New("embedder returned an empty vector")
		}
		if len(v) != len(vecs[0]) {
			return fmt.Errorf("embedder returned vectors of different sizes (%d and %d)", len(vecs[0]), len(v))
		}
		if err := normalize(v); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO embeddings(model, hash, dim, vec) VALUES(?,?,?,?)", model, hashes[i], len(v), encodeVec(v)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type StatusInfo struct {
	Configured bool
	Provider   string
	Model      string
	Chunks     int
	Embedded   int
	Pending    int
}

func Status(ctx context.Context, env *ops.Env) (StatusInfo, error) {
	cfg := env.Cfg.Semantic
	if !cfg.Enabled() {
		return StatusInfo{}, nil
	}
	emb, err := NewEmbedder(cfg)
	if err != nil {
		return StatusInfo{}, ops.Errorf(ops.CodeInvalid, "%v", err)
	}
	syncMu.Lock()
	defer syncMu.Unlock()
	if _, err := refreshChunks(ctx, env); err != nil {
		return StatusInfo{}, internal("refresh chunks: %v", err)
	}
	invalidate()
	s := StatusInfo{Configured: true, Provider: cfg.Provider, Model: emb.Model()}
	if s.Chunks, err = scalarInt(ctx, env, "SELECT count(*) FROM chunks"); err != nil {
		return s, internal("status: %v", err)
	}
	if s.Embedded, err = scalarInt(ctx, env, "SELECT count(*) FROM chunks WHERE hash IN (SELECT hash FROM embeddings WHERE model=?)", s.Model); err != nil {
		return s, internal("status: %v", err)
	}
	s.Pending = s.Chunks - s.Embedded
	return s, nil
}
