package semantic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/embed"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

var keywords = []string{"apple", "banana", "cherry"}

type fake struct {
	mu        sync.Mutex
	model     string
	calls     int
	texts     []string
	failAfter int
}

func (f *fake) Model() string { return f.model }

func (f *fake) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failAfter > 0 && f.calls >= f.failAfter {
		return nil, errors.New("fake embedder down")
	}
	f.calls++
	out := make([][]float32, len(texts))
	for i, t := range texts {
		f.texts = append(f.texts, t)
		v := make([]float32, len(keywords)+1)
		v[len(keywords)] = 0.01
		for j, k := range keywords {
			v[j] = float32(strings.Count(strings.ToLower(t), k))
		}
		out[i] = v
	}
	return out, nil
}

func (f *fake) embedded() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.texts) }

type vault struct {
	dir string
	env *ops.Env
	f   *fake
}

func newVault(t *testing.T, files map[string]string) *vault {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfg := &config.Config{NotesDir: dir, Semantic: config.Semantic{
		Provider: "command", Model: "fake-1", EmbedCmd: "unused", BatchSize: 2, ExcludeTags: []string{"private"}, PricePerMTok: 1,
	}}
	v := &vault{dir: dir, f: &fake{model: "fake-1"}}
	v.env = &ops.Env{Cfg: cfg, Store: st, Index: index.New(st, dir), Now: time.Now}
	for rel, c := range files {
		v.write(t, rel, c)
	}
	v.reindex(t)
	old := NewEmbedder
	NewEmbedder = func(config.Semantic) (embed.Embedder, error) { return v.f, nil }
	t.Cleanup(func() { NewEmbedder = old })
	return v
}

