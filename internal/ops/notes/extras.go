package notes

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/ops"
)

type PathResp struct {
	Path string `json:"path"`
}

type QuickSaveReq struct {
	Name string `json:"name"`
}

type DailyReq struct {
	Offset int `json:"offset"`
}

type TemplateReq struct {
	Name string `json:"name"`
}

type TemplateResp struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Text string `json:"text"`
}

type TemplateRenameReq struct {
	Name string `json:"name"`
	New  string `json:"new"`
}

type ExportReq struct {
	Path string `json:"path"`
}

type ExportResp struct {
	Path  string `json:"path"`
	Files int    `json:"files"`
}

type ImportReq struct {
	Path  string `json:"path"`
	Force bool   `json:"force"`
}

type ImportResp struct {
	Files int `json:"files"`
}

type pinRow struct {
	ID       string `json:"id"`
	PinnedAt int64  `json:"pinned_at"`
}

func quickPath(env *ops.Env) string { return filepath.Join(env.Cfg.NotesDir, ".quick.md") }

func tplName(name string) string { return strings.TrimSuffix(strings.TrimSpace(name), ".md") }

func init() {
	ops.Handle("quick.open", func(ctx context.Context, env *ops.Env, _ ops.Empty) (PathResp, error) {
		p := quickPath(env)
		if err := createExclusive(p, nil); err != nil {
			var oe *ops.Error
			if !errors.As(err, &oe) || oe.Code != ops.CodeConflict {
				return PathResp{}, err
			}
		}
		return PathResp{Path: p}, nil
	})

	ops.Handle("quick.save", func(ctx context.Context, env *ops.Env, r QuickSaveReq) (CreateResp, error) {
		p := quickPath(env)
		data, mt, err := readFile(p)
		if err != nil {
			return CreateResp{}, err
		}
		if strings.TrimSpace(string(data)) == "" {
			return CreateResp{}, ops.Errorf(ops.CodeInvalid, "quick note is empty")
		}
		id, err := newID(r.Name, "", ".md")
		if err != nil {
			return CreateResp{}, err
		}
		resp, err := createNote(env, id, data)
		if err != nil {
			return CreateResp{}, err
		}
		if err := writeChecked(p, nil, mt); err != nil {
			return resp, err
		}
		return resp, nil
	})

	ops.Handle("daily.open", func(ctx context.Context, env *ops.Env, r DailyReq) (CreateResp, error) {
		day := env.Now().AddDate(0, 0, r.Offset)
		id := "daily/" + day.Format("060102") + "_daily.md"
		p := abs(env, id)
		if _, err := os.Stat(p); err == nil {
			return CreateResp{ID: id, Path: p}, nil
		}
		if r.Offset != 0 {
			return CreateResp{}, ops.Errorf(ops.CodeNotFound, "no daily note for %s", day.Format("2006-01-02"))
		}
		tpl, err := templateBytes("daily")
		var oe *ops.Error
		if err != nil && !(errors.As(err, &oe) && oe.Code == ops.CodeNotFound) {
			return CreateResp{}, err
		}
		return createNote(env, id, tpl, "daily")
	})

	ops.Handle("templates.list", func(ctx context.Context, env *ops.Env, _ ops.Empty) ([]string, error) {
		es, err := os.ReadDir(TemplatesDir())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, ops.Errorf(ops.CodeInternal, "list templates: %v", err)
		}
		out := []string{}
		for _, e := range es {
			if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".md") && !strings.HasPrefix(e.Name(), ".") {
				out = append(out, strings.TrimSuffix(e.Name(), ".md"))
			}
		}
		sort.Strings(out)
		return out, nil
	})

	ops.Handle("templates.read", func(ctx context.Context, env *ops.Env, r TemplateReq) (TemplateResp, error) {
		p, err := templatePath(tplName(r.Name))
		if err != nil {
			return TemplateResp{}, err
		}
		data, _, err := readFile(p)
		if err != nil {
			return TemplateResp{}, ops.Errorf(ops.CodeNotFound, "template %q not found", r.Name)
		}
		return TemplateResp{Name: tplName(r.Name), Path: p, Text: string(data)}, nil
	})

	ops.Handle("templates.ensure", func(ctx context.Context, env *ops.Env, r TemplateReq) (TemplateResp, error) {
		p, err := templatePath(tplName(r.Name))
		if err != nil {
			return TemplateResp{}, err
		}
		if err := createExclusive(p, nil); err != nil {
			var oe *ops.Error
			if !errors.As(err, &oe) || oe.Code != ops.CodeConflict {
				return TemplateResp{}, err
			}
		}
		return TemplateResp{Name: tplName(r.Name), Path: p}, nil
	})

	ops.Handle("templates.rename", func(ctx context.Context, env *ops.Env, r TemplateRenameReq) (TemplateResp, error) {
		from, err := templatePath(tplName(r.Name))
		if err != nil {
			return TemplateResp{}, err
		}
		to, err := templatePath(tplName(r.New))
		if err != nil {
			return TemplateResp{}, err
		}
		if _, err := os.Stat(from); err != nil {
			return TemplateResp{}, ops.Errorf(ops.CodeNotFound, "template %q not found", r.Name)
		}
		if _, err := os.Lstat(to); err == nil {
			return TemplateResp{}, ops.Errorf(ops.CodeConflict, "template %q already exists", r.New)
		}
		if err := os.Rename(from, to); err != nil {
			return TemplateResp{}, ops.Errorf(ops.CodeInternal, "rename template: %v", err)
		}
		return TemplateResp{Name: tplName(r.New), Path: to}, nil
	})

	ops.Handle("templates.delete", func(ctx context.Context, env *ops.Env, r TemplateReq) (ops.Empty, error) {
		p, err := templatePath(tplName(r.Name))
		if err != nil {
			return ops.Empty{}, err
		}
		if err := os.Remove(p); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return ops.Empty{}, ops.Errorf(ops.CodeNotFound, "template %q not found", r.Name)
			}
			return ops.Empty{}, ops.Errorf(ops.CodeInternal, "delete template: %v", err)
		}
		return ops.Empty{}, nil
	})

	ops.Handle("export", exportOp)
	ops.Handle("import", importOp)
}

