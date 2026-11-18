package search

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/banorton/gote/internal/fuzzy"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/query"
)

const defaultLimit = 50

type Result struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Kind    string  `json:"kind"`
	Path    string  `json:"path"`
	Snippet string  `json:"snippet"`
	Line    int     `json:"line"`
	Score   float64 `json:"score"`
}

type QueryReq struct {
	Q        string `json:"q"`
	Limit    int    `json:"limit"`
	Semantic bool   `json:"semantic"`
	// Kind restricts results to "note" or "diagram"; empty means both.
	Kind string `json:"kind"`
}

type QueryResp struct {
	Results []Result `json:"results"`
}

// SemanticHook is set by the semantic search phase; nil means not configured.
// kind is "" or a note kind to restrict results to.
var SemanticHook func(ctx context.Context, env *ops.Env, q string, limit int, kind string) ([]Result, error)

func checkKind(kind string) error {
	if kind != "" && kind != "note" && kind != "diagram" {
		return ops.Errorf(ops.CodeInvalid, "kind must be note or diagram, got %q", kind)
	}
	return nil
}

func init() {
	ops.Handle("search.query", runQuery)
	ops.Handle("search.fuzzy", runFuzzy)
	ops.Handle("search.corpus", runCorpus)
	ops.Handle("search.lines", runLines)
	ops.Handle("search.preview", runPreview)
}

func runQuery(ctx context.Context, env *ops.Env, req QueryReq) (QueryResp, error) {
	if req.Limit < 0 {
		return QueryResp{}, ops.Errorf(ops.CodeInvalid, "limit must be >= 0")
	}
	if err := checkKind(req.Kind); err != nil {
		return QueryResp{}, err
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if req.Semantic {
		if SemanticHook == nil {
			return QueryResp{}, ops.Errorf(ops.CodeInvalid, "semantic search not configured")
		}
		rs, err := SemanticHook(ctx, env, req.Q, limit, req.Kind)
		if err != nil {
			return QueryResp{}, err
		}
		return QueryResp{Results: nonNil(rs)}, nil
	}
	q, err := query.Parse(req.Q)
	if err != nil {
		return QueryResp{}, ops.Errorf(ops.CodeInvalid, "%v", err)
	}
	if req.Kind != "" {
		q.Filters = append(q.Filters, query.Filter{Kind: query.KindNoteKind, Value: req.Kind})
	}
	sqlText, args := buildSQL(q, limit)
	rows, err := env.Store.DB.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return QueryResp{}, ops.Errorf(ops.CodeInternal, "search: %v", err)
	}
	defer rows.Close()
	var out []Result
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.ID, &r.Title, &r.Kind, &r.Snippet, &r.Score); err != nil {
			return QueryResp{}, ops.Errorf(ops.CodeInternal, "search: %v", err)
		}
		r.Path = filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(r.ID))
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return QueryResp{}, ops.Errorf(ops.CodeInternal, "search: %v", err)
	}
	var terms []string
	for _, t := range q.Terms {
		if !t.Neg {
			terms = append(terms, strings.ToLower(t.Text))
		}
	}
	if len(terms) > 0 {
		for i := range out {
			out[i].Line = firstMatchLine(out[i].Path, terms)
		}
	}
	return QueryResp{Results: nonNil(out)}, nil
}

func nonNil(rs []Result) []Result {
	if rs == nil {
		return []Result{}
	}
	return rs
}

func buildSQL(q query.Query, limit int) (string, []any) {
	var pos, neg []string
	for _, t := range q.Terms {
		if t.Neg {
			neg = append(neg, query.Quote(t.Text))
		} else {
			pos = append(pos, query.Quote(t.Text))
		}
	}
	var where []string
	var wargs []any
	for _, f := range q.Filters {
		clause, a := filterSQL(f)
		where = append(where, clause)
		wargs = append(wargs, a...)
	}
	for _, n := range neg {
		where = append(where, "n.rowid NOT IN (SELECT rowid FROM notes_fts WHERE notes_fts MATCH ?)")
		wargs = append(wargs, n)
	}
	var sb strings.Builder
	var args []any
	if len(pos) > 0 {
		sb.WriteString(`SELECT n.id, n.title, n.kind, snippet(notes_fts, 4, '', '', '…', 12), -bm25(notes_fts, 0, 10, 5, 3, 1)
FROM notes_fts JOIN notes n ON n.rowid = notes_fts.rowid WHERE notes_fts MATCH ?`)
		args = append(args, strings.Join(pos, " "))
	} else {
		sb.WriteString(`SELECT n.id, n.title, n.kind, '', 0 FROM notes n WHERE 1=1`)
	}
	for _, w := range where {
		sb.WriteString(" AND " + w)
	}
	args = append(args, wargs...)
	if len(pos) > 0 {
		sb.WriteString(" ORDER BY bm25(notes_fts, 0, 10, 5, 3, 1), n.id")
	} else {
		sb.WriteString(" ORDER BY n.mtime_ns DESC, n.id")
	}
	sb.WriteString(" LIMIT ?")
	args = append(args, limit)
	return sb.String(), args
}

