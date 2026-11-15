package diagram

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

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

func init() {
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