func addFile(tw *tar.Writer, name, p string, info fs.FileInfo) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: info.Size(), ModTime: info.ModTime()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func addTree(tw *tar.Writer, prefix, root, skip string) (int, error) {
	n := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && p == root {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() || p == skip {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		n++
		return addFile(tw, prefix+filepath.ToSlash(rel), p, info)
	})
	return n, err
}

func addBytes(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func exportOp(ctx context.Context, env *ops.Env, r ExportReq) (ExportResp, error) {
	if !filepath.IsAbs(r.Path) {
		return ExportResp{}, ops.Errorf(ops.CodeInvalid, "export path must be absolute: %s", r.Path)
	}
	rs, err := env.Store.DB.QueryContext(ctx, "SELECT note_id, pinned_at FROM pins ORDER BY pinned_at")
	if err != nil {
		return ExportResp{}, dbErr("pins", err)
	}
	pins := []pinRow{}
	for rs.Next() {
		var p pinRow
		if err := rs.Scan(&p.ID, &p.PinnedAt); err != nil {
			rs.Close()
			return ExportResp{}, dbErr("pins", err)
		}
		pins = append(pins, p)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return ExportResp{}, dbErr("pins", err)
	}
	pinJSON, err := json.Marshal(pins)
	if err != nil {
		return ExportResp{}, ops.Errorf(ops.CodeInternal, "encode pins: %v", err)
	}

	out, err := os.Create(r.Path)
	if err != nil {
		return ExportResp{}, ops.Errorf(ops.CodeInternal, "create %s: %v", r.Path, err)
	}
	defer out.Close()
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	fail := func(err error) (ExportResp, error) {
		return ExportResp{}, ops.Errorf(ops.CodeInternal, "export: %v", err)
	}
	files, err := addTree(tw, "notes/", env.Cfg.NotesDir, r.Path)
	if err != nil {
		return fail(err)
	}
	n, err := addTree(tw, "templates/", TemplatesDir(), "")
	if err != nil {
		return fail(err)
	}
	files += n
	if cfg, err := os.ReadFile(config.Path()); err == nil {
		if err := addBytes(tw, "config.yaml", cfg); err != nil {
			return fail(err)
		}
		files++
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fail(err)
	}
	if err := addBytes(tw, "pins.json", pinJSON); err != nil {
		return fail(err)
	}
	if err := tw.Close(); err != nil {
		return fail(err)
	}
	if err := gz.Close(); err != nil {
		return fail(err)
	}
	if err := out.Close(); err != nil {
		return fail(err)
	}
	return ExportResp{Path: r.Path, Files: files}, nil
}

func destFor(env *ops.Env, name string) (string, bool, error) {
	clean := path.Clean(name)
	if clean != strings.TrimSuffix(name, "/") || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false, ops.Errorf(ops.CodeInvalid, "unsafe path in archive: %q", name)
	}
	switch {
	case strings.HasPrefix(clean, "notes/"):
		return filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(strings.TrimPrefix(clean, "notes/"))), true, nil
	case strings.HasPrefix(clean, "templates/"):
		return filepath.Join(TemplatesDir(), filepath.FromSlash(strings.TrimPrefix(clean, "templates/"))), true, nil
	}
	return "", false, nil
}