func filterSQL(f query.Filter) (string, []any) {
	var clause string
	var args []any
	switch f.Kind {
	case query.KindTag:
		clause = "EXISTS (SELECT 1 FROM tags t WHERE t.note_id = n.id AND lower(t.tag) = lower(?))"
		args = []any{f.Value}
	case query.KindNoteKind:
		clause = "n.kind = ?"
		args = []any{f.Value}
	case query.KindCreated:
		lo, hi := f.Range.CreatedBounds()
		var parts []string
		if lo != "" {
			parts = append(parts, "n.created >= ?")
			args = append(args, lo)
		}
		if hi != "" {
			parts = append(parts, "n.created < ?")
			args = append(args, hi)
		}
		clause = "(n.created IS NOT NULL AND " + strings.Join(parts, " AND ") + ")"
	case query.KindModified:
		var parts []string
		if f.Range.HasLo {
			parts = append(parts, "n.mtime_ns >= ?")
			args = append(args, f.Range.Lo.UnixNano())
		}
		if f.Range.HasHi {
			parts = append(parts, "n.mtime_ns < ?")
			args = append(args, f.Range.Hi.UnixNano())
		}
		clause = "(" + strings.Join(parts, " AND ") + ")"
	default:
		clause = fmt.Sprintf(`EXISTS (SELECT 1 FROM json_each(n.fields, '$."%s"') j WHERE lower(CAST(j.value AS TEXT)) = lower(?) OR (j.type IN ('true','false') AND j.type = lower(?)))`, f.Field)
		args = []any{f.Value, f.Value}
	}
	if f.Neg {
		clause = "NOT " + clause
	}
	return clause, args
}

func bodyStart(lines []string) int {
	if len(lines) == 0 || lines[0] != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return i + 1
		}
	}
	return 0
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	return strings.Split(text, "\n")
}

func firstMatchLine(path string, terms []string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	lines := splitLines(string(data))
	for i := bodyStart(lines); i < len(lines); i++ {
		l := strings.ToLower(lines[i])
		for _, t := range terms {
			if strings.Contains(l, t) {
				return i + 1
			}
		}
	}
	return 0
}

type CorpusItem struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Kind    string   `json:"kind"`
	Path    string   `json:"path"`
	Aliases []string `json:"aliases"`
}

// SearchString is the text the fuzzy matcher sees for one note.
func SearchString(it CorpusItem) string {
	s := it.Title + "  " + it.Path
	if len(it.Aliases) > 0 {
		s += "  " + strings.Join(it.Aliases, " ")
	}
	return s
}

type CorpusResp struct {
	Items []CorpusItem `json:"items"`
}

func runCorpus(ctx context.Context, env *ops.Env, _ ops.Empty) (CorpusResp, error) {
	return loadCorpus(ctx, env)
}

func loadCorpus(ctx context.Context, env *ops.Env) (CorpusResp, error) {
	rows, err := env.Store.DB.QueryContext(ctx, `SELECT n.id, n.title, n.kind,
		COALESCE((SELECT group_concat(alias, char(31)) FROM aliases a WHERE a.note_id = n.id), '')
		FROM notes n ORDER BY n.title COLLATE NOCASE, n.id`)
	if err != nil {
		return CorpusResp{}, ops.Errorf(ops.CodeInternal, "corpus: %v", err)
	}
	defer rows.Close()
	out := CorpusResp{Items: []CorpusItem{}}
	for rows.Next() {
		var it CorpusItem
		var al string
		if err := rows.Scan(&it.ID, &it.Title, &it.Kind, &al); err != nil {
			return CorpusResp{}, ops.Errorf(ops.CodeInternal, "corpus: %v", err)
		}
		it.Path = it.ID
		it.Aliases = []string{}
		if al != "" {
			it.Aliases = strings.Split(al, "\x1f")
		}
		out.Items = append(out.Items, it)
	}
	return out, rows.Err()
}