func (v *vault) write(t *testing.T, rel, c string) {
	t.Helper()
	p := filepath.Join(v.dir, rel)
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (v *vault) reindex(t *testing.T) {
	t.Helper()
	if _, err := v.env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func (v *vault) sync(t *testing.T, confirm bool) Stats {
	t.Helper()
	s, err := Sync(context.Background(), v.env, confirm)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	return s
}

func (v *vault) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := v.env.Store.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

var fixture = map[string]string{
	"a.md":         "# Fruit\napple apple\n# More\nbanana\n",
	"b.md":         "cherry cherry cherry\n",
	"secret.md":    "---\ntags: [private]\n---\napple\n",
	"off.md":       "---\nsemantic: false\n---\nbanana\n",
	"d.excalidraw": `{"type":"excalidraw","version":2,"elements":[{"id":"1","type":"text","text":"banana banana","x":0,"y":0,"width":1,"height":1}]}`,
}

func TestSyncFirstThenNoop(t *testing.T) {
	v := newVault(t, fixture)
	s := v.sync(t, true)
	if s.Chunks != 4 || s.Embedded != 4 || s.Pending != 0 || s.Tokens == 0 {
		t.Fatalf("%+v", s)
	}
	if v.f.calls != 2 {
		t.Fatalf("batch size 2 over 4 chunks: calls=%d", v.f.calls)
	}
	s = v.sync(t, false)
	if s.Embedded != 0 || s.Chunks != 4 || v.f.calls != 2 {
		t.Fatalf("%+v calls=%d", s, v.f.calls)
	}
	if got := v.count(t, "SELECT count(*) FROM embeddings WHERE model='fake-1'"); got != 4 {
		t.Fatalf("embeddings %d", got)
	}
}

func TestSyncVectorsNormalized(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n"})
	v.sync(t, true)
	var blob []byte
	var dim int
	if err := v.env.Store.DB.QueryRow("SELECT vec, dim FROM embeddings").Scan(&blob, &dim); err != nil {
		t.Fatal(err)
	}
	vec, err := decodeVec(blob, dim)
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, x := range vec {
		sum += float64(x) * float64(x)
	}
	if sum < 0.999 || sum > 1.001 || dim != len(keywords)+1 {
		t.Fatalf("norm^2=%v dim=%d", sum, dim)
	}
}

func TestSyncEditOneSectionEmbedsOne(t *testing.T) {
	v := newVault(t, fixture)
	v.sync(t, true)
	v.write(t, "a.md", "# Fruit\napple apple\n# More\nbanana banana\n")
	v.reindex(t)
	s := v.sync(t, false)
	if s.Embedded != 1 {
		t.Fatalf("%+v", s)
	}
	if got := v.count(t, "SELECT count(*) FROM embeddings"); got != 4 {
		t.Fatalf("stale vector not pruned: %d", got)
	}
}

func TestSyncDeleteNotePrunes(t *testing.T) {
	v := newVault(t, fixture)
	v.sync(t, true)
	os.Remove(filepath.Join(v.dir, "b.md"))
	v.reindex(t)
	s := v.sync(t, false)
	if s.Embedded != 0 || s.Chunks != 3 {
		t.Fatalf("%+v", s)
	}
	if got := v.count(t, "SELECT count(*) FROM embeddings"); got != 3 {
		t.Fatalf("embeddings %d", got)
	}
}

func TestSyncModelChangeNeedsConfirm(t *testing.T) {
	v := newVault(t, fixture)
	v.sync(t, true)
	v.f.model = "fake-2"
	_, err := Sync(context.Background(), v.env, false)
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeInvalid || !strings.Contains(oe.Msg, "model changed: 4 chunks") || !strings.Contains(oe.Msg, "--yes") || !strings.Contains(oe.Msg, "$") {
		t.Fatalf("%v", err)
	}
	if v.count(t, "SELECT count(*) FROM embeddings WHERE model='fake-2'") != 0 {
		t.Fatal("embedded without confirm")
	}
	s := v.sync(t, true)
	if s.Embedded != 4 {
		t.Fatalf("%+v", s)
	}
	if v.count(t, "SELECT count(*) FROM embeddings WHERE model='fake-1'") != 4 {
		t.Fatal("old model rows should be kept")
	}
	if _, err := Sync(context.Background(), v.env, false); err != nil {
		t.Fatalf("model already has rows, no confirm needed: %v", err)
	}
}

func TestSyncInterruptedKeepsBatches(t *testing.T) {
	v := newVault(t, fixture)
	v.f.failAfter = 1
	_, err := Sync(context.Background(), v.env, true)
	if err == nil || !strings.Contains(err.Error(), "fake embedder down") {
		t.Fatalf("%v", err)
	}
	if got := v.count(t, "SELECT count(*) FROM embeddings"); got != 2 {
		t.Fatalf("batch 1 rows lost: %d", got)
	}
	v.f.failAfter = 0
	s := v.sync(t, false)
	if s.Embedded != 2 || s.Pending != 0 {
		t.Fatalf("resume: %+v", s)
	}
}

func TestSyncDuplicateTextEmbeddedOnce(t *testing.T) {
	v := newVault(t, map[string]string{"x.md": "---\ntitle: Same\n---\napple\n", "y.md": "---\ntitle: Same\n---\napple\n"})
	s := v.sync(t, true)
	if s.Chunks != 2 || s.Embedded != 1 {
		t.Fatalf("%+v", s)
	}
}

func TestSyncUnconfigured(t *testing.T) {
	v := newVault(t, fixture)
	v.env.Cfg.Semantic = config.Semantic{}
	_, err := Sync(context.Background(), v.env, false)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("%v", err)
	}
}

func TestStatus(t *testing.T) {
	v := newVault(t, fixture)
	st, err := Status(context.Background(), v.env)
	if err != nil || !st.Configured || st.Provider != "command" || st.Model != "fake-1" || st.Chunks != 4 || st.Embedded != 0 || st.Pending != 4 {
		t.Fatalf("%v %+v", err, st)
	}
	v.sync(t, true)
	st, _ = Status(context.Background(), v.env)
	if st.Embedded != 4 || st.Pending != 0 {
		t.Fatalf("%+v", st)
	}
	v.env.Cfg.Semantic = config.Semantic{}
	st, err = Status(context.Background(), v.env)
	if err != nil || st.Configured {
		t.Fatalf("%v %+v", err, st)
	}
}

func TestSearchNearestNote(t *testing.T) {
	v := newVault(t, fixture)
	v.sync(t, true)
	rs, err := Search(context.Background(), v.env, "cherry", 10)
	if err != nil || len(rs) == 0 || rs[0].ID != "b.md" {
		t.Fatalf("%v %+v", err, rs)
	}
	r := rs[0]
	if r.Kind != "note" || r.Path != filepath.Join(v.dir, "b.md") || r.Line != 1 || !strings.HasPrefix(r.Snippet, "cherry") || r.Score < 0.9 {
		t.Fatalf("%+v", r)
	}
	rs, _ = Search(context.Background(), v.env, "banana", 10)
	if rs[0].ID != "d.excalidraw" && rs[0].ID != "a.md" {
		t.Fatalf("%+v", rs)
	}
	seen := map[string]bool{}
	for _, r := range rs {
		if seen[r.ID] {
			t.Fatalf("duplicate note %s", r.ID)
		}
		seen[r.ID] = true
	}
	rs, _ = Search(context.Background(), v.env, "apple", 1)
	if len(rs) != 1 || rs[0].ID != "a.md" || rs[0].Line != 1 {
		t.Fatalf("%+v", rs)
	}
}

func TestSearchErrors(t *testing.T) {
	v := newVault(t, fixture)
	_, err := Search(context.Background(), v.env, "apple", 5)
	if err == nil || !strings.Contains(err.Error(), "no embeddings yet; run gote embed") {
		t.Fatalf("%v", err)
	}
	v.env.Cfg.Semantic = config.Semantic{}
	_, err = Search(context.Background(), v.env, "apple", 5)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("%v", err)
	}
}

