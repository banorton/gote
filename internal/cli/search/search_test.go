package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/cli/results"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	opsearch "github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/store"
)

func testCtx(t *testing.T, files map[string]string) (*cli.Ctx, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
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
	cfg := &config.Config{NotesDir: dir, Editor: "vi"}
	env := &ops.Env{Cfg: cfg, Store: st, Index: ix, Now: time.Now}
	var out bytes.Buffer
	return &cli.Ctx{Client: client.Local(env), Cfg: cfg, Out: &out, Err: &out}, &out
}

var files = map[string]string{
	"k8s.md":     "---\ntitle: Kubernetes guide\ntags: [ops]\n---\nintro\nall about kubernetes pods\n",
	"other.md":   "nothing relevant\n",
	"notes/x.md": "---\ntags: [ops]\n---\nkubernetes again\n",
}

func TestSearchPrintsRankedList(t *testing.T) {
	c, out := testCtx(t, files)
	if err := run(c, []string{"-t", "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	want := "Kubernetes guide  k8s.md:6\n    intro all about kubernetes pods\nx  notes/x.md:4\n    kubernetes again\n"
	if out.String() != want {
		t.Fatalf("got %q want %q", out.String(), want)
	}
}

func setTTY(t *testing.T, v bool) {
	t.Helper()
	old := results.IsTTY
	results.IsTTY = func(io.Writer) bool { return v }
	t.Cleanup(func() { results.IsTTY = old })
}

func TestSearchListFlagPrintsStyledList(t *testing.T) {
	for _, flag := range []string{"--list", "-l"} {
		c, out := testCtx(t, files)
		setTTY(t, true)
		if err := run(c, []string{flag, "-t", "kubernetes"}); err != nil {
			t.Fatal(err)
		}
		s := out.String()
		if !strings.HasPrefix(s, "\x1b[1mKubernetes guide\x1b[0m  \x1b[2mk8s.md:6\x1b[0m\n    intro all about \x1b[1;33mkubernetes\x1b[0m pods\n") {
			t.Fatalf("%s: %q", flag, s)
		}
	}
}

func TestSearchNoANSIWhenNotTTY(t *testing.T) {
	c, out := testCtx(t, files)
	setTTY(t, false)
	if err := run(c, []string{"-t", "kubernetes"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatalf("%q", out.String())
	}
}

func TestSearchSemanticListFormat(t *testing.T) {
	c, out := testCtx(t, files)
	opsearch.SemanticHook = func(ctx context.Context, env *ops.Env, q string, limit int, kind string, all bool) ([]opsearch.Result, error) {
		return []opsearch.Result{{ID: "k8s.md", Title: "Kubernetes guide", Kind: "note", Line: 5, Score: 0.8249, Snippet: "\n  first chunk line\nsecond"}}, nil
	}
	t.Cleanup(func() { opsearch.SemanticHook = nil })
	if err := run(c, []string{"-s", "orchestration"}); err != nil {
		t.Fatal(err)
	}
	if want := "Kubernetes guide  k8s.md:5  0.82\n    first chunk line\n"; out.String() != want {
		t.Fatalf("%q", out.String())
	}
}

func TestSearchAllIncludesArchived(t *testing.T) {
	fs := map[string]string{
		"live.md": "---\ntags: [x]\n---\nzebra\n",
		"old.md":  "---\ntags: [Archive]\n---\nzebra\n",
	}
	ids := func(args ...string) string {
		c, out := testCtx(t, fs)
		c.JSON = true
		if err := run(c, args); err != nil {
			t.Fatal(err)
		}
		var resp struct{ Results []struct{ ID string } }
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, r := range resp.Results {
			got = append(got, r.ID)
		}
		return strings.Join(got, ",")
	}
	if got := ids("-t", "zebra"); got != "live.md" {
		t.Errorf("-t: %q", got)
	}
	if got := ids("-t", "--all", "zebra"); got != "live.md,old.md" {
		t.Errorf("-t --all: %q", got)
	}
	if got := ids("-t", "zebra", "tag:archive"); got != "old.md" {
		t.Errorf("-t tag:archive: %q", got)
	}
	if got := ids("old"); got != "" {
		t.Errorf("fuzzy: %q", got)
	}
	if got := ids("--all", "old"); got != "old.md" {
		t.Errorf("fuzzy --all: %q", got)
	}
}

func TestSearchJSON(t *testing.T) {
	c, out := testCtx(t, files)
	c.JSON = true
	if err := run(c, []string{"--text", "kubernetes", "tag:ops"}); err != nil {
		t.Fatal(err)
	}
	var resp opsearch.QueryResp
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil || len(resp.Results) != 2 {
		t.Fatalf("%v %s", err, out)
	}
}

func TestSearchLimitFlag(t *testing.T) {
	for _, args := range [][]string{{"-t", "-n", "1", "kubernetes"}, {"-t", "kubernetes", "-n=1"}, {"-t", "--n", "1", "kubernetes"}} {
		c, out := testCtx(t, files)
		c.JSON = true
		if err := run(c, args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var resp opsearch.QueryResp
		json.Unmarshal(out.Bytes(), &resp)
		if len(resp.Results) != 1 {
			t.Fatalf("%v: %d results", args, len(resp.Results))
		}
	}
}

func TestSearchNegationIsNotAFlag(t *testing.T) {
	c, out := testCtx(t, files)
	c.JSON = true
	if err := run(c, []string{"-t", "kubernetes", "-tag:ops"}); err != nil {
		t.Fatal(err)
	}
	var resp opsearch.QueryResp
	json.Unmarshal(out.Bytes(), &resp)
	if len(resp.Results) != 0 {
		t.Fatalf("%s", out)
	}
}

func TestSearchDoubleDashEndsFlags(t *testing.T) {
	c, _ := testCtx(t, files)
	if err := run(c, []string{"-t", "--", "-s"}); err != nil {
		t.Fatalf("-s after -- is a term: %v", err)
	}
}

func TestSearchSemanticNotConfigured(t *testing.T) {
	for _, flag := range []string{"-s", "--semantic"} {
		c, _ := testCtx(t, files)
		err := run(c, []string{flag, "kubernetes"})
		if err == nil || !strings.Contains(err.Error(), "semantic search not configured") {
			t.Fatalf("%s: %v", flag, err)
		}
	}
}

func TestSearchBadLimitIsUsage(t *testing.T) {
	c, _ := testCtx(t, files)
	var ue *cli.UsageError
	for _, args := range [][]string{{"-n", "x", "a"}, {"-n"}, {"-n", "-1", "a"}, {"-t", "-s", "a"}, {"--text", "--semantic", "a"}} {
		if err := run(c, args); !errors.As(err, &ue) {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func TestSearchNoArgsNotTTYIsUsage(t *testing.T) {
	c, _ := testCtx(t, files)
	var ue *cli.UsageError
	if err := run(c, nil); !errors.As(err, &ue) {
		t.Fatalf("%v", err)
	}
}

func TestSearchInvalidQuerySurfaces(t *testing.T) {
	c, _ := testCtx(t, files)
	err := run(c, []string{"-t", "created:260"})
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeInvalid {
		t.Fatalf("%v", err)
	}
}

func TestInvokeUnsetIsNotAvailable(t *testing.T) {
	c, _ := testCtx(t, files)
	old := Invoke
	Invoke = nil
	defer func() { Invoke = old }()
	err := invoke(c, "view", []string{"a.md"})
	if err == nil || err.Error() != "view: not available" {
		t.Fatalf("%v", err)
	}
}

func TestInvokeQuietDiscardsOutput(t *testing.T) {
	c, out := testCtx(t, files)
	old := Invoke
	Invoke = func(cc *cli.Ctx, name string, args []string) error {
		cc.Out.Write([]byte("noisy"))
		return nil
	}
	defer func() { Invoke = old }()
	if err := invokeQuiet(c, "pin", []string{"a.md"}); err != nil || out.Len() != 0 {
		t.Fatalf("%v %q", err, out)
	}
}

func TestPreviewText(t *testing.T) {
	got := previewText(opsearch.PreviewResp{Start: 8, Lines: []string{"a", "b", "c"}}, 9)
	want := "   8  a\n>  9  b\n  10  c"
	if got != want {
		t.Fatalf("%q", got)
	}
}

func TestSearchDefaultIsFuzzyTitleMatch(t *testing.T) {
	c, out := testCtx(t, files)
	if err := run(c, []string{"kube"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "Kubernetes guide  (k8s.md)\n" {
		t.Fatalf("%q", got)
	}
}

func TestSearchFuzzyDoesNotSearchBody(t *testing.T) {
	c, out := testCtx(t, files)
	if err := run(c, []string{"pods"}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("%q", out)
	}
}

func TestSearchFuzzyJSONShape(t *testing.T) {
	c, out := testCtx(t, files)
	c.JSON = true
	if err := run(c, []string{"kube"}); err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil || len(raw.Results) != 1 {
		t.Fatalf("%v %s", err, out)
	}
	r := raw.Results[0]
	for _, k := range []string{"id", "title", "kind", "path", "score"} {
		if _, ok := r[k]; !ok {
			t.Fatalf("missing %q in %v", k, r)
		}
	}
	if len(r) != 5 || r["id"] != "k8s.md" {
		t.Fatalf("%v", r)
	}
}

func TestSearchFuzzyLimit(t *testing.T) {
	c, out := testCtx(t, map[string]string{"ka.md": "x", "kb.md": "x", "kc.md": "x"})
	c.JSON = true
	if err := run(c, []string{"-n", "2", "k"}); err != nil {
		t.Fatal(err)
	}
	var resp struct{ Results []any }
	json.Unmarshal(out.Bytes(), &resp)
	if len(resp.Results) != 2 {
		t.Fatalf("%s", out)
	}
}

func TestSearchFuzzyAppliesFilters(t *testing.T) {
	c, out := testCtx(t, files)
	c.JSON = true
	if err := run(c, []string{"kube", "tag:ops"}); err != nil {
		t.Fatal(err)
	}
	var resp struct{ Results []struct{ ID string } }
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil || len(resp.Results) != 1 || resp.Results[0].ID != "k8s.md" {
		t.Fatalf("%v %s", err, out)
	}
}

func TestSearchTextAndSemanticTogetherExit2(t *testing.T) {
	c, _ := testCtx(t, files)
	var ue *cli.UsageError
	if err := run(c, []string{"-t", "-s", "kube"}); !errors.As(err, &ue) {
		t.Fatalf("%v", err)
	}
}

func TestFilterOnly(t *testing.T) {
	c, _ := testCtx(t, files)
	for q, want := range map[string]bool{
		"tag:ops": true, "tag:ops -tag:x": true, "kube": false, "kube tag:ops": false, "": false,
		"created:260314": false, `created:2026-03`: true,
	} {
		if got := FilterOnly(c, q); got != want {
			t.Errorf("%q: got %v", q, got)
		}
	}
}

func TestFilterOnlyNotTTYPrintsListNewestFirst(t *testing.T) {
	c, out := testCtx(t, files)
	if err := run(c, []string{"tag:ops"}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "Kubernetes guide  (k8s.md)") || !strings.Contains(got, "x  (notes/x.md)") || strings.Contains(got, "other") {
		t.Fatalf("%q", got)
	}
}