type Line struct {
	ID   string `json:"id"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type LinesResp struct {
	Lines []Line `json:"lines"`
}

func noteIDs(ctx context.Context, env *ops.Env) ([]string, error) {
	rows, err := env.Store.DB.QueryContext(ctx, "SELECT id FROM notes WHERE kind = 'note' ORDER BY id")
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "%v", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, ops.Errorf(ops.CodeInternal, "%v", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func runLines(ctx context.Context, env *ops.Env, _ ops.Empty) (LinesResp, error) {
	ids, err := noteIDs(ctx, env)
	if err != nil {
		return LinesResp{}, err
	}
	out := LinesResp{Lines: []Line{}}
	for _, id := range ids {
		data, err := os.ReadFile(filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id)))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return LinesResp{}, ops.Errorf(ops.CodeInternal, "read %s: %v", id, err)
		}
		lines := splitLines(string(data))
		for i := bodyStart(lines); i < len(lines); i++ {
			if t := strings.TrimSpace(lines[i]); t != "" {
				out.Lines = append(out.Lines, Line{ID: id, Line: i + 1, Text: t})
			}
		}
	}
	return out, nil
}

const previewSize = 60

type PreviewReq struct {
	ID   string `json:"id"`
	Line int    `json:"line"`
}

type PreviewResp struct {
	ID    string   `json:"id"`
	Start int      `json:"start"`
	Lines []string `json:"lines"`
}

func runPreview(ctx context.Context, env *ops.Env, req PreviewReq) (PreviewResp, error) {
	var id string
	err := env.Store.DB.QueryRowContext(ctx, "SELECT id FROM notes WHERE id = ?", req.ID).Scan(&id)
	if err != nil {
		return PreviewResp{}, ops.Errorf(ops.CodeNotFound, "note %q not found", req.ID)
	}
	data, err := os.ReadFile(filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id)))
	if err != nil {
		return PreviewResp{}, ops.Errorf(ops.CodeNotFound, "read %s: %v", id, err)
	}
	lines := splitLines(string(data))
	start := 1
	if req.Line > 0 {
		start = req.Line - previewSize/2
	}
	if start+previewSize-1 > len(lines) {
		start = len(lines) - previewSize + 1
	}
	if start < 1 {
		start = 1
	}
	end := start + previewSize - 1
	if end > len(lines) {
		end = len(lines)
	}
	return PreviewResp{ID: id, Start: start, Lines: lines[start-1 : end]}, nil
}

type FuzzyReq struct {
	Q     string `json:"q"`
	Limit int    `json:"limit"`
	// Kind restricts matches to "note" or "diagram"; empty means both.
	Kind string `json:"kind"`
}

type FuzzyResult struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Path  string  `json:"path"`
	Score float64 `json:"score"`
}

type FuzzyResp struct {
	Results []FuzzyResult `json:"results"`
}

// runFuzzy matches the same corpus the finder uses (title, path, aliases).
// Filter syntax is full-text only, so it is rejected rather than guessed at.
func runFuzzy(ctx context.Context, env *ops.Env, req FuzzyReq) (FuzzyResp, error) {
	if req.Limit < 0 {
		return FuzzyResp{}, ops.Errorf(ops.CodeInvalid, "limit must be >= 0")
	}
	if err := checkKind(req.Kind); err != nil {
		return FuzzyResp{}, err
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	if strings.TrimSpace(req.Q) == "" {
		return FuzzyResp{}, ops.Errorf(ops.CodeInvalid, "empty query")
	}
	q, err := query.Parse(req.Q)
	if err != nil {
		return FuzzyResp{}, ops.Errorf(ops.CodeInvalid, "%v (filters and negation need full-text search: use -t)", err)
	}
	for _, t := range q.Terms {
		if t.Neg || t.Phrase {
			return FuzzyResp{}, ops.Errorf(ops.CodeInvalid, "negation and quoted phrases need full-text search: use -t")
		}
	}
	if len(q.Filters) > 0 {
		return FuzzyResp{}, ops.Errorf(ops.CodeInvalid, "filters (tag:, kind:, created:, field:value) need full-text search: use -t")
	}
	corpus, err := loadCorpus(ctx, env)
	if err != nil {
		return FuzzyResp{}, err
	}
	if req.Kind != "" {
		kept := corpus.Items[:0]
		for _, it := range corpus.Items {
			if it.Kind == req.Kind {
				kept = append(kept, it)
			}
		}
		corpus.Items = kept
	}
	strs := make([]string, len(corpus.Items))
	for i, it := range corpus.Items {
		strs[i] = SearchString(it)
	}
	out := FuzzyResp{Results: []FuzzyResult{}}
	for _, m := range fuzzy.Match(req.Q, strs) {
		if len(out.Results) == limit {
			break
		}
		it := corpus.Items[m.Index]
		out.Results = append(out.Results, FuzzyResult{
			ID: it.ID, Title: it.Title, Kind: it.Kind,
			Path:  filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(it.ID)),
			Score: float64(m.Score),
		})
	}
	return out, nil
}
