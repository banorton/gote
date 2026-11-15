package notes

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/banorton/gote/internal/ops"
)

type DeleteResp struct {
	TrashPath string `json:"trash_path"`
}

type TrashEntry struct {
	TrashPath  string `json:"trash_path"`
	OriginalID string `json:"original_id"`
	Deleted    string `json:"deleted"`

	at time.Time
}

type RecoverReq struct {
	TrashPath  string `json:"trash_path"`
	OriginalID string `json:"original_id"`
}

type RecoverResp struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type TrashEmptyResp struct {
	Removed int `json:"removed"`
}

var trashSuffix = regexp.MustCompile(`_\d{12}(\.[^./]+)$`)

func trashDir(env *ops.Env) string { return filepath.Join(env.Cfg.NotesDir, ".trash") }

func listTrash(env *ops.Env) ([]TrashEntry, error) {
	root := trashDir(env)
	var out []TrashEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && p == root {
				return nil
			}
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		out = append(out, TrashEntry{
			TrashPath:  p,
			OriginalID: trashSuffix.ReplaceAllString(rel, "$1"),
			Deleted:    info.ModTime().Format(tsLayout),
			at:         info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "list trash: %v", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].at.Equal(out[j].at) {
			return out[i].at.After(out[j].at)
		}
		return out[i].TrashPath > out[j].TrashPath
	})
	return out, nil
}

func init() {
	ops.Handle("notes.delete", func(ctx context.Context, env *ops.Env, r IDReq) (DeleteResp, error) {
		if err := checkID(r.ID); err != nil {
			return DeleteResp{}, err
		}
		src := abs(env, r.ID)
		if _, err := os.Lstat(src); err != nil {
			return DeleteResp{}, ops.Errorf(ops.CodeNotFound, "note %q not found", r.ID)
		}
		dst := filepath.Join(trashDir(env), filepath.FromSlash(r.ID))
		if _, err := os.Lstat(dst); err == nil {
			ext := path.Ext(r.ID)
			dst = filepath.Join(trashDir(env), filepath.FromSlash(strings.TrimSuffix(r.ID, ext)+"_"+stamp(env)+ext))
			if _, err := os.Lstat(dst); err == nil {
				return DeleteResp{}, ops.Errorf(ops.CodeConflict, "%s already exists in trash", dst)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return DeleteResp{}, ops.Errorf(ops.CodeInternal, "create trash dir: %v", err)
		}
		if err := os.Rename(src, dst); err != nil {
			return DeleteResp{}, ops.Errorf(ops.CodeInternal, "move to trash: %v", err)
		}
		// mtime doubles as the deletion time shown by `notes.trash`.
		if err := os.Chtimes(dst, env.Now(), env.Now()); err != nil {
			return DeleteResp{}, ops.Errorf(ops.CodeInternal, "stamp trash entry: %v", err)
		}
		if err := env.Index.RemoveFile(r.ID); err != nil {
			return DeleteResp{}, ops.Errorf(ops.CodeInternal, "unindex %s: %v", r.ID, err)
		}
		for _, t := range []string{"pins", "visits"} {
			if _, err := env.Store.DB.ExecContext(ctx, "DELETE FROM "+t+" WHERE note_id=?", r.ID); err != nil {
				return DeleteResp{}, dbErr("clear "+t, err)
			}
		}
		return DeleteResp{TrashPath: dst}, nil
	})

	ops.Handle("notes.trash", func(ctx context.Context, env *ops.Env, _ ops.Empty) ([]TrashEntry, error) {
		es, err := listTrash(env)
		if es == nil {
			es = []TrashEntry{}
		}
		return es, err
	})

	ops.Handle("notes.recover", func(ctx context.Context, env *ops.Env, r RecoverReq) (RecoverResp, error) {
		es, err := listTrash(env)
		if err != nil {
			return RecoverResp{}, err
		}
		var pick *TrashEntry
		for i := range es {
			e := &es[i]
			if (r.TrashPath != "" && (e.TrashPath == r.TrashPath || e.TrashPath == filepath.Join(trashDir(env), filepath.FromSlash(r.TrashPath)))) ||
				(r.TrashPath == "" && r.OriginalID != "" && e.OriginalID == r.OriginalID) {
				pick = e
				break
			}
		}
		if r.TrashPath == "" && r.OriginalID == "" {
			return RecoverResp{}, ops.Errorf(ops.CodeInvalid, "recover needs trash_path or original_id")
		}
		if pick == nil {
			return RecoverResp{}, ops.Errorf(ops.CodeNotFound, "no trash entry for %q", r.TrashPath+r.OriginalID)
		}
		if err := checkID(pick.OriginalID); err != nil {
			return RecoverResp{}, err
		}
		dst := abs(env, pick.OriginalID)
		if _, err := os.Lstat(dst); err == nil {
			return RecoverResp{}, ops.Errorf(ops.CodeConflict, "%s already exists", pick.OriginalID)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return RecoverResp{}, ops.Errorf(ops.CodeInternal, "create dir: %v", err)
		}
		if err := os.Rename(pick.TrashPath, dst); err != nil {
			return RecoverResp{}, ops.Errorf(ops.CodeInternal, "recover: %v", err)
		}
		if err := reindex(env, pick.OriginalID); err != nil {
			return RecoverResp{}, err
		}
		return RecoverResp{ID: pick.OriginalID, Path: dst}, nil
	})

	ops.Handle("notes.trash_empty", func(ctx context.Context, env *ops.Env, _ ops.Empty) (TrashEmptyResp, error) {
		es, err := listTrash(env)
		if err != nil {
			return TrashEmptyResp{}, err
		}
		if err := os.RemoveAll(trashDir(env)); err != nil {
			return TrashEmptyResp{}, ops.Errorf(ops.CodeInternal, "empty trash: %v", err)
		}
		return TrashEmptyResp{Removed: len(es)}, nil
	})
}
