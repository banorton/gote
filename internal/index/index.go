package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/banorton/gote/internal/note"
	"github.com/banorton/gote/internal/parse"
	"github.com/banorton/gote/internal/store"
)

type Stats struct {
	Scanned, Added, Updated, Touched, Removed int
	Elapsed                                   time.Duration
}

// Parser turns file bytes into a Note; the returned Kind selects its table role.
type Parser func(id string, src []byte) note.Note

var parsers = map[string]Parser{}

// RegisterParser makes files with the given extension (".md") indexable. Call
// from init(); panics on duplicates.
func RegisterParser(ext string, p Parser) {
	ext = strings.ToLower(ext)
	if _, dup := parsers[ext]; dup {
		panic("index: duplicate parser for " + ext)
	}
	parsers[ext] = p
}

func init() { RegisterParser(".md", parse.Note) }

type Indexer struct {
	st   *store.Store
	root string

	mu   sync.Mutex
	last Stats

	hookMu sync.Mutex
	hooks  map[int]func(Stats)
	nextID int
}

// OnReconcile subscribes fn to every reconcile, IndexFile or RemoveFile that
// added, updated or removed a note. fn runs on the mutating goroutine after the
// indexer lock is released, so it must not block. The returned func unsubscribes.
func (ix *Indexer) OnReconcile(fn func(Stats)) (cancel func()) {
	ix.hookMu.Lock()
	defer ix.hookMu.Unlock()
	if ix.hooks == nil {
		ix.hooks = map[int]func(Stats){}
	}
	id := ix.nextID
	ix.nextID++
	ix.hooks[id] = fn
	return func() {
		ix.hookMu.Lock()
		defer ix.hookMu.Unlock()
		delete(ix.hooks, id)
	}
}

func (ix *Indexer) notify(st Stats) {
	if st.Added+st.Updated+st.Removed == 0 {
		return
	}
	ix.hookMu.Lock()
	fns := make([]func(Stats), 0, len(ix.hooks))
	for _, f := range ix.hooks {
		fns = append(fns, f)
	}
	ix.hookMu.Unlock()
	for _, f := range fns {
		f(st)
	}
}

// New resolves a symlinked notes dir because WalkDir does not follow a
// symlinked root; a path that cannot be resolved surfaces as a walk error.
func New(st *store.Store, notesDir string) *Indexer {
	if r, err := filepath.EvalSymlinks(notesDir); err == nil {
		notesDir = r
	}
	return &Indexer{st: st, root: notesDir}
}

func (ix *Indexer) LastStats() Stats {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	return ix.last
}

type fileInfo struct{ mtime, size int64 }

type row struct {
	mtime, size int64
	hash        string
}

func parserFor(id string) (Parser, bool) {
	p, ok := parsers[strings.ToLower(path.Ext(id))]
	return p, ok
}

