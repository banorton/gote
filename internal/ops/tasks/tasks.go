package tasks

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/query"
)

const searchLimit = 1000

type ListReq struct {
	All bool   `json:"all"`
	Q   string `json:"q"`
	// Archived includes tasks in archived notes (a tag:archive query does too).
	Archived bool `json:"archived"`
}

type Task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Line  int    `json:"line"`
	Text  string `json:"text"`
	Done  bool   `json:"done"`
}

func init() {
	ops.Handle("tasks.list", runList)
}

func matchingIDs(ctx context.Context, env *ops.Env, q string) (map[string]bool, error) {
	op, ok := ops.Lookup("search.query")
	if !ok {
		return nil, ops.Errorf(ops.CodeInternal, "tasks.list: search.query is not registered")
	}
	raw, err := json.Marshal(map[string]any{"q": q, "limit": searchLimit, "all": true})
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "tasks.list: %v", err)
	}
	res, err := op.Run(ctx, env, raw)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(res)
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "tasks.list: %v", err)
	}
	var resp struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "tasks.list: bad search.query response: %v", err)
	}
	ids := make(map[string]bool, len(resp.Results))
	for _, r := range resp.Results {
		ids[r.ID] = true
	}
	return ids, nil
}

func runList(ctx context.Context, env *ops.Env, req ListReq) ([]Task, error) {
	var ids map[string]bool
	if strings.TrimSpace(req.Q) != "" {
		var err error
		if ids, err = matchingIDs(ctx, env, req.Q); err != nil {
			return nil, err
		}
	}
	archived := req.Archived
	if strings.TrimSpace(req.Q) != "" {
		pq, err := query.Parse(req.Q)
		if err != nil {
			return nil, ops.Errorf(ops.CodeInvalid, "%v", err)
		}
		archived = archived || pq.MentionsArchive()
	}
	q := `SELECT t.note_id, n.title, t.line, t.text, t.done FROM tasks t JOIN notes n ON n.id = t.note_id WHERE ` + ops.Visible("n", archived)
	if !req.All {
		q += ` AND t.done = 0`
	}
	q += ` ORDER BY n.mtime_ns DESC, t.note_id, t.line`
	rows, err := env.Store.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "tasks.list: %v", err)
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		var done int
		if err := rows.Scan(&t.ID, &t.Title, &t.Line, &t.Text, &done); err != nil {
			return nil, ops.Errorf(ops.CodeInternal, "tasks.list: %v", err)
		}
		if ids != nil && !ids[t.ID] {
			continue
		}
		t.Done = done != 0
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, ops.Errorf(ops.CodeInternal, "tasks.list: %v", err)
	}
	return out, nil
}
