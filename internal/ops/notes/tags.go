package notes

import (
	"context"
	"slices"
	"strings"

	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/parse"
	"github.com/banorton/gote/internal/parse/frontmatter"
)

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type TagsNotesReq struct {
	Tags []string `json:"tags"`
}

type TagsEditReq struct {
	Tag string   `json:"tag"`
	IDs []string `json:"ids"`
}

func editTags(ctx context.Context, env *ops.Env, r TagsEditReq, edit func(tags []string, tag string) []string) ([]string, error) {
	tag := strings.TrimSpace(r.Tag)
	if tag == "" {
		return nil, ops.Errorf(ops.CodeInvalid, "empty tag")
	}
	type plan struct {
		id, path string
		data     []byte
		mtime    int64
	}
	var plans []plan
	for _, id := range r.IDs {
		if err := checkID(id); err != nil {
			return nil, err
		}
		p := abs(env, id)
		data, mt, err := readFile(p)
		if err != nil {
			return nil, err
		}
		old := parse.Note(id, data).Tags
		next := edit(slices.Clone(old), tag)
		if slices.Equal(old, next) {
			continue
		}
		if next == nil {
			next = []string{}
		}
		out, err := frontmatter.Update(data, func(m *frontmatter.Map) { m.Set("tags", next) })
		if err != nil {
			return nil, ops.Errorf(ops.CodeInvalid, "%s: %v", id, err)
		}
		plans = append(plans, plan{id, p, out, mt})
	}
	changed := []string{}
	for _, pl := range plans {
		if err := writeChecked(pl.path, pl.data, pl.mtime); err != nil {
			return changed, err
		}
		if err := reindex(env, pl.id); err != nil {
			return changed, err
		}
		changed = append(changed, pl.id)
	}
	return changed, nil
}

func init() {
	ops.Handle("tags.list", func(ctx context.Context, env *ops.Env, _ ops.Empty) ([]TagCount, error) {
		rs, err := env.Store.DB.QueryContext(ctx, "SELECT tag, count(*) c FROM tags GROUP BY tag ORDER BY c DESC, tag")
		if err != nil {
			return nil, dbErr("tags", err)
		}
		defer rs.Close()
		out := []TagCount{}
		for rs.Next() {
			var t TagCount
			if err := rs.Scan(&t.Tag, &t.Count); err != nil {
				return nil, dbErr("tags", err)
			}
			out = append(out, t)
		}
		return out, dbErr("tags", rs.Err())
	})

	ops.Handle("tags.notes", func(ctx context.Context, env *ops.Env, r TagsNotesReq) ([]Ref, error) {
		if len(r.Tags) == 0 {
			return nil, ops.Errorf(ops.CodeInvalid, "no tags given")
		}
		uniq := slices.Compact(slices.Sorted(slices.Values(r.Tags)))
		args := make([]any, 0, len(uniq)+1)
		for _, t := range uniq {
			args = append(args, t)
		}
		args = append(args, len(uniq))
		return queryRefs(ctx, env, `SELECT n.id, n.title, n.kind FROM notes n WHERE n.id IN (
			SELECT note_id FROM tags WHERE tag IN (`+strings.TrimSuffix(strings.Repeat("?,", len(uniq)), ",")+`)
			GROUP BY note_id HAVING count(DISTINCT tag)=?) ORDER BY lower(n.title), n.id`, args...)
	})

	ops.Handle("tags.add", func(ctx context.Context, env *ops.Env, r TagsEditReq) ([]string, error) {
		return editTags(ctx, env, r, func(tags []string, tag string) []string {
			if slices.Contains(tags, tag) {
				return tags
			}
			return append(tags, tag)
		})
	})

	ops.Handle("tags.remove", func(ctx context.Context, env *ops.Env, r TagsEditReq) ([]string, error) {
		return editTags(ctx, env, r, func(tags []string, tag string) []string {
			return slices.DeleteFunc(tags, func(t string) bool { return t == tag })
		})
	})
}
