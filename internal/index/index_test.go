package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/banorton/gote/internal/store"
)

func write(t testing.TB, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t testing.TB) (string, *store.Store, *Indexer) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return dir, st, New(st, dir)
}

func count(t testing.TB, st *store.Store, q string, args ...any) int {
	t.Helper()
	var n int
	if err := st.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func dst(t testing.TB, st *store.Store, src, raw string) string {
	t.Helper()
	var d *string
	if err := st.DB.QueryRow("SELECT dst_id FROM links WHERE src=? AND dst_raw=?", src, raw).Scan(&d); err != nil {
		t.Fatalf("%s %s: %v", src, raw, err)
	}
	if d == nil {
		return "<nil>"
	}
	return *d
}

func reconcile(t testing.TB, ix *Indexer) Stats {
	t.Helper()
	s, err := ix.Reconcile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFirstReconcileAddsAll(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "a.md", "---\ntags: [x, y]\naliases: [Alpha]\n---\n# H\n[[b]]\n- [ ] t\n")
	write(t, dir, "sub/b.md", "body")
	write(t, dir, "readme.txt", "ignored")
	s := reconcile(t, ix)
	if s.Scanned != 2 || s.Added != 2 || s.Updated != 0 || s.Removed != 0 {
		t.Fatalf("%+v", s)
	}
	if count(t, st, "SELECT count(*) FROM tags WHERE note_id='a.md'") != 2 ||
		count(t, st, "SELECT count(*) FROM aliases WHERE note_id='a.md' AND alias_lc='alpha'") != 1 ||
		count(t, st, "SELECT count(*) FROM headings") != 1 ||
		count(t, st, "SELECT count(*) FROM tasks") != 1 ||
		count(t, st, "SELECT count(*) FROM notes_fts WHERE notes_fts MATCH 'body'") != 1 {
		t.Fatal("child rows wrong")
	}
	if dst(t, st, "a.md", "b") != "sub/b.md" {
		t.Fatal("link unresolved")
	}
	s = reconcile(t, ix)
	if s != (Stats{Scanned: 2, Elapsed: s.Elapsed}) {
		t.Fatalf("noop %+v", s)
	}
}

func TestTouchNoContentChange(t *testing.T) {
	dir, _, ix := setup(t)
	write(t, dir, "a.md", "hello")
	reconcile(t, ix)
	future := time.Now().Add(time.Hour)
	os.Chtimes(filepath.Join(dir, "a.md"), future, future)
	s := reconcile(t, ix)
	if s.Touched != 1 || s.Added != 0 || s.Updated != 0 {
		t.Fatalf("%+v", s)
	}
	if s = reconcile(t, ix); s.Touched != 0 {
		t.Fatalf("mtime not stored: %+v", s)
	}
}

func TestEditUpdatesOne(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "a.md", "one")
	write(t, dir, "b.md", "two")
	reconcile(t, ix)
	write(t, dir, "a.md", "changed words")
	s := reconcile(t, ix)
	if s.Updated != 1 || s.Added != 0 || s.Touched != 0 {
		t.Fatalf("%+v", s)
	}
	if count(t, st, "SELECT count(*) FROM notes_fts WHERE notes_fts MATCH 'changed'") != 1 ||
		count(t, st, "SELECT count(*) FROM notes_fts WHERE note_id='a.md'") != 1 {
		t.Fatal("fts not replaced")
	}
}

func TestDeleteRemovesRows(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "a.md", "---\ntags: [x]\n---\n# H\n[[b]]")
	write(t, dir, "b.md", "x")
	reconcile(t, ix)
	os.Remove(filepath.Join(dir, "a.md"))
	s := reconcile(t, ix)
	if s.Removed != 1 {
		t.Fatalf("%+v", s)
	}
	for _, tbl := range []string{"tags", "headings", "links", "aliases", "tasks"} {
		if count(t, st, "SELECT count(*) FROM "+tbl) != 0 {
			t.Fatalf("%s rows remain", tbl)
		}
	}
	if count(t, st, "SELECT count(*) FROM notes_fts WHERE note_id='a.md'") != 0 {
		t.Fatal("fts remains")
	}
}

func TestAmbiguousResolvesShortestPath(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "z/deep/x.md", "")
	write(t, dir, "a/x.md", "")
	write(t, dir, "b/x.md", "")
	write(t, dir, "src.md", "[[x]] [[Z/Deep/X]] [[a/x]] [[x.md]]")
	reconcile(t, ix)
	if got := dst(t, st, "src.md", "x"); got != "a/x.md" {
		t.Fatal(got)
	}
	if got := dst(t, st, "src.md", "Z/Deep/X"); got != "z/deep/x.md" {
		t.Fatal(got)
	}
	if got := dst(t, st, "src.md", "a/x"); got != "a/x.md" {
		t.Fatal(got)
	}
	if got := dst(t, st, "src.md", "x.md"); got != "a/x.md" {
		t.Fatal(got)
	}
}

func TestAliasResolution(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "real.md", "---\naliases: [Nick Name]\n---\n")
	write(t, dir, "src.md", "[[nick name]]")
	reconcile(t, ix)
	if got := dst(t, st, "src.md", "nick name"); got != "real.md" {
		t.Fatal(got)
	}
}

