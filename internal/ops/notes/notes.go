package notes

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/dates"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/parse/frontmatter"
	"github.com/goccy/go-yaml"
)

type ResolveReq struct {
	Ref string `json:"ref"`
}

type ResolveResp = Ref

type ReadResp struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Text    string `json:"text"`
	MtimeNS int64  `json:"mtime_ns"`
}

type CreateReq struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	Prefix   string `json:"prefix"`
}

type CreateResp struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

type RecentReq struct {
	Limit int `json:"limit"`
}

type ListReq struct {
	Kind string `json:"kind"`
}

type DupReq struct {
	ID  string `json:"id"`
	New string `json:"new"`
}

type InfoResp struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Kind      string   `json:"kind"`
	Path      string   `json:"path"`
	Created   string   `json:"created"`
	Modified  string   `json:"modified"`
	Size      int64    `json:"size"`
	Tags      []string `json:"tags"`
	Aliases   []string `json:"aliases"`
	Links     int      `json:"links"`
	Backlinks int      `json:"backlinks"`
	Pinned    bool     `json:"pinned"`
}

func TemplatesDir() string { return filepath.Join(filepath.Dir(config.Path()), "templates") }

func templatePath(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || strings.HasPrefix(name, ".") {
		return "", ops.Errorf(ops.CodeInvalid, "invalid template name %q", name)
	}
	return filepath.Join(TemplatesDir(), strings.TrimSuffix(name, ".md")+".md"), nil
}

// withCreated returns src with created set and empty tags/aliases stubbed when
// absent, adding a frontmatter block if needed.
func withCreated(src []byte, ts string) ([]byte, error) {
	present := map[string]bool{}
	if fm, _, ok, _ := frontmatter.Split(src); ok {
		var keys map[string]any
		if yaml.Unmarshal(fm, &keys) == nil {
			for k := range keys {
				present[k] = true
			}
		}
	}
	out, err := frontmatter.Update(src, func(m *frontmatter.Map) {
		m.Set("created", frontmatter.Quoted(ts))
		for _, k := range []string{"tags", "aliases"} {
			if !present[k] {
				m.Set(k, []string{})
			}
		}
	})
	if err != nil {
		return nil, ops.Errorf(ops.CodeInvalid, "set created: %v", err)
	}
	return out, nil
}

func templateBytes(name string) ([]byte, error) {
	if name == "" {
		return nil, nil
	}
	p, err := templatePath(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil, ops.Errorf(ops.CodeNotFound, "template %q not found", name)
	}
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "read template %s: %v", p, err)
	}
	return data, nil
}

func prefixed(name, prefix string, env *ops.Env) (string, error) {
	var p string
	switch prefix {
	case "":
	case "date":
		p = env.Now().Format("060102") + "_"
	case "datetime":
		p = prefixStamp(env) + "_"
	default:
		return "", ops.Errorf(ops.CodeInvalid, "invalid prefix %q (want \"\", \"date\" or \"datetime\")", prefix)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ops.Errorf(ops.CodeInvalid, "empty note name")
	}
	i := strings.LastIndex(name, "/")
	return name[:i+1] + p + name[i+1:], nil
}

func createNote(env *ops.Env, id string, content []byte) (CreateResp, error) {
	out, err := withCreated(content, stamp(env))
	if err != nil {
		return CreateResp{}, err
	}
	p := abs(env, id)
	if err := createExclusive(p, out); err != nil {
		return CreateResp{}, err
	}
	if err := reindex(env, id); err != nil {
		return CreateResp{}, err
	}
	return CreateResp{ID: id, Path: p, Created: true}, nil
}

