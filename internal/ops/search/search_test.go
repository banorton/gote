package search

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
	_ "github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

type vault struct {
	dir string
	c   client.Client
	ix  *index.Indexer
}

func write(t testing.TB, dir, rel, content string, mtime time.Time) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func newVault(t testing.TB, files map[string]string, mtimes map[string]time.Time) *vault {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	for rel, content := range files {
		mt, ok := mtimes[rel]
		if !ok {
			mt = time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
		}
		write(t, dir, rel, content, mt)
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
	env := &ops.Env{Cfg: &config.Config{NotesDir: dir}, Store: st, Index: ix, Now: time.Now}
	return &vault{dir: dir, c: client.Local(env), ix: ix}
}

func (v *vault) query(t testing.TB, q string) []Result {
	t.Helper()
	var resp QueryResp
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: q}, &resp); err != nil {
		t.Fatalf("query %q: %v", q, err)
	}
	return resp.Results
}

func ids(rs []Result) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func has(rs []Result, id string) bool {
	for _, r := range rs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func errCode(err error) string {
	var oe *ops.Error
	if errors.As(err, &oe) {
		return oe.Code
	}
	return ""
}

var fixture = map[string]string{
	"title-hit.md": "---\ntitle: Kubernetes guide\ntags: [ops]\n---\nunrelated words here\n",
	"body-hit.md":  "---\ntags: [ops]\n---\nfirst line\nthis note mentions kubernetes once\n",
	"untagged.md":  "plain text about kubernetes cluster\n",
	"go.md":        "---\ntags: [go, lang]\nstatus: draft\nlabels: [Alpha, beta]\n---\nthe go language\n",
	"rust.md":      "---\ntags: [lang]\nstatus: done\n---\nthe rust language\n",
	"sub/deep.md":  "---\naliases: [deepnote]\ncreated: \"260314153045\"\n---\nnested content with unique zebra\n",
	"old.md":       "---\ncreated: \"250101000000\"\n---\nancient content\n",
	"march.md":     "---\ncreated: \"260301000000\"\n---\nmarch content\n",
	"april.md":     "---\ncreated: \"260401000000\"\n---\napril content\n",
}

func TestTitleOutranksBody(t *testing.T) {
	v := newVault(t, fixture, nil)
	rs := v.query(t, "kubernetes")
	if len(rs) != 3 {
		t.Fatalf("%v", ids(rs))
	}
	if rs[0].ID != "title-hit.md" {
		t.Fatalf("title match should rank first: %v", ids(rs))
	}
	if rs[0].Score <= rs[1].Score {
		t.Fatalf("scores not descending: %+v", rs)
	}
}

func TestResultShape(t *testing.T) {
	v := newVault(t, fixture, nil)
	rs := v.query(t, "zebra")
	if len(rs) != 1 {
		t.Fatal(ids(rs))
	}
	r := rs[0]
	if r.ID != "sub/deep.md" || r.Kind != "note" || r.Title != "deep" {
		t.Fatalf("%+v", r)
	}
	if r.Path != filepath.Join(v.dir, "sub", "deep.md") {
		t.Fatalf("path %q", r.Path)
	}
	if !strings.Contains(r.Snippet, "zebra") {
		t.Fatalf("snippet %q", r.Snippet)
	}
	if r.Line != 5 {
		t.Fatalf("line %d, want 5 (file-relative, frontmatter counted)", r.Line)
	}
}

func TestTagFilter(t *testing.T) {
	v := newVault(t, fixture, nil)
	rs := v.query(t, "tag:ops")
	if len(rs) != 2 || has(rs, "untagged.md") {
		t.Fatal(ids(rs))
	}
	rs = v.query(t, "kubernetes tag:ops")
	if len(rs) != 2 || has(rs, "untagged.md") {
		t.Fatal(ids(rs))
	}
	rs = v.query(t, "kubernetes -tag:ops")
	if len(rs) != 1 || rs[0].ID != "untagged.md" {
		t.Fatal(ids(rs))
	}
	rs = v.query(t, "tag:GO")
	if len(rs) != 1 || rs[0].ID != "go.md" {
		t.Fatalf("tag match is case-insensitive: %v", ids(rs))
	}
}

func TestFieldFilter(t *testing.T) {
	v := newVault(t, fixture, nil)
	if rs := v.query(t, "status:draft"); len(rs) != 1 || rs[0].ID != "go.md" {
		t.Fatal(ids(rs))
	}
	if rs := v.query(t, "status:DONE"); len(rs) != 1 || rs[0].ID != "rust.md" {
		t.Fatal(ids(rs))
	}
	if rs := v.query(t, "labels:alpha"); len(rs) != 1 || rs[0].ID != "go.md" {
		t.Fatalf("list field: %v", ids(rs))
	}
	if rs := v.query(t, "labels:gamma"); len(rs) != 0 {
		t.Fatal(ids(rs))
	}
	if rs := v.query(t, "language -status:draft"); len(rs) != 1 || rs[0].ID != "rust.md" {
		t.Fatal(ids(rs))
	}
	if rs := v.query(t, "nosuchfield:x"); len(rs) != 0 {
		t.Fatal(ids(rs))
	}
}

func TestBooleanField(t *testing.T) {
	v := newVault(t, map[string]string{
		"a.md": "---\ndraft: true\n---\nx\n",
		"b.md": "---\ndraft: false\n---\nx\n",
	}, nil)
	if rs := v.query(t, "draft:true"); len(rs) != 1 || rs[0].ID != "a.md" {
		t.Fatal(ids(rs))
	}
}

func TestCreatedRanges(t *testing.T) {
	v := newVault(t, fixture, nil)
	cases := map[string][]string{
		"created:2603":       {"march.md", "sub/deep.md"},
		"created:26":         {"april.md", "march.md", "sub/deep.md"},
		"created:260314":     {"sub/deep.md"},
		"created:>2603":      {"april.md"},
		"created:<2603":      {"old.md"},
		"created:2601..2603": {"march.md", "sub/deep.md"},
		"-created:26":        nil,
	}
	for q, want := range cases {
		got := ids(v.query(t, q))
		if q == "-created:26" {
			if has(v.query(t, q), "march.md") || has(v.query(t, q), "april.md") {
				t.Errorf("%s: %v", q, got)
			}
			continue
		}
		if !sameSet(got, want) {
			t.Errorf("%s: got %v want %v", q, got, want)
		}
	}
}

func TestModifiedRanges(t *testing.T) {
	mt := map[string]time.Time{
		"a.md": time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
		"b.md": time.Date(2026, 3, 31, 23, 59, 59, 0, time.Local),
		"c.md": time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local),
		"d.md": time.Date(2026, 2, 28, 23, 59, 59, 0, time.Local),
	}
	v := newVault(t, map[string]string{"a.md": "x\n", "b.md": "x\n", "c.md": "x\n", "d.md": "x\n"}, mt)
	cases := map[string][]string{
		"modified:2603":       {"a.md", "b.md"},
		"modified:>2603":      {"c.md"},
		"modified:<2603":      {"d.md"},
		"modified:2602..2603": {"a.md", "b.md", "d.md"},
		"modified:26":         {"a.md", "b.md", "c.md", "d.md"},
	}
	for q, want := range cases {
		if got := ids(v.query(t, q)); !sameSet(got, want) {
			t.Errorf("%s: got %v want %v", q, got, want)
		}
	}
}

