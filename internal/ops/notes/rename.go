package notes

import (
	"context"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/banorton/gote/internal/ops"
)

type RenameReq struct {
	ID  string `json:"id"`
	New string `json:"new"`
}

type RenameResp struct {
	ID        string   `json:"id"`
	Path      string   `json:"path"`
	Rewritten []string `json:"rewritten"`
}

var (
	wikiRe = regexp.MustCompile(`(!?)\[\[([^\[\]|#\n]*)((?:#[^\[\]|\n]*)?)((?:\|[^\[\]\n]*)?)\]\]`)
	mdRe   = regexp.MustCompile(`(\[[^\]\n]*\]\()([^)\s#]+)((?:#[^)\s]*)?)\)`)
)

func trimExt(s string) string {
	if hasNoteExt(s) {
		return strings.TrimSuffix(s, path.Ext(s))
	}
	return s
}

// codeSpans returns [start,end) byte ranges of inline code in line.
func codeSpans(line string) [][2]int {
	var out [][2]int
	for i := 0; i < len(line); {
		if line[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(line) && line[j] == '`' {
			j++
		}
		n := j - i
		k := j
		closed := false
		for k < len(line) {
			if line[k] != '`' {
				k++
				continue
			}
			m := k
			for m < len(line) && line[m] == '`' {
				m++
			}
			if m-k == n {
				out = append(out, [2]int{i, m})
				i, closed = m, true
				break
			}
			k = m
		}
		if !closed {
			i = j
		}
	}
	return out
}

func inSpan(spans [][2]int, pos int) bool {
	for _, s := range spans {
		if pos >= s[0] && pos < s[1] {
			return true
		}
	}
	return false
}

type renamer struct {
	oldID, newID string
	oldNoExt     string
	oldStem      string
}

func (r *renamer) namesOld(raw string) bool {
	t := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), "./")
	t = trimExt(t)
	if strings.Contains(t, "/") {
		return r.oldNoExt == t || strings.HasSuffix(r.oldNoExt, "/"+t)
	}
	return t == r.oldStem
}

func (r *renamer) wikiTarget(raw string) string {
	t := strings.TrimPrefix(strings.TrimSpace(raw), "./")
	ext := ""
	if hasNoteExt(t) {
		ext = path.Ext(t)
	}
	newNoExt := trimExt(r.newID)
	if !strings.Contains(t, "/") {
		return path.Base(newNoExt) + ext
	}
	if path.Dir(r.oldID) != path.Dir(r.newID) {
		return newNoExt + ext
	}
	n := len(strings.Split(trimExt(t), "/"))
	segs := strings.Split(newNoExt, "/")
	if n > len(segs) {
		n = len(segs)
	}
	return strings.Join(segs[len(segs)-n:], "/") + ext
}

func (r *renamer) mdTarget(src, raw string) (string, bool) {
	if path.Join(path.Dir(src), strings.ToLower(raw)) == strings.ToLower(r.oldID) {
		return relPath(path.Dir(src), r.newID), true
	}
	if r.namesOld(raw) {
		return r.wikiTarget(raw), true
	}
	return "", false
}

func relPath(fromDir, to string) string {
	if fromDir == "." {
		return to
	}
	f, t := strings.Split(fromDir, "/"), strings.Split(to, "/")
	i := 0
	for i < len(f) && i < len(t)-1 && f[i] == t[i] {
		i++
	}
	parts := []string{}
	for range f[i:] {
		parts = append(parts, "..")
	}
	return strings.Join(append(parts, t[i:]...), "/")
}

