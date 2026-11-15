package notes

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/banorton/gote/internal/ops"
)

const tsLayout = "060102150405"

type IDReq struct {
	ID string `json:"id"`
}

type Ref struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Path  string `json:"path"`
}

func abs(env *ops.Env, id string) string {
	return filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id))
}

func hasNoteExt(s string) bool {
	e := strings.ToLower(path.Ext(s))
	return e == ".md" || e == ".excalidraw"
}

func validSegments(p string) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return ops.Errorf(ops.CodeInvalid, "invalid path %q", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || strings.HasPrefix(seg, ".") {
			return ops.Errorf(ops.CodeInvalid, "invalid path %q: empty or dot-prefixed segment", p)
		}
	}
	return nil
}

// checkID validates a caller-supplied note id before it touches the filesystem.
func checkID(id string) error {
	if err := validSegments(id); err != nil {
		return err
	}
	if !hasNoteExt(id) {
		return ops.Errorf(ops.CodeInvalid, "invalid note id %q: unsupported extension", id)
	}
	return nil
}

func readFile(p string) ([]byte, int64, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, 0, ops.Errorf(ops.CodeNotFound, "file %s not found", p)
		}
		return nil, 0, ops.Errorf(ops.CodeInternal, "read %s: %v", p, err)
	}
	st, err := os.Stat(p)
	if err != nil {
		return nil, 0, ops.Errorf(ops.CodeInternal, "stat %s: %v", p, err)
	}
	return data, st.ModTime().UnixNano(), nil
}

// checkMtime fails with a conflict unless p still has the expected mtime.
func checkMtime(p string, expect int64) error {
	st, err := os.Stat(p)
	if err != nil {
		return ops.Errorf(ops.CodeConflict, "%s changed on disk: %v", p, err)
	}
	if st.ModTime().UnixNano() != expect {
		return ops.Errorf(ops.CodeConflict, "%s was modified since it was read", p)
	}
	return nil
}

// writeChecked overwrites p atomically after verifying its mtime.
func writeChecked(p string, data []byte, expect int64) error {
	if err := checkMtime(p, expect); err != nil {
		return err
	}
	st, err := os.Stat(p)
	if err != nil {
		return ops.Errorf(ops.CodeInternal, "stat %s: %v", p, err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".gote-tmp-*")
	if err != nil {
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	if err := tmp.Close(); err != nil {
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	if err := os.Chmod(tmp.Name(), st.Mode().Perm()); err != nil {
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	return nil
}

// createExclusive writes a new file, failing with conflict if it exists.
func createExclusive(p string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return ops.Errorf(ops.CodeInternal, "create dir for %s: %v", p, err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ops.Errorf(ops.CodeConflict, "%s already exists", p)
		}
		return ops.Errorf(ops.CodeInternal, "create %s: %v", p, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	if err := f.Close(); err != nil {
		return ops.Errorf(ops.CodeInternal, "write %s: %v", p, err)
	}
	return nil
}

func reindex(env *ops.Env, id string) error {
	if err := env.Index.IndexFile(id); err != nil {
		return ops.Errorf(ops.CodeInternal, "index %s: %v", id, err)
	}
	return nil
}

func dbErr(what string, err error) error {
	if err == nil {
		return nil
	}
	return ops.Errorf(ops.CodeInternal, "%s: %v", what, err)
}

func refOf(env *ops.Env, id, title, kind string) Ref {
	return Ref{ID: id, Title: title, Kind: kind, Path: abs(env, id)}
}

func queryRefs(ctx context.Context, env *ops.Env, q string, args ...any) ([]Ref, error) {
	rs, err := env.Store.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "query: %v", err)
	}
	defer rs.Close()
	out := []Ref{}
	for rs.Next() {
		var id, title, kind string
		if err := rs.Scan(&id, &title, &kind); err != nil {
			return nil, ops.Errorf(ops.CodeInternal, "query: %v", err)
		}
		out = append(out, refOf(env, id, title, kind))
	}
	return out, dbErr("query", rs.Err())
}

func noteRow(ctx context.Context, env *ops.Env, id string) (title, kind string, err error) {
	err = env.Store.DB.QueryRowContext(ctx, "SELECT title, kind FROM notes WHERE id=?", id).Scan(&title, &kind)
	if err == sql.ErrNoRows {
		return "", "", ops.Errorf(ops.CodeNotFound, "note %q not found", id)
	}
	return title, kind, dbErr("lookup "+id, err)
}

func stamp(env *ops.Env) string { return env.Now().Format(tsLayout) }

// newID builds the id for a create/rename/duplicate target. Without a "/" the
// directory of dir is kept; ext defaults to defExt.
func newID(name, dir, defExt string) (string, error) {
	name = strings.TrimSpace(name)
	if !hasNoteExt(name) {
		name += defExt
	}
	if !strings.Contains(name, "/") && dir != "" && dir != "." {
		name = dir + "/" + name
	}
	if err := validSegments(name); err != nil {
		return "", err
	}
	return name, nil
}

func dirOf(id string) string { return path.Dir(id) }

func extOf(id string) string { return path.Ext(id) }

func timeFromNS(ns int64) time.Time { return time.Unix(0, ns) }

func column(ctx context.Context, env *ops.Env, q string, args ...any) ([]string, error) {
	rs, err := env.Store.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "query: %v", err)
	}
	defer rs.Close()
	out := []string{}
	for rs.Next() {
		var s string
		if err := rs.Scan(&s); err != nil {
			return nil, ops.Errorf(ops.CodeInternal, "query: %v", err)
		}
		out = append(out, s)
	}
	return out, dbErr("query", rs.Err())
}