func init() {
	ops.Handle("notes.resolve", func(ctx context.Context, env *ops.Env, r ResolveReq) (ResolveResp, error) {
		id, err := ops.ResolveNote(ctx, env, r.Ref)
		if err != nil {
			return ResolveResp{}, err
		}
		title, kind, err := noteRow(ctx, env, id)
		if err != nil {
			return ResolveResp{}, err
		}
		return refOf(env, id, title, kind), nil
	})

	ops.Handle("notes.read", func(ctx context.Context, env *ops.Env, r IDReq) (ReadResp, error) {
		if err := checkID(r.ID); err != nil {
			return ReadResp{}, err
		}
		data, mt, err := readFile(abs(env, r.ID))
		if err != nil {
			return ReadResp{}, err
		}
		return ReadResp{ID: r.ID, Path: abs(env, r.ID), Text: string(data), MtimeNS: mt}, nil
	})

	ops.Handle("notes.list", func(ctx context.Context, env *ops.Env, r ListReq) ([]Ref, error) {
		kind := r.Kind
		if kind == "" {
			kind = "note"
		}
		return queryRefs(ctx, env, "SELECT id, title, kind FROM notes WHERE kind=? ORDER BY lower(title), id", kind)
	})

	ops.Handle("notes.create", func(ctx context.Context, env *ops.Env, r CreateReq) (CreateResp, error) {
		name, err := prefixed(r.Name, r.Prefix, env)
		if err != nil {
			return CreateResp{}, err
		}
		id, err := newID(name, "", ".md")
		if err != nil {
			return CreateResp{}, err
		}
		tpl, err := templateBytes(r.Template)
		if err != nil {
			return CreateResp{}, err
		}
		return createNote(env, id, tpl)
	})

	ops.Handle("notes.visit", func(ctx context.Context, env *ops.Env, r IDReq) (ops.Empty, error) {
		if _, _, err := noteRow(ctx, env, r.ID); err != nil {
			return ops.Empty{}, err
		}
		_, err := env.Store.DB.ExecContext(ctx, "INSERT OR REPLACE INTO visits VALUES(?,?)", r.ID, env.Now().UnixNano())
		return ops.Empty{}, dbErr("visit", err)
	})

	ops.Handle("notes.recent", func(ctx context.Context, env *ops.Env, r RecentReq) ([]Ref, error) {
		limit := r.Limit
		if limit <= 0 {
			limit = 50
		}
		return queryRefs(ctx, env, `SELECT n.id, n.title, n.kind FROM visits v JOIN notes n ON n.id=v.note_id
			ORDER BY v.last_visited DESC LIMIT ?`, limit)
	})

	ops.Handle("notes.info", func(ctx context.Context, env *ops.Env, r IDReq) (InfoResp, error) {
		var i InfoResp
		var created sql.NullInt64
		var mtime int64
		err := env.Store.DB.QueryRowContext(ctx, "SELECT id, title, kind, created, mtime_ns, size FROM notes WHERE id=?", r.ID).
			Scan(&i.ID, &i.Title, &i.Kind, &created, &mtime, &i.Size)
		if err == sql.ErrNoRows {
			return i, ops.Errorf(ops.CodeNotFound, "note %q not found", r.ID)
		}
		if err != nil {
			return i, dbErr("info", err)
		}
		i.Path = abs(env, i.ID)
		if created.Valid {
			i.Created = dates.Format(time.Unix(created.Int64, 0))
		}
		i.Modified = dates.Format(timeFromNS(mtime))
		if i.Tags, err = column(ctx, env, "SELECT tag FROM tags WHERE note_id=? ORDER BY rowid", r.ID); err != nil {
			return i, err
		}
		if i.Aliases, err = column(ctx, env, "SELECT alias FROM aliases WHERE note_id=? ORDER BY rowid", r.ID); err != nil {
			return i, err
		}
		err = env.Store.DB.QueryRowContext(ctx, `SELECT
			(SELECT count(*) FROM links WHERE src=?),
			(SELECT count(*) FROM links WHERE dst_id=? AND src<>?)`, r.ID, r.ID, r.ID).Scan(&i.Links, &i.Backlinks)
		if err != nil {
			return i, dbErr("info", err)
		}
		var pinned int
		if err := env.Store.DB.QueryRowContext(ctx, "SELECT count(*) FROM pins WHERE note_id=?", r.ID).Scan(&pinned); err != nil {
			return i, dbErr("info", err)
		}
		i.Pinned = pinned > 0
		return i, nil
	})

	ops.Handle("notes.duplicate", func(ctx context.Context, env *ops.Env, r DupReq) (CreateResp, error) {
		if err := checkID(r.ID); err != nil {
			return CreateResp{}, err
		}
		data, _, err := readFile(abs(env, r.ID))
		if err != nil {
			return CreateResp{}, err
		}
		id, err := newID(r.New, dirOf(r.ID), extOf(r.ID))
		if err != nil {
			return CreateResp{}, err
		}
		return createNote(env, id, data)
	})
}