// rewrite returns src with every link to the old note retargeted.
func (r *renamer) rewrite(srcID, src string) string {
	var sb strings.Builder
	fence := ""
	for _, line := range strings.SplitAfter(src, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			mark := trimmed[:3]
			switch {
			case fence == "":
				fence = mark
			case fence == mark:
				fence = ""
			}
			sb.WriteString(line)
			continue
		}
		if fence != "" {
			sb.WriteString(line)
			continue
		}
		spans := codeSpans(line)
		type edit struct {
			start, end int
			text       string
		}
		var edits []edit
		for _, m := range wikiRe.FindAllStringSubmatchIndex(line, -1) {
			if inSpan(spans, m[0]) {
				continue
			}
			raw := line[m[4]:m[5]]
			if !r.namesOld(raw) {
				continue
			}
			edits = append(edits, edit{m[4], m[5], r.wikiTarget(raw)})
		}
		for _, m := range mdRe.FindAllStringSubmatchIndex(line, -1) {
			if inSpan(spans, m[0]) {
				continue
			}
			raw := line[m[4]:m[5]]
			if nt, ok := r.mdTarget(srcID, raw); ok {
				edits = append(edits, edit{m[4], m[5], nt})
			}
		}
		sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
		pos := 0
		for _, e := range edits {
			sb.WriteString(line[pos:e.start])
			sb.WriteString(e.text)
			pos = e.end
		}
		sb.WriteString(line[pos:])
	}
	return sb.String()
}

func init() {
	ops.Handle("notes.rename", func(ctx context.Context, env *ops.Env, req RenameReq) (RenameResp, error) {
		if _, _, err := noteRow(ctx, env, req.ID); err != nil {
			return RenameResp{}, err
		}
		newName, err := newID(req.New, dirOf(req.ID), extOf(req.ID))
		if err != nil {
			return RenameResp{}, err
		}
		if newName == req.ID {
			return RenameResp{}, ops.Errorf(ops.CodeInvalid, "new name is the same as the old name")
		}
		if _, err := os.Lstat(abs(env, newName)); err == nil {
			return RenameResp{}, ops.Errorf(ops.CodeConflict, "%s already exists", newName)
		}
		rn := &renamer{oldID: req.ID, newID: newName, oldNoExt: strings.ToLower(trimExt(req.ID))}
		rn.oldStem = path.Base(rn.oldNoExt)

		srcs, err := column(ctx, env, "SELECT DISTINCT src FROM links WHERE dst_id=? ORDER BY src", req.ID)
		if err != nil {
			return RenameResp{}, err
		}
		type plan struct {
			id, path string
			data     []byte
			mtime    int64
		}
		var plans []plan
		for _, s := range srcs {
			p := abs(env, s)
			data, mt, err := readFile(p)
			if err != nil {
				return RenameResp{}, err
			}
			var indexed int64
			if err := env.Store.DB.QueryRowContext(ctx, "SELECT mtime_ns FROM notes WHERE id=?", s).Scan(&indexed); err != nil {
				return RenameResp{}, dbErr("lookup "+s, err)
			}
			if indexed != mt {
				return RenameResp{}, ops.Errorf(ops.CodeConflict, "%s was modified since it was indexed; run again after reindexing", s)
			}
			out := rn.rewrite(s, string(data))
			if out != string(data) {
				plans = append(plans, plan{s, p, []byte(out), mt})
			}
		}
		for _, pl := range plans {
			if err := writeChecked(pl.path, pl.data, pl.mtime); err != nil {
				return RenameResp{}, err
			}
		}
		if err := os.MkdirAll(path.Dir(abs(env, newName)), 0o755); err != nil {
			return RenameResp{}, ops.Errorf(ops.CodeInternal, "create dir: %v", err)
		}
		if err := os.Rename(abs(env, req.ID), abs(env, newName)); err != nil {
			return RenameResp{}, ops.Errorf(ops.CodeInternal, "move %s: %v", req.ID, err)
		}
		for _, t := range []string{"pins", "visits"} {
			if _, err := env.Store.DB.ExecContext(ctx, "UPDATE "+t+" SET note_id=? WHERE note_id=?", newName, req.ID); err != nil {
				return RenameResp{}, dbErr("move "+t, err)
			}
		}
		if err := reindex(env, newName); err != nil {
			return RenameResp{}, err
		}
		if err := env.Index.RemoveFile(req.ID); err != nil {
			return RenameResp{}, ops.Errorf(ops.CodeInternal, "unindex %s: %v", req.ID, err)
		}
		rewritten := []string{}
		for _, pl := range plans {
			id := pl.id
			if id == req.ID {
				rewritten = append(rewritten, newName)
				continue
			}
			if err := reindex(env, id); err != nil {
				return RenameResp{}, err
			}
			rewritten = append(rewritten, id)
		}
		return RenameResp{ID: newName, Path: abs(env, newName), Rewritten: rewritten}, nil
	})
}