func hidden(id string) bool {
	for _, seg := range strings.Split(id, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func (ix *Indexer) walk(ctx context.Context) (files map[string]fileInfo, attachments []string, err error) {
	files = map[string]fileInfo{}
	err = filepath.WalkDir(ix.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if p == ix.root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(ix.root, p)
		if err != nil {
			return err
		}
		id := filepath.ToSlash(rel)
		if _, ok := parserFor(id); !ok {
			attachments = append(attachments, id)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files[id] = fileInfo{info.ModTime().UnixNano(), info.Size()}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("walk %s: %w", ix.root, err)
	}
	return files, attachments, nil
}

// Reconcile brings the cache in line with the files on disk in one transaction.
func (ix *Indexer) Reconcile(ctx context.Context) (Stats, error) {
	ix.mu.Lock()
	st, err := ix.reconcile(ctx)
	ix.mu.Unlock()
	if err == nil {
		ix.notify(st)
	}
	return st, err
}

func (ix *Indexer) reconcile(ctx context.Context) (Stats, error) {
	start := time.Now()
	files, attachments, err := ix.walk(ctx)
	if err != nil {
		return Stats{}, err
	}
	tx, err := ix.st.DB.BeginTx(ctx, nil)
	if err != nil {
		return Stats{}, err
	}
	defer tx.Rollback()

	existing := map[string]row{}
	rs, err := tx.QueryContext(ctx, "SELECT id, mtime_ns, size, hash FROM notes")
	if err != nil {
		return Stats{}, err
	}
	for rs.Next() {
		var id string
		var r row
		if err := rs.Scan(&id, &r.mtime, &r.size, &r.hash); err != nil {
			rs.Close()
			return Stats{}, err
		}
		existing[id] = r
	}
	if err := rs.Err(); err != nil {
		return Stats{}, err
	}
	rs.Close()

	structural, err := syncAttachments(ctx, tx, attachments)
	if err != nil {
		return Stats{}, err
	}

	st := Stats{Scanned: len(files)}
	ids := make([]string, 0, len(files))
	for id := range files {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var changed []string
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return Stats{}, err
		}
		fi := files[id]
		old, had := existing[id]
		if had && old.mtime == fi.mtime && old.size == fi.size {
			continue
		}
		var oldp *row
		if had {
			oldp = &old
		}
		s, c, err := ix.applyFile(ctx, tx, id, fi, oldp, &st)
		if err != nil {
			return Stats{}, err
		}
		structural = structural || s
		if c {
			changed = append(changed, id)
		}
	}
	for id := range existing {
		if _, ok := files[id]; ok {
			continue
		}
		if err := deleteNote(ctx, tx, id); err != nil {
			return Stats{}, err
		}
		st.Removed++
		structural = true
	}
	if structural {
		err = resolveAll(ctx, tx)
	} else if len(changed) > 0 {
		err = resolveFor(ctx, tx, changed)
	}
	if err != nil {
		return Stats{}, err
	}
	if err := tx.Commit(); err != nil {
		return Stats{}, err
	}
	st.Elapsed = time.Since(start)
	ix.last = st
	return st, nil
}

// syncAttachments replaces the attachment table when the set of non-note files
// changed, reporting whether it did so links get re-resolved.
func syncAttachments(ctx context.Context, tx *sql.Tx, paths []string) (bool, error) {
	have := map[string]bool{}
	rs, err := tx.QueryContext(ctx, "SELECT path FROM attachments")
	if err != nil {
		return false, err
	}
	for rs.Next() {
		var p string
		if err := rs.Scan(&p); err != nil {
			rs.Close()
			return false, err
		}
		have[p] = true
	}
	if err := rs.Err(); err != nil {
		return false, err
	}
	rs.Close()
	same := len(have) == len(paths)
	for _, p := range paths {
		if !have[p] {
			same = false
			break
		}
	}
	if same {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM attachments"); err != nil {
		return false, err
	}
	for _, p := range paths {
		if _, err := tx.ExecContext(ctx, "INSERT INTO attachments VALUES(?,?)", p, note.Fold(p)); err != nil {
			return false, err
		}
	}
	return true, nil
}

func deleteNote(ctx context.Context, tx *sql.Tx, id string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM notes_fts WHERE rowid=(SELECT rowid FROM notes WHERE id=?)", id); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM notes WHERE id=?", id)
	return err
}

// applyFile reads and indexes one file. It reports whether the set of notes or
// aliases changed (structural) and whether the note's rows were rewritten.
func (ix *Indexer) applyFile(ctx context.Context, tx *sql.Tx, id string, fi fileInfo, old *row, st *Stats) (structural, rewritten bool, err error) {
	src, err := os.ReadFile(filepath.Join(ix.root, filepath.FromSlash(id)))
	if err != nil {
		return false, false, fmt.Errorf("read %s: %w", id, err)
	}
	hash := strconv.FormatUint(xxhash.Sum64(src), 16)
	if old != nil && old.hash == hash {
		_, err := tx.ExecContext(ctx, "UPDATE notes SET mtime_ns=?, size=? WHERE id=?", fi.mtime, fi.size, id)
		st.Touched++
		return false, false, err
	}
	p, _ := parserFor(id)
	n := p(id, src)

	var oldAliases []string
	if old != nil {
		oldAliases, err = aliasesOf(ctx, tx, id)
		if err != nil {
			return false, false, err
		}
		if err := deleteNote(ctx, tx, id); err != nil {
			return false, false, err
		}
		st.Updated++
	} else {
		st.Added++
	}
	if err := insertNote(ctx, tx, n, fi, hash); err != nil {
		return false, false, fmt.Errorf("index %s: %w", id, err)
	}
	return old == nil || !sameStrings(oldAliases, lowerAll(n.Aliases)), true, nil
}

func lowerAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = note.Fold(s)
	}
	return out
}

