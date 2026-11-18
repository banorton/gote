package diagram

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

	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/ops"
)

type IDReq struct {
	ID string `json:"id"`
}

type ReadResp struct {
	ID      string        `json:"id"`
	Path    string        `json:"path"`
	Summary excal.Summary `json:"summary"`
}

type DescribeReq struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Clear       bool   `json:"clear"`
}

type DescribeResp struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type CheckResp struct {
	Issues []excal.Issue `json:"issues"`
}

type CreateReq struct {
	Name string `json:"name"`
}

type CreateResp struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type ListReq struct {
	Limit int `json:"limit"`
}

type ListItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
}

func init() {
	ops.Handle("diagram.list", listDiagrams)
	ops.Handle("diagram.read", func(ctx context.Context, env *ops.Env, req IDReq) (ReadResp, error) {
		abs, d, err := load(env, req.ID)
		if err != nil {
			return ReadResp{}, err
		}
		return ReadResp{ID: req.ID, Path: abs, Summary: excal.Summarize(d)}, nil
	})
	ops.Handle("diagram.check", func(ctx context.Context, env *ops.Env, req IDReq) (CheckResp, error) {
		_, d, err := load(env, req.ID)
		if err != nil {
			return CheckResp{}, err
		}
		iss := excal.Check(d)
		if iss == nil {
			iss = []excal.Issue{}
		}
		return CheckResp{Issues: iss}, nil
	})
	ops.Handle("diagram.create", createDiagram)
	ops.Handle("diagram.describe", describeDiagram)
}

func describeDiagram(ctx context.Context, env *ops.Env, req DescribeReq) (DescribeResp, error) {
	desc := strings.TrimSpace(req.Description)
	switch {
	case req.Clear && desc != "":
		return DescribeResp{}, ops.Errorf(ops.CodeInvalid, "description and clear are mutually exclusive")
	case !req.Clear && desc == "":
		return DescribeResp{}, ops.Errorf(ops.CodeInvalid, "description is empty; pass text or clear")
	}
	abs, _, err := load(env, req.ID)
	if err != nil {
		return DescribeResp{}, err
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return DescribeResp{}, ops.Errorf(ops.CodeInternal, "read %s: %v", req.ID, err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return DescribeResp{}, ops.Errorf(ops.CodeInternal, "stat %s: %v", req.ID, err)
	}
	var indexed int64
	err = env.Store.DB.QueryRowContext(ctx, "SELECT mtime_ns FROM notes WHERE id = ?", req.ID).Scan(&indexed)
	if errors.Is(err, sql.ErrNoRows) {
		return DescribeResp{}, ops.Errorf(ops.CodeConflict, "%s is not indexed yet; run again after reindexing", req.ID)
	}
	if err != nil {
		return DescribeResp{}, ops.Errorf(ops.CodeInternal, "lookup %s: %v", req.ID, err)
	}
	if indexed != st.ModTime().UnixNano() {
		return DescribeResp{}, ops.Errorf(ops.CodeConflict, "%s was modified since it was indexed; run again after reindexing", req.ID)
	}
	out, err := excal.SetDescription(src, desc)
	if err != nil {
		return DescribeResp{}, ops.Errorf(ops.CodeInvalid, "%s: %v", req.ID, err)
	}
	if err := writeAtomic(abs, out, st.Mode().Perm()); err != nil {
		return DescribeResp{}, ops.Errorf(ops.CodeInternal, "write %s: %v", req.ID, err)
	}
	if err := env.Index.IndexFile(req.ID); err != nil {
		return DescribeResp{}, ops.Errorf(ops.CodeInternal, "index %s: %v", req.ID, err)
	}
	return DescribeResp{ID: req.ID, Description: desc}, nil
}

// writeAtomic replaces path via a hidden temp file in the same directory.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gote-write-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func load(env *ops.Env, id string) (string, *excal.Doc, error) {
	if !fs.ValidPath(id) || id == "." {
		return "", nil, ops.Errorf(ops.CodeInvalid, "invalid diagram id %q", id)
	}
	if !strings.EqualFold(path.Ext(id), ".excalidraw") {
		return "", nil, ops.Errorf(ops.CodeInvalid, "%s is not a drawing", id)
	}
	abs := filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id))
	src, err := os.ReadFile(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil, ops.Errorf(ops.CodeNotFound, "diagram %s not found", id)
	}
	if err != nil {
		return "", nil, ops.Errorf(ops.CodeInternal, "read %s: %v", id, err)
	}
	d, err := excal.Decode(src)
	if err != nil {
		return "", nil, ops.Errorf(ops.CodeInvalid, "%s: %v", id, err)
	}
	return abs, d, nil
}

func createDiagram(ctx context.Context, env *ops.Env, req CreateReq) (CreateResp, error) {
	name := strings.Join(strings.Fields(req.Name), "_")
	if strings.ContainsAny(name, `/\`) {
		return CreateResp{}, ops.Errorf(ops.CodeInvalid, "diagram name %q must not contain path separators", req.Name)
	}
	dir := filepath.ToSlash(filepath.Clean(env.Cfg.DiagramDir))
	if !filepath.IsLocal(dir) {
		return CreateResp{}, ops.Errorf(ops.CodeInvalid, "diagram_dir %q must be a relative path inside the notes dir", env.Cfg.DiagramDir)
	}
	file := env.Now().Format("060102150405") + "_diagram"
	if name != "" {
		file += "_" + name
	}
	id := path.Join(dir, file+".excalidraw")
	abs := filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return CreateResp{}, ops.Errorf(ops.CodeInternal, "create %s: %v", dir, err)
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return CreateResp{}, ops.Errorf(ops.CodeConflict, "%s already exists", id)
	}
	if err != nil {
		return CreateResp{}, ops.Errorf(ops.CodeInternal, "create %s: %v", id, err)
	}
	_, werr := f.Write(excal.NewScene())
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return CreateResp{}, ops.Errorf(ops.CodeInternal, "write %s: %v", id, werr)
	}
	if err := env.Index.IndexFile(id); err != nil {
		return CreateResp{}, ops.Errorf(ops.CodeInternal, "index %s: %v", id, err)
	}
	return CreateResp{ID: id, Path: abs}, nil
}

func listDiagrams(ctx context.Context, env *ops.Env, req ListReq) ([]ListItem, error) {
	if req.Limit < 0 {
		return nil, ops.Errorf(ops.CodeInvalid, "limit must be >= 0")
	}
	q := "SELECT id, title, mtime_ns FROM notes WHERE kind = 'diagram' ORDER BY mtime_ns DESC, id"
	var args []any
	if req.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, req.Limit)
	}
	rows, err := env.Store.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "list diagrams: %v", err)
	}
	defer rows.Close()
	out := []ListItem{}
	for rows.Next() {
		var it ListItem
		var ns int64
		if err := rows.Scan(&it.ID, &it.Title, &ns); err != nil {
			return nil, ops.Errorf(ops.CodeInternal, "list diagrams: %v", err)
		}
		it.Path = filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(it.ID))
		it.Modified = time.Unix(0, ns).Format("060102150405")
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "list diagrams: %v", err)
	}
	return out, nil
}
