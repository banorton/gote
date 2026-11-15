package index

import (
	"context"
	"database/sql"
	"path"
	"sort"
	"strings"
)

type resolver struct {
	byStem  map[string][]string
	byAlias map[string][]string
	byLC    map[string]string
}

func knownExt(s string) bool {
	_, ok := parsers[strings.ToLower(path.Ext(s))]
	return ok
}

func trimKnownExt(s string) string {
	if knownExt(s) {
		return strings.TrimSuffix(s, path.Ext(s))
	}
	return s
}

func newResolver(ctx context.Context, tx *sql.Tx) (*resolver, error) {
	r := &resolver{byStem: map[string][]string{}, byAlias: map[string][]string{}, byLC: map[string]string{}}
	rs, err := tx.QueryContext(ctx, "SELECT id, stem FROM notes")
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id, stem string
		if err := rs.Scan(&id, &stem); err != nil {
			rs.Close()
			return nil, err
		}
		r.byStem[stem] = append(r.byStem[stem], id)
		r.byLC[strings.ToLower(id)] = id
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	rs.Close()
	rs, err = tx.QueryContext(ctx, "SELECT alias_lc, note_id FROM aliases")
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	for rs.Next() {
		var a, id string
		if err := rs.Scan(&a, &id); err != nil {
			return nil, err
		}
		r.byAlias[a] = append(r.byAlias[a], id)
	}
	return r, rs.Err()
}

func pick(c []string) string {
	if len(c) == 0 {
		return ""
	}
	sort.Slice(c, func(i, j int) bool {
		if len(c[i]) != len(c[j]) {
			return len(c[i]) < len(c[j])
		}
		return c[i] < c[j]
	})
	return c[0]
}

// resolve implements the link resolution rule: case-insensitive; a target with
// "/" is a path-suffix match; otherwise filename stem or alias; ties go to the
// shortest path, then lexicographic. md links try a src-relative path first.
func (r *resolver) resolve(src, raw, kind string) string {
	if raw == "" {
		return src
	}
	t := strings.ToLower(strings.TrimSpace(raw))
	if kind == "md" {
		if id, ok := r.byLC[path.Join(path.Dir(src), t)]; ok {
			return id
		}
	}
	hasExt := knownExt(t)
	var cands []string
	if strings.Contains(t, "/") {
		t = strings.TrimPrefix(t, "./")
		for _, id := range r.byStem[path.Base(trimKnownExt(t))] {
			lc := strings.ToLower(id)
			if !hasExt {
				lc = trimKnownExt(lc)
			}
			if lc == t || strings.HasSuffix(lc, "/"+t) {
				cands = append(cands, id)
			}
		}
		return pick(cands)
	}
	for _, id := range r.byStem[trimKnownExt(t)] {
		if !hasExt || strings.ToLower(path.Base(id)) == t {
			cands = append(cands, id)
		}
	}
	cands = append(cands, r.byAlias[t]...)
	return pick(cands)
}

type linkRow struct {
	rowid         int64
	src, raw, knd string
	dst           sql.NullString
}

func applyResolution(ctx context.Context, tx *sql.Tx, rows []linkRow, r *resolver) error {
	for _, l := range rows {
		got := r.resolve(l.src, l.raw, l.knd)
		if (got == "" && !l.dst.Valid) || (l.dst.Valid && l.dst.String == got) {
			continue
		}
		var v any
		if got != "" {
			v = got
		}
		if _, err := tx.ExecContext(ctx, "UPDATE links SET dst_id=? WHERE rowid=?", v, l.rowid); err != nil {
			return err
		}
	}
	return nil
}

func loadLinks(ctx context.Context, tx *sql.Tx, q string, args ...any) ([]linkRow, error) {
	rs, err := tx.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []linkRow
	for rs.Next() {
		var l linkRow
		if err := rs.Scan(&l.rowid, &l.src, &l.raw, &l.knd, &l.dst); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rs.Err()
}

const linkSelect = "SELECT rowid, src, dst_raw, kind, dst_id FROM links"

func resolveAll(ctx context.Context, tx *sql.Tx) error {
	r, err := newResolver(ctx, tx)
	if err != nil {
		return err
	}
	rows, err := loadLinks(ctx, tx, linkSelect)
	if err != nil {
		return err
	}
	return applyResolution(ctx, tx, rows, r)
}

func resolveFor(ctx context.Context, tx *sql.Tx, srcs []string) error {
	r, err := newResolver(ctx, tx)
	if err != nil {
		return err
	}
	for _, s := range srcs {
		rows, err := loadLinks(ctx, tx, linkSelect+" WHERE src=?", s)
		if err != nil {
			return err
		}
		if err := applyResolution(ctx, tx, rows, r); err != nil {
			return err
		}
	}
	return nil
}
