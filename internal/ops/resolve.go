package ops

import (
	"context"
	"database/sql"
	"path"
	"sort"
	"strings"

	"github.com/banorton/gote/internal/note"
)

func hasNoteExt(s string) bool {
	e := strings.ToLower(path.Ext(s))
	return e == ".md" || e == ".excalidraw"
}

func trimNoteExt(s string) string {
	if hasNoteExt(s) {
		return strings.TrimSuffix(s, path.Ext(s))
	}
	return s
}

// ResolveNote maps a user-facing reference to a note id. "-" is the last
// visited note; an exact id wins; otherwise the link resolution rule applies
// (case-insensitive path suffix, filename stem or alias, shortest path first).
func ResolveNote(ctx context.Context, env *Env, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", Errorf(CodeInvalid, "empty note reference")
	}
	db := env.Store.DB
	if ref == "-" {
		var id string
		err := db.QueryRowContext(ctx, `SELECT v.note_id FROM visits v JOIN notes n ON n.id = v.note_id
			ORDER BY v.last_visited DESC LIMIT 1`).Scan(&id)
		if err == sql.ErrNoRows {
			return "", Errorf(CodeNotFound, "no last visited note")
		}
		if err != nil {
			return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
		}
		return id, nil
	}
	var id string
	err := db.QueryRowContext(ctx, "SELECT id FROM notes WHERE id=?", ref).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
	}

	t := strings.TrimPrefix(note.Fold(ref), "./")
	hasExt := hasNoteExt(t)
	var cands []string
	collect := func(q string, args ...any) error {
		rs, err := db.QueryContext(ctx, q, args...)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				return err
			}
			cands = append(cands, id)
		}
		return rs.Err()
	}
	if strings.Contains(t, "/") {
		rs, err := db.QueryContext(ctx, "SELECT id FROM notes WHERE stem=?", path.Base(trimNoteExt(t)))
		if err != nil {
			return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
		}
		defer rs.Close()
		for rs.Next() {
			var id string
			if err := rs.Scan(&id); err != nil {
				return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
			}
			lc := note.Fold(id)
			if !hasExt {
				lc = trimNoteExt(lc)
			}
			if lc == t || strings.HasSuffix(lc, "/"+t) {
				cands = append(cands, id)
			}
		}
		if err := rs.Err(); err != nil {
			return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
		}
	} else {
		if err := collect("SELECT id FROM notes WHERE stem=?", trimNoteExt(t)); err != nil {
			return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
		}
		if hasExt {
			kept := cands[:0]
			for _, id := range cands {
				if note.Fold(path.Base(id)) == t {
					kept = append(kept, id)
				}
			}
			cands = kept
		}
		if err := collect("SELECT note_id FROM aliases WHERE alias_lc=?", t); err != nil {
			return "", Errorf(CodeInternal, "resolve %q: %v", ref, err)
		}
	}
	if len(cands) == 0 {
		return "", Errorf(CodeNotFound, "note %q not found", ref)
	}
	sort.Slice(cands, func(i, j int) bool {
		if len(cands[i]) != len(cands[j]) {
			return len(cands[i]) < len(cands[j])
		}
		return cands[i] < cands[j]
	})
	return cands[0], nil
}