func importOp(ctx context.Context, env *ops.Env, r ImportReq) (ImportResp, error) {
	f, err := os.Open(r.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ImportResp{}, ops.Errorf(ops.CodeNotFound, "archive %s not found", r.Path)
		}
		return ImportResp{}, ops.Errorf(ops.CodeInternal, "open %s: %v", r.Path, err)
	}
	defer f.Close()
	if !r.Force {
		es, err := os.ReadDir(env.Cfg.NotesDir)
		if err != nil {
			return ImportResp{}, ops.Errorf(ops.CodeInternal, "read notes dir: %v", err)
		}
		if len(es) > 0 {
			return ImportResp{}, ops.Errorf(ops.CodeConflict, "notes dir %s is not empty; use force to import anyway", env.Cfg.NotesDir)
		}
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return ImportResp{}, ops.Errorf(ops.CodeInvalid, "%s is not a gzip archive: %v", r.Path, err)
	}
	tr := tar.NewReader(gz)
	var pins []pinRow
	files := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ImportResp{}, ops.Errorf(ops.CodeInvalid, "read archive: %v", err)
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if h.Name == "pins.json" {
			if err := json.NewDecoder(tr).Decode(&pins); err != nil {
				return ImportResp{}, ops.Errorf(ops.CodeInvalid, "pins.json: %v", err)
			}
			continue
		}
		dst, ok, err := destFor(env, h.Name)
		if err != nil {
			return ImportResp{}, err
		}
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return ImportResp{}, ops.Errorf(ops.CodeInternal, "create dir: %v", err)
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return ImportResp{}, ops.Errorf(ops.CodeInternal, "write %s: %v", dst, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return ImportResp{}, ops.Errorf(ops.CodeInternal, "write %s: %v", dst, err)
		}
		if err := out.Close(); err != nil {
			return ImportResp{}, ops.Errorf(ops.CodeInternal, "write %s: %v", dst, err)
		}
		if err := os.Chtimes(dst, h.ModTime, h.ModTime); err != nil {
			return ImportResp{}, ops.Errorf(ops.CodeInternal, "stamp %s: %v", dst, err)
		}
		files++
	}
	if err := env.Index.Rebuild(ctx); err != nil {
		return ImportResp{}, ops.Errorf(ops.CodeInternal, "rebuild index: %v", err)
	}
	for _, p := range pins {
		if _, err := env.Store.DB.ExecContext(ctx,
			"INSERT OR REPLACE INTO pins SELECT id, ? FROM notes WHERE id=?", p.PinnedAt, p.ID); err != nil {
			return ImportResp{}, dbErr("restore pin", err)
		}
	}
	return ImportResp{Files: files}, nil
}