func TestSearchCacheInvalidatedBySync(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n"})
	v.sync(t, true)
	if rs, _ := Search(context.Background(), v.env, "apple", 5); len(rs) != 1 {
		t.Fatalf("%+v", rs)
	}
	v.write(t, "c.md", "apple apple\n")
	v.reindex(t)
	v.sync(t, false)
	rs, _ := Search(context.Background(), v.env, "apple", 5)
	if len(rs) != 2 {
		t.Fatalf("cache stale: %+v", rs)
	}
}

func TestBackgroundEmbedsNewFile(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n"})
	v.sync(t, true)
	before := v.f.embedded()
	var logs []string
	var mu sync.Mutex
	stop := StartBackground(context.Background(), v.env, func(f string, a ...any) {
		mu.Lock()
		logs = append(logs, fmt.Sprintf(f, a...))
		mu.Unlock()
	})
	defer stop()
	v.write(t, "n.md", "cherry\n")
	if _, err := v.env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(4 * time.Second)
	for v.f.embedded() == before && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if v.f.embedded() != before+1 {
		t.Fatalf("not embedded in time: %d -> %d", before, v.f.embedded())
	}
}

func TestBackgroundNoopWhenUnconfigured(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n"})
	v.env.Cfg.Semantic = config.Semantic{}
	stop := StartBackground(context.Background(), v.env, t.Logf)
	defer stop()
	v.write(t, "n.md", "cherry\n")
	v.reindex(t)
	time.Sleep(100 * time.Millisecond)
	if v.f.embedded() != 0 {
		t.Fatal("embedded while unconfigured")
	}
}

func TestBackgroundLogsModelChangeRefusal(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n"})
	v.sync(t, true)
	v.f.model = "fake-2"
	old := Debounce
	Debounce = 20 * time.Millisecond
	defer func() { Debounce = old }()
	logged := make(chan string, 4)
	stop := StartBackground(context.Background(), v.env, func(f string, a ...any) { logged <- fmt.Sprintf(f, a...) })
	defer stop()
	v.write(t, "n.md", "cherry\n")
	v.reindex(t)
	select {
	case m := <-logged:
		if !strings.Contains(m, "model changed") {
			t.Fatal(m)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no log")
	}
}

func TestFirstSyncRequiresConfirm(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n", "b.md": "banana\n"})
	_, err := Sync(context.Background(), v.env, false)
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeInvalid || !strings.Contains(oe.Msg, "first embed: 2 chunks") {
		t.Fatalf("want first-embed refusal, got %v", err)
	}
	if v.f.embedded() != 0 {
		t.Fatal("embedded before confirm")
	}
	if st := v.sync(t, true); st.Embedded != 2 {
		t.Fatalf("confirmed sync embedded %d", st.Embedded)
	}
	v.write(t, "c.md", "cherry\n")
	v.reindex(t)
	if st := v.sync(t, false); st.Embedded != 1 {
		t.Fatalf("incremental sync after first embed: %d", st.Embedded)
	}
}

func TestBackgroundDoesNotRunFirstEmbed(t *testing.T) {
	v := newVault(t, map[string]string{"a.md": "apple\n"})
	old := Debounce
	Debounce = 20 * time.Millisecond
	defer func() { Debounce = old }()
	logged := make(chan string, 4)
	stop := StartBackground(context.Background(), v.env, func(f string, a ...any) { logged <- fmt.Sprintf(f, a...) })
	defer stop()
	v.write(t, "n.md", "cherry\n")
	v.reindex(t)
	select {
	case msg := <-logged:
		if !strings.Contains(msg, "first embed") {
			t.Fatalf("unexpected log %q", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no refusal logged")
	}
	if v.f.embedded() != 0 {
		t.Fatal("background ran the first embed")
	}
}