func TestFilterOnlyOrdersByMtimeDesc(t *testing.T) {
	mt := map[string]time.Time{
		"a.md": time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
		"b.md": time.Date(2026, 3, 3, 0, 0, 0, 0, time.Local),
		"c.md": time.Date(2026, 3, 2, 0, 0, 0, 0, time.Local),
	}
	v := newVault(t, map[string]string{"a.md": "x\n", "b.md": "x\n", "c.md": "x\n"}, mt)
	got := ids(v.query(t, "modified:2603"))
	if strings.Join(got, ",") != "b.md,c.md,a.md" {
		t.Fatal(got)
	}
}

func TestNegatedTermOnly(t *testing.T) {
	v := newVault(t, fixture, nil)
	rs := v.query(t, "-kubernetes")
	if has(rs, "title-hit.md") || has(rs, "body-hit.md") || !has(rs, "go.md") {
		t.Fatal(ids(rs))
	}
	rs = v.query(t, "language -rust")
	if len(rs) != 1 || rs[0].ID != "go.md" {
		t.Fatal(ids(rs))
	}
}

func TestPhrase(t *testing.T) {
	v := newVault(t, map[string]string{
		"a.md": "the quick brown fox\n",
		"b.md": "brown quick fox\n",
	}, nil)
	rs := v.query(t, `"quick brown"`)
	if len(rs) != 1 || rs[0].ID != "a.md" {
		t.Fatal(ids(rs))
	}
}