func sameStrings(a, b []string) bool {
	a, b = append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func aliasesOf(ctx context.Context, tx *sql.Tx, id string) ([]string, error) {
	rs, err := tx.QueryContext(ctx, "SELECT alias_lc FROM aliases WHERE note_id=?", id)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var a string
		if err := rs.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rs.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func insertNote(ctx context.Context, tx *sql.Tx, n note.Note, fi fileInfo, hash string) error {
	fields, err := json.Marshal(n.Fields)
	if err != nil {
		return fmt.Errorf("encode frontmatter fields: %w", err)
	}
	sem := 0
	if n.Semantic {
		sem = 1
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO notes(id, kind, title, stem, created, mtime_ns, size, hash, fields, semantic, parse_error)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		n.ID, n.Kind, n.Title, n.Stem, nullable(n.Created), fi.mtime, fi.size, hash, string(fields), sem, nullable(n.ParseErr)); err != nil {
		return err
	}
	for _, a := range n.Aliases {
		if _, err := tx.ExecContext(ctx, "INSERT INTO aliases VALUES(?,?,?)", n.ID, note.Fold(a), a); err != nil {
			return err
		}
	}
	for _, t := range n.Tags {
		if _, err := tx.ExecContext(ctx, "INSERT INTO tags VALUES(?,?)", n.ID, t); err != nil {
			return err
		}
	}
	for _, l := range n.Links {
		if _, err := tx.ExecContext(ctx, "INSERT INTO links(src, dst_raw, dst_id, fragment, alias, kind, line) VALUES(?,?,NULL,?,?,?,?)",
			n.ID, l.Target, l.Fragment, l.Alias, l.Kind, l.Line); err != nil {
			return err
		}
	}
	for _, h := range n.Headings {
		if _, err := tx.ExecContext(ctx, "INSERT INTO headings VALUES(?,?,?,?)", n.ID, h.Level, h.Text, h.Line); err != nil {
			return err
		}
	}
	for _, t := range n.Tasks {
		done := 0
		if t.Done {
			done = 1
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO tasks VALUES(?,?,?,?)", n.ID, t.Line, t.Text, done); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO notes_fts(rowid, note_id, title, aliases, tags, body)
		VALUES((SELECT rowid FROM notes WHERE id=?),?,?,?,?,?)`,
		n.ID, n.ID, n.Title, strings.Join(n.Aliases, " "), strings.Join(n.Tags, " "), n.Body)
	return err
}

// IndexFile re-indexes one file after a write; a missing file is removed.
func (ix *Indexer) IndexFile(id string) error {
	ix.mu.Lock()
	st, err := ix.indexFile(id)
	ix.mu.Unlock()
	if err == nil {
		ix.notify(st)
	}
	return err
}

func (ix *Indexer) indexFile(id string) (Stats, error) {
	if hidden(id) {
		return Stats{}, fmt.Errorf("index %s: path is not indexable", id)
	}
	if _, ok := parserFor(id); !ok {
		return Stats{}, fmt.Errorf("index %s: unsupported file type", id)
	}
	abs := filepath.Join(ix.root, filepath.FromSlash(id))
	info, err := os.Lstat(abs)
	if os.IsNotExist(err) {
		return ix.removeFile(id)
	}
	if err != nil {
		return Stats{}, err
	}
	if !info.Mode().IsRegular() {
		return Stats{}, fmt.Errorf("index %s: not a regular file", id)
	}
	ctx := context.Background()
	tx, err := ix.st.DB.BeginTx(ctx, nil)
	if err != nil {
		return Stats{}, err
	}
	defer tx.Rollback()
	var old *row
	var r row
	switch err := tx.QueryRowContext(ctx, "SELECT mtime_ns, size, hash FROM notes WHERE id=?", id).Scan(&r.mtime, &r.size, &r.hash); err {
	case nil:
		old = &r
	case sql.ErrNoRows:
	default:
		return Stats{}, err
	}
	var st Stats
	structural, rewritten, err := ix.applyFile(ctx, tx, id, fileInfo{info.ModTime().UnixNano(), info.Size()}, old, &st)
	if err != nil {
		return Stats{}, err
	}
	if structural {
		err = resolveAll(ctx, tx)
	} else if rewritten {
		err = resolveFor(ctx, tx, []string{id})
	}
	if err != nil {
		return Stats{}, err
	}
	return st, tx.Commit()
}

func (ix *Indexer) RemoveFile(id string) error {
	ix.mu.Lock()
	st, err := ix.removeFile(id)
	ix.mu.Unlock()
	if err == nil {
		ix.notify(st)
	}
	return err
}

func (ix *Indexer) removeFile(id string) (Stats, error) {
	ctx := context.Background()
	tx, err := ix.st.DB.BeginTx(ctx, nil)
	if err != nil {
		return Stats{}, err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM notes WHERE id=?", id).Scan(&n); err != nil {
		return Stats{}, err
	}
	if err := deleteNote(ctx, tx, id); err != nil {
		return Stats{}, err
	}
	if err := resolveAll(ctx, tx); err != nil {
		return Stats{}, err
	}
	return Stats{Removed: n}, tx.Commit()
}

// Rebuild drops and recreates the cache, then reconciles.
func (ix *Indexer) Rebuild(ctx context.Context) error {
	ix.mu.Lock()
	if err := ix.st.ResetCache(); err != nil {
		ix.mu.Unlock()
		return err
	}
	st, err := ix.reconcile(ctx)
	ix.mu.Unlock()
	if err == nil {
		ix.notify(st)
	}
	return err
}
