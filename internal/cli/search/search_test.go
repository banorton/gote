package search

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
	if err := run(c, []string{"kubernetes"}); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.HasPrefix(s, "Kubernetes guide  (k8s.md:6)\n") {
		t.Fatalf("%q", s)
	}
	if !strings.Contains(s, "x  (notes/x.md:4)\n") || !strings.Contains(s, "    ") {
		t.Fatalf("%q", s)
	}
	if strings.Contains(s, "other.md") {
		t.Fatal(s)
	}
}

func TestSearchJSON(t *testing.T) {
	c, out := testCtx(t, files)
	c.JSON = true
	if err := run(c, []string{"kubernetes", "tag:ops"}); err != nil {
		t.Fatal(err)
	}
	var resp opsearch.QueryResp
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil || len(resp.Results) != 2 {
		t.Fatalf("%v %s", err, out)
	}
}

func TestSearchLimitFlag(t *testing.T) {
	for _, args := range [][]string{{"-n", "1", "kubernetes"}, {"kubernetes", "-n=1"}, {"--n", "1", "kubernetes"}} {
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
	if err := run(c, []string{"kubernetes", "-tag:ops"}); err != nil {
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
	if err := run(c, []string{"--", "-s"}); err != nil {
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
	for _, args := range [][]string{{"-n", "x", "a"}, {"-n"}, {"-n", "-1", "a"}} {
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
	err := run(c, []string{"created:260"})
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

func TestSearchStringIncludesPathAndAliases(t *testing.T) {
	got := noteSearchString(opsearch.CorpusItem{ID: "sub/a.md", Title: "A", Path: "sub/a.md", Aliases: []string{"x", "y"}})
	if got != "A  sub/a.md  x y" {
		t.Fatalf("%q", got)
	}
	got = noteSearchString(opsearch.CorpusItem{ID: "a.md", Title: "A", Path: "a.md", Aliases: []string{}})
	if got != "A  a.md" {
		t.Fatalf("%q", got)
	}
}