func TestAliasAndStemming(t *testing.T) {
	v := newVault(t, fixture, nil)
	if rs := v.query(t, "deepnote"); len(rs) != 1 || rs[0].ID != "sub/deep.md" {
		t.Fatalf("alias: %v", ids(rs))
	}
	if rs := v.query(t, "languages"); len(rs) != 2 {
		t.Fatalf("porter stemming: %v", ids(rs))
	}
}

func TestFTSInjection(t *testing.T) {
	v := newVault(t, fixture, nil)
	for _, q := range []string{
		`"`, `""`, `"unbalanced`, `*`, `kube*`, `NEAR`, `NEAR(a b)`, `a OR b`, `a AND b`, `a NOT b`,
		`foo:bar`, `title:kubernetes`, `(`, `)`, `(kubernetes`, `^kubernetes`, `-`, `--`, `-*`,
		`kubernetes"`, `a"b`, `{x}`, `col:val OR`, `'; DROP TABLE notes; --`, `\`, `%`, `NOT`, `+ -`, `a:b:c`,
	} {
		var resp QueryResp
		err := v.c.Call(context.Background(), "search.query", QueryReq{Q: q}, &resp)
		if err != nil && errCode(err) != ops.CodeInvalid {
			t.Errorf("%q: non-invalid error: %v", q, err)
		}
	}
	if rs := v.query(t, "a OR kubernetes"); len(rs) != 0 {
		t.Fatalf("OR must be a literal term, not an operator: %v", ids(rs))
	}
}

func TestEmptyQueryInvalid(t *testing.T) {
	v := newVault(t, fixture, nil)
	for _, q := range []string{"", "   "} {
		err := v.c.Call(context.Background(), "search.query", QueryReq{Q: q}, nil)
		if errCode(err) != ops.CodeInvalid {
			t.Errorf("%q: %v", q, err)
		}
	}
	err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "created:260"}, nil)
	if errCode(err) != ops.CodeInvalid || !strings.Contains(err.Error(), "created:260") {
		t.Errorf("%v", err)
	}
}

func TestLimit(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < 60; i++ {
		files["n"+string(rune('a'+i/26))+string(rune('a'+i%26))+".md"] = "common\n"
	}
	v := newVault(t, files, nil)
	if rs := v.query(t, "common"); len(rs) != 50 {
		t.Fatalf("default limit: %d", len(rs))
	}
	var resp QueryResp
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "common", Limit: 5}, &resp); err != nil || len(resp.Results) != 5 {
		t.Fatalf("%v %d", err, len(resp.Results))
	}
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "common", Limit: -1}, nil); errCode(err) != ops.CodeInvalid {
		t.Fatal(err)
	}
}

func TestSemanticNotConfigured(t *testing.T) {
	v := newVault(t, fixture, nil)
	err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "x", Semantic: true}, nil)
	if errCode(err) != ops.CodeInvalid || !strings.Contains(err.Error(), "semantic search not configured") {
		t.Fatal(err)
	}
}

func TestSemanticHookCalled(t *testing.T) {
	v := newVault(t, fixture, nil)
	SemanticHook = func(ctx context.Context, env *ops.Env, q string, limit int, kind string) ([]Result, error) {
		return []Result{{ID: "x.md", Title: q}}, nil
	}
	defer func() { SemanticHook = nil }()
	var resp QueryResp
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "hello", Semantic: true, Limit: 3}, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Title != "hello" {
		t.Fatalf("%+v", resp)
	}
}

func TestCorpus(t *testing.T) {
	v := newVault(t, fixture, nil)
	var resp CorpusResp
	if err := v.c.Call(context.Background(), "search.corpus", nil, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Items) != len(fixture) {
		t.Fatalf("%d items", len(resp.Items))
	}
	for _, it := range resp.Items {
		if it.ID == "sub/deep.md" {
			if it.Path != "sub/deep.md" || it.Title != "deep" || len(it.Aliases) != 1 || it.Aliases[0] != "deepnote" {
				t.Fatalf("%+v", it)
			}
			return
		}
	}
	t.Fatal("deep.md missing")
}

func TestLines(t *testing.T) {
	v := newVault(t, map[string]string{
		"a.md": "---\ntags: [x]\n---\nfirst\n\n  \nsecond\n",
	}, nil)
	var resp LinesResp
	if err := v.c.Call(context.Background(), "search.lines", nil, &resp); err != nil {
		t.Fatal(err)
	}
	want := []Line{{ID: "a.md", Line: 4, Text: "first"}, {ID: "a.md", Line: 7, Text: "second"}}
	if len(resp.Lines) != 2 || resp.Lines[0] != want[0] || resp.Lines[1] != want[1] {
		t.Fatalf("%+v", resp.Lines)
	}
}

func TestPreview(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 200; i++ {
		b.WriteString("line")
		b.WriteString(strings.Repeat("x", i%3))
		b.WriteString("\n")
	}
	v := newVault(t, map[string]string{"big.md": b.String(), "small.md": "a\nb\nc\n"}, nil)
	var resp PreviewResp
	if err := v.c.Call(context.Background(), "search.preview", PreviewReq{ID: "big.md", Line: 100}, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Lines) != 60 || resp.Start > 100 || resp.Start+len(resp.Lines) <= 100 {
		t.Fatalf("start=%d n=%d", resp.Start, len(resp.Lines))
	}
	if err := v.c.Call(context.Background(), "search.preview", PreviewReq{ID: "big.md", Line: 0}, &resp); err != nil || resp.Start != 1 || len(resp.Lines) != 60 {
		t.Fatalf("%v start=%d n=%d", err, resp.Start, len(resp.Lines))
	}
	if err := v.c.Call(context.Background(), "search.preview", PreviewReq{ID: "big.md", Line: 199}, &resp); err != nil || resp.Start+len(resp.Lines)-1 != 200 || len(resp.Lines) != 60 {
		t.Fatalf("%v start=%d n=%d", err, resp.Start, len(resp.Lines))
	}
	if err := v.c.Call(context.Background(), "search.preview", PreviewReq{ID: "small.md", Line: 2}, &resp); err != nil || len(resp.Lines) != 3 {
		t.Fatalf("%v %+v", err, resp)
	}
	err := v.c.Call(context.Background(), "search.preview", PreviewReq{ID: "../../etc/passwd"}, &resp)
	if errCode(err) != ops.CodeNotFound {
		t.Fatalf("unindexed id must be not_found: %v", err)
	}
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, n := range m {
		if n != 0 {
			return false
		}
	}
	return true
}

func (v *vault) fuzzy(t testing.TB, q string, limit int) ([]FuzzyResult, error) {
	t.Helper()
	var resp FuzzyResp
	err := v.c.Call(context.Background(), "search.fuzzy", FuzzyReq{Q: q, Limit: limit}, &resp)
	return resp.Results, err
}

var fuzzyFiles = map[string]string{
	"k/u/b/e.md":   "---\ntitle: Misc\n---\nnothing\n",
	"k8s.md":       "---\ntitle: Kubernetes notes\naliases: [cluster]\n---\npods\n",
	"other.md":     "unrelated\n",
	"body-only.md": "mentions kubernetes deeply in the body\n",
}

func TestFuzzyRanksTitleAboveWeakPath(t *testing.T) {
	v := newVault(t, fuzzyFiles, nil)
	rs, err := v.fuzzy(t, "kube", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 || rs[0].ID != "k8s.md" || rs[1].ID != "k/u/b/e.md" {
		t.Fatalf("%+v", rs)
	}
	if rs[0].Title != "Kubernetes notes" || rs[0].Kind != "note" || rs[0].Score <= rs[1].Score {
		t.Fatalf("%+v", rs)
	}
	if rs[0].Path != filepath.Join(v.dir, "k8s.md") {
		t.Fatalf("path %q", rs[0].Path)
	}
}

func TestFuzzyMatchesAliasAndSkipsBody(t *testing.T) {
	v := newVault(t, fuzzyFiles, nil)
	rs, _ := v.fuzzy(t, "cluster", 0)
	if len(rs) != 1 || rs[0].ID != "k8s.md" {
		t.Fatalf("%+v", rs)
	}
	if rs, _ = v.fuzzy(t, "deeply", 0); len(rs) != 0 {
		t.Fatalf("body must not match: %+v", rs)
	}
}

func TestFuzzyLimitAndEmptyShape(t *testing.T) {
	v := newVault(t, fuzzyFiles, nil)
	if rs, _ := v.fuzzy(t, "kube", 1); len(rs) != 1 {
		t.Fatalf("%+v", rs)
	}
	rs, err := v.fuzzy(t, "zzzzqq", 0)
	if err != nil || rs == nil || len(rs) != 0 {
		t.Fatalf("%v %#v", err, rs)
	}
}

func TestFuzzyRejectsFilterTokensWithHint(t *testing.T) {
	v := newVault(t, fuzzyFiles, nil)
	for _, q := range []string{"kube tag:ops", "kube -tag:ops", "kube -other", "created:2611"} {
		_, err := v.fuzzy(t, q, 0)
		var oe *ops.Error
		if !errors.As(err, &oe) || oe.Code != ops.CodeInvalid || !strings.Contains(err.Error(), "-t") {
			t.Fatalf("%q: %v", q, err)
		}
	}
}

func TestFuzzyEmptyAndNegativeLimitInvalid(t *testing.T) {
	v := newVault(t, fuzzyFiles, nil)
	if _, err := v.fuzzy(t, "  ", 0); err == nil {
		t.Fatal("empty query")
	}
	if _, err := v.fuzzy(t, "kube", -1); err == nil {
		t.Fatal("negative limit")
	}
}

func TestSearchStringIncludesPathAndAliases(t *testing.T) {
	got := SearchString(CorpusItem{ID: "sub/a.md", Title: "A", Path: "sub/a.md", Aliases: []string{"x", "y"}})
	if got != "A  sub/a.md  x y" {
		t.Fatalf("%q", got)
	}
	if got = SearchString(CorpusItem{ID: "a.md", Title: "A", Path: "a.md", Aliases: []string{}}); got != "A  a.md" {
		t.Fatalf("%q", got)
	}
}

var kindFiles = map[string]string{
	"a.md":              "kubernetes note\n",
	"b.md":              "other note\n",
	"d1.excalidraw":     `{"type":"excalidraw","elements":[{"id":"t1","type":"text","text":"kubernetes sketch"}]}`,
	"sub/d2.excalidraw": `{"type":"excalidraw","elements":[{"id":"t1","type":"text","text":"other sketch"}]}`,
}

func kindVault(t *testing.T) *vault {
	return newVault(t, kindFiles, map[string]time.Time{
		"d1.excalidraw":     time.Date(2026, 5, 1, 0, 0, 0, 0, time.Local),
		"sub/d2.excalidraw": time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local),
		"a.md":              time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local),
		"b.md":              time.Date(2026, 4, 1, 0, 0, 0, 0, time.Local),
	})
}

func TestKindFilter(t *testing.T) {
	v := kindVault(t)
	if got := ids(v.query(t, "kind:diagram")); strings.Join(got, ",") != "sub/d2.excalidraw,d1.excalidraw" {
		t.Fatalf("filter-only diagram: %v", got)
	}
	if got := ids(v.query(t, "kind:note")); strings.Join(got, ",") != "b.md,a.md" {
		t.Fatalf("filter-only note: %v", got)
	}
	if got := ids(v.query(t, "kind:DIAGRAM")); len(got) != 2 {
		t.Fatalf("case: %v", got)
	}
	if got := ids(v.query(t, "-kind:diagram")); strings.Join(got, ",") != "b.md,a.md" {
		t.Fatalf("negated: %v", got)
	}
	if got := ids(v.query(t, "kubernetes kind:diagram")); strings.Join(got, ",") != "d1.excalidraw" {
		t.Fatalf("with term: %v", got)
	}
	if got := ids(v.query(t, "kubernetes -kind:diagram")); strings.Join(got, ",") != "a.md" {
		t.Fatalf("term negated kind: %v", got)
	}
	if got := ids(v.query(t, "kind:diagram modified:2606")); strings.Join(got, ",") != "sub/d2.excalidraw" {
		t.Fatalf("combined: %v", got)
	}
}

func TestKindUnknownValueInvalid(t *testing.T) {
	v := kindVault(t)
	var resp QueryResp
	err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "kind:image"}, &resp)
	if errCode(err) != ops.CodeInvalid || !strings.Contains(err.Error(), "diagram") || !strings.Contains(err.Error(), "note") {
		t.Fatalf("%v", err)
	}
}

func TestFuzzyRejectsKindFilterWithHint(t *testing.T) {
	v := kindVault(t)
	for _, q := range []string{"kube kind:diagram", "kube -kind:note"} {
		_, err := v.fuzzy(t, q, 0)
		if errCode(err) != ops.CodeInvalid || !strings.Contains(err.Error(), "-t") {
			t.Fatalf("%q: %v", q, err)
		}
	}
}

var sharedFiles = map[string]string{
	"kube-notes.md":          "kubernetes in a note\n",
	"kube-sketch.excalidraw": `{"type":"excalidraw","gote":{"description":"rollout topology"},"elements":[{"id":"t1","type":"text","text":"kubernetes sketch"}]}`,
	"other.excalidraw":       `{"type":"excalidraw","gote":{"description":"zebra layout"},"elements":[]}`,
}

func TestQueryKindRestriction(t *testing.T) {
	v := newVault(t, sharedFiles, nil)
	var resp QueryResp
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "kubernetes", Kind: "diagram"}, &resp); err != nil {
		t.Fatal(err)
	}
	if got := ids(resp.Results); strings.Join(got, ",") != "kube-sketch.excalidraw" {
		t.Fatalf("%v", got)
	}
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "zebra", Kind: "diagram"}, &resp); err != nil || !has(resp.Results, "other.excalidraw") {
		t.Fatalf("description not searchable: %v %v", err, ids(resp.Results))
	}
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "kind:note kubernetes", Kind: "diagram"}, &resp); err != nil || len(resp.Results) != 0 {
		t.Fatalf("restriction must AND with user filters: %v %v", err, ids(resp.Results))
	}
	err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "x", Kind: "image"}, &resp)
	if errCode(err) != ops.CodeInvalid {
		t.Fatalf("%v", err)
	}
}

func TestQueryKindPassedToSemanticHook(t *testing.T) {
	v := newVault(t, sharedFiles, nil)
	var got string
	SemanticHook = func(ctx context.Context, env *ops.Env, q string, limit int, kind string) ([]Result, error) {
		got = kind
		return nil, nil
	}
	defer func() { SemanticHook = nil }()
	if err := v.c.Call(context.Background(), "search.query", QueryReq{Q: "x", Semantic: true, Kind: "diagram"}, nil); err != nil || got != "diagram" {
		t.Fatalf("%v %q", err, got)
	}
}

func TestFuzzyKindRestriction(t *testing.T) {
	v := newVault(t, sharedFiles, nil)
	var resp FuzzyResp
	if err := v.c.Call(context.Background(), "search.fuzzy", FuzzyReq{Q: "kube", Kind: "diagram"}, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].ID != "kube-sketch.excalidraw" {
		t.Fatalf("%+v", resp.Results)
	}
	if err := v.c.Call(context.Background(), "search.fuzzy", FuzzyReq{Q: "kube"}, &resp); err != nil || len(resp.Results) != 2 {
		t.Fatalf("%v %+v", err, resp.Results)
	}
}
