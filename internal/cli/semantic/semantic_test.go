package semantic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/embed"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/semantic"
	"github.com/banorton/gote/internal/semantic"
	"github.com/banorton/gote/internal/store"
)

type fakeEmb struct{ model string }

func (f fakeEmb) Model() string { return f.model }
func (fakeEmb) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{1, float32(i)}
	}
	return out, nil
}

func testCtx(t *testing.T, sem config.Semantic, model string) (*cli.Ctx, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.md"), []byte("two\n"), 0o644)
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ix := index.New(st, dir)
	ix.Reconcile(context.Background())
	old := semantic.NewEmbedder
	semantic.NewEmbedder = func(config.Semantic) (embed.Embedder, error) { return fakeEmb{model}, nil }
	t.Cleanup(func() { semantic.NewEmbedder = old })
	cfg := &config.Config{NotesDir: dir, Semantic: sem}
	env := &ops.Env{Cfg: cfg, Store: st, Index: ix, Now: time.Now}
	var out, errb bytes.Buffer
	return &cli.Ctx{Client: client.Local(env), Cfg: cfg, Out: &out, Err: &errb}, &out, &errb
}

var enabled = config.Semantic{Provider: "command", Model: "m1", EmbedCmd: "x", BatchSize: 8, PricePerMTok: 1}

func TestEmbedAndStatus(t *testing.T) {
	c, out, _ := testCtx(t, enabled, "m1")
	var oe *ops.Error
	if err := run(c, nil); !errors.As(err, &oe) || !strings.Contains(oe.Msg, "first embed: 2 chunks") || !strings.Contains(oe.Msg, "rerun with --yes") {
		t.Fatalf("want first-embed refusal, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("output on refusal: %q", out.String())
	}
	if err := run(c, []string{"--yes"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "embedded 2 of 2 chunks") {
		t.Fatalf("%q", out.String())
	}
	out.Reset()
	if err := run(c, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"model: m1", "chunks: 2", "embedded: 2", "pending: 0"} {
		if !strings.Contains(out.String(), w) {
			t.Fatalf("missing %q in %q", w, out.String())
		}
	}
}

func TestEmbedJSON(t *testing.T) {
	c, out, _ := testCtx(t, enabled, "m1")
	c.JSON = true
	if err := run(c, []string{"--yes"}); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil || m["embedded"] != float64(2) {
		t.Fatalf("%v %s", err, out.String())
	}
	out.Reset()
	if err := run(c, []string{"status"}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &m); err != nil || m["configured"] != true {
		t.Fatalf("%v %s", err, out.String())
	}
}

func TestEmbedYesFlag(t *testing.T) {
	c, _, _ := testCtx(t, enabled, "m1")
	var first *ops.Error
	if err := run(c, nil); !errors.As(err, &first) || !strings.Contains(first.Msg, "first embed") {
		t.Fatalf("%v", err)
	}
	if err := run(c, []string{"--yes"}); err != nil {
		t.Fatal(err)
	}
	semantic.NewEmbedder = func(config.Semantic) (embed.Embedder, error) { return fakeEmb{"m2"}, nil }
	err := run(c, nil)
	var oe *ops.Error
	if !errors.As(err, &oe) || !strings.Contains(oe.Msg, "rerun with --yes") {
		t.Fatalf("%v", err)
	}
	if err := run(c, []string{"--yes"}); err != nil {
		t.Fatal(err)
	}
}

func TestEmbedUsageErrors(t *testing.T) {
	c, _, _ := testCtx(t, enabled, "m1")
	var ue *cli.UsageError
	if err := run(c, []string{"bogus"}); !errors.As(err, &ue) {
		t.Fatalf("%v", err)
	}
	if err := run(c, []string{"--nope"}); !errors.As(err, &ue) {
		t.Fatalf("%v", err)
	}
}

func TestEmbedUnconfigured(t *testing.T) {
	c, out, _ := testCtx(t, config.Semantic{}, "m1")
	if err := run(c, nil); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("%v", err)
	}
	if err := run(c, []string{"status"}); err != nil || !strings.Contains(out.String(), "not configured") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func keyConfig(t *testing.T, mode os.FileMode) {
	t.Helper()
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", d)
	p := filepath.Join(d, "gote", "config.yaml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("x"), 0o644)
	os.Chmod(p, mode)
}

func TestEmbedWarnsAboutOpenKeyFile(t *testing.T) {
	keyConfig(t, 0o644)
	sem := config.Semantic{Provider: "openai", Model: "m1", APIKey: "sk-secret", BatchSize: 8}
	c, _, errb := testCtx(t, sem, "m1")
	if err := run(c, []string{"--yes"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "chmod 600") || strings.Contains(errb.String(), "sk-secret") {
		t.Fatalf("%q", errb.String())
	}
}

func TestSearchSemanticWarnsAboutOpenKeyFile(t *testing.T) {
	keyConfig(t, 0o644)
	sem := config.Semantic{Provider: "openai", Model: "m1", APIKey: "sk-secret", BatchSize: 8}
	c, _, errb := testCtx(t, sem, "m1")
	cmd, ok := cli.Lookup("search")
	if !ok {
		t.Fatal("search not registered")
	}
	cmd.Run(c, []string{"-s", "x"})
	if !strings.Contains(errb.String(), "chmod 600") {
		t.Fatalf("%q", errb.String())
	}
	errb.Reset()
	cmd.Run(c, []string{"x"})
	if errb.Len() != 0 {
		t.Fatalf("plain search warned: %q", errb.String())
	}
	keyConfig(t, 0o600)
	cmd.Run(c, []string{"-s", "x"})
	if errb.Len() != 0 {
		t.Fatalf("0600 config warned: %q", errb.String())
	}
}