func TestMDLinkRelative(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "a/x.md", "")
	write(t, dir, "b/x.md", "")
	write(t, dir, "b/src.md", "[l](x.md) [m](../a/x.md)")
	reconcile(t, ix)
	if got := dst(t, st, "b/src.md", "x.md"); got != "b/x.md" {
		t.Fatal(got)
	}
	if got := dst(t, st, "b/src.md", "../a/x.md"); got != "a/x.md" {
		t.Fatal(got)
	}
}

func TestAddingNoteResolvesDangling(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "src.md", "[[later]]")
	reconcile(t, ix)
	if got := dst(t, st, "src.md", "later"); got != "<nil>" {
		t.Fatal(got)
	}
	write(t, dir, "later.md", "")
	reconcile(t, ix)
	if got := dst(t, st, "src.md", "later"); got != "later.md" {
		t.Fatal(got)
	}
	os.Remove(filepath.Join(dir, "later.md"))
	reconcile(t, ix)
	if got := dst(t, st, "src.md", "later"); got != "<nil>" {
		t.Fatalf("removed target should dangle: %s", got)
	}
}

func TestAliasChangeReresolves(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "t.md", "")
	write(t, dir, "src.md", "[[nick]]")
	reconcile(t, ix)
	write(t, dir, "t.md", "---\naliases: [nick]\n---\n")
	reconcile(t, ix)
	if got := dst(t, st, "src.md", "nick"); got != "t.md" {
		t.Fatal(got)
	}
}

func TestDotDirsAndSymlinksIgnored(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, ".trash/a.md", "")
	write(t, dir, ".git/b.md", "")
	write(t, dir, ".quick.md", "")
	write(t, dir, "real.md", "")
	write(t, dir, "outside/o.md", "")
	os.Symlink(filepath.Join(dir, "real.md"), filepath.Join(dir, "link.md"))
	os.Symlink(filepath.Join(dir, "outside"), filepath.Join(dir, "linkdir"))
	s := reconcile(t, ix)
	if s.Added != 2 || count(t, st, "SELECT count(*) FROM notes WHERE id IN ('real.md','outside/o.md')") != 2 {
		t.Fatalf("%+v", s)
	}
}

func TestIndexFileAndRemoveFile(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "src.md", "[[n]]")
	reconcile(t, ix)
	write(t, dir, "n.md", "hello")
	if err := ix.IndexFile("n.md"); err != nil {
		t.Fatal(err)
	}
	if dst(t, st, "src.md", "n") != "n.md" {
		t.Fatal("not resolved after IndexFile")
	}
	if err := ix.RemoveFile("n.md"); err != nil {
		t.Fatal(err)
	}
	if dst(t, st, "src.md", "n") != "<nil>" || count(t, st, "SELECT count(*) FROM notes WHERE id='n.md'") != 0 {
		t.Fatal("RemoveFile")
	}
}

func TestRebuild(t *testing.T) {
	dir, st, ix := setup(t)
	write(t, dir, "a.md", "x")
	reconcile(t, ix)
	st.DB.Exec("INSERT INTO pins VALUES('a.md',1)")
	if err := ix.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count(t, st, "SELECT count(*) FROM notes") != 1 || count(t, st, "SELECT count(*) FROM pins") != 1 {
		t.Fatal("rebuild")
	}
	if ix.LastStats().Added != 1 {
		t.Fatalf("%+v", ix.LastStats())
	}
}

func BenchmarkNoopReconcile2000(b *testing.B) {
	dir, _, ix := setup(b)
	for i := 0; i < 2000; i++ {
		write(b, dir, fmt.Sprintf("d%d/note%d.md", i%20, i), fmt.Sprintf("---\ntags: [t%d]\n---\n# Title %d\nlink [[note%d]] body text\n", i%7, i, (i+1)%2000))
	}
	reconcile(b, ix)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := reconcile(b, ix)
		if s.Scanned != 2000 || s.Added+s.Updated+s.Touched+s.Removed != 0 {
			b.Fatalf("%+v", s)
		}
	}
}

func TestSymlinkedRootIndexed(t *testing.T) {
	real, st, _ := setup(t)
	write(t, real, "a.md", "[[b]]")
	write(t, real, "sub/b.md", "x")
	link := filepath.Join(t.TempDir(), "vault-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	ix := New(st, link)
	s := reconcile(t, ix)
	if s.Added != 2 || dst(t, st, "a.md", "b") != "sub/b.md" {
		t.Fatalf("%+v", s)
	}
}

func TestNoopReconcileBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("timing test")
	}
	dir, _, ix := setup(t)
	for i := 0; i < 2000; i++ {
		write(t, dir, fmt.Sprintf("d%d/note%d.md", i%20, i), fmt.Sprintf("---\ntags: [t%d]\n---\n# Title %d\nlink [[note%d]] body text\n", i%7, i, (i+1)%2000))
	}
	reconcile(t, ix)
	s := reconcile(t, ix)
	if s.Scanned != 2000 || s.Added+s.Updated+s.Touched+s.Removed != 0 {
		t.Fatalf("%+v", s)
	}
	if s.Elapsed > 100*time.Millisecond {
		t.Fatalf("no-op reconcile took %v, budget 100ms", s.Elapsed)
	}
}
