package notes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

type harness struct {
	t   *testing.T
	dir string
	cfg string
	env *ops.Env
	c   client.Client
	now time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, dir: t.TempDir(), now: time.Date(2026, 3, 5, 14, 7, 9, 0, time.Local)}
	h.cfg = t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", h.cfg)
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h.env = &ops.Env{Cfg: &config.Config{NotesDir: h.dir, Editor: "true"}, Store: st, Index: index.New(st, h.dir),
		Now: func() time.Time { return h.now }}
	h.c = client.Local(h.env)
	return h
}

func (h *harness) write(rel, content string) {
	h.t.Helper()
	p := filepath.Join(h.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) template(name, content string) {
	h.t.Helper()
	p := filepath.Join(h.cfg, "gote", "templates", name+".md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) read(rel string) string {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.dir, filepath.FromSlash(rel)))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

func (h *harness) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(h.dir, filepath.FromSlash(rel)))
	return err == nil
}

func (h *harness) reconcile() {
	h.t.Helper()
	if _, err := h.env.Index.Reconcile(context.Background()); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) call(op string, req, resp any) error {
	return h.c.Call(context.Background(), op, req, resp)
}

func (h *harness) must(op string, req, resp any) {
	h.t.Helper()
	if err := h.call(op, req, resp); err != nil {
		h.t.Fatalf("%s: %v", op, err)
	}
}

func (h *harness) wantCode(op string, req any, code string) {
	h.t.Helper()
	err := h.call(op, req, nil)
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != code {
		h.t.Fatalf("%s: want %s, got %v", op, code, err)
	}
}

func (h *harness) count(q string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.env.Store.DB.QueryRow(q, args...).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func has(s string, subs ...string) bool {
	for _, x := range subs {
		if !strings.Contains(s, x) {
			return false
		}
	}
	return true
}
