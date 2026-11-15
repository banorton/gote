package graph

import (
	"context"
	"database/sql"

	"github.com/banorton/gote/internal/ops"
)

const maxDepth = 5

type IDReq struct {
	ID string `json:"id"`
}

type NeighborsReq struct {
	ID    string `json:"id"`
	Depth int    `json:"depth"`
}

type LinkOut struct {
	ID    *string `json:"id"`
	Raw   string  `json:"raw"`
	Title string  `json:"title"`
	Kind  string  `json:"kind"`
	Line  int     `json:"line"`
}

type BackOut struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
	Line  int    `json:"line"`
}

type NoteRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Kind  string `json:"kind"`
}

type Broken struct {
	Src  string `json:"src"`
	Raw  string `json:"raw"`
	Line int    `json:"line"`
}

type Node struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Dir      string `json:"dir,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Seen     bool   `json:"seen,omitempty"`
	Children []Node `json:"children"`
}

func internal(op string, err error) error {
	return ops.Errorf(ops.CodeInternal, "%s: %v", op, err)
}

func title(ctx context.Context, env *ops.Env, op, id string) (string, error) {
	var t string
	err := env.Store.DB.QueryRowContext(ctx, "SELECT title FROM notes WHERE id=?", id).Scan(&t)
	if err == sql.ErrNoRows {
		return "", ops.Errorf(ops.CodeNotFound, "note %q not found", id)
	}
	if err != nil {
		return "", internal(op, err)
	}
	return t, nil
}

func init() {
	ops.Handle("graph.links", func(ctx context.Context, env *ops.Env, r IDReq) ([]LinkOut, error) {
		if _, err := title(ctx, env, "graph.links", r.ID); err != nil {
			return nil, err
		}
		rows, err := env.Store.DB.QueryContext(ctx, `SELECT l.dst_id, l.dst_raw, COALESCE(n.title, l.dst_raw), l.kind, l.line
			FROM links l LEFT JOIN notes n ON n.id = l.dst_id WHERE l.src=? ORDER BY l.rowid`, r.ID)
		if err != nil {
			return nil, internal("graph.links", err)
		}
		defer rows.Close()
		out := []LinkOut{}
		for rows.Next() {
			var l LinkOut
			if err := rows.Scan(&l.ID, &l.Raw, &l.Title, &l.Kind, &l.Line); err != nil {
				return nil, internal("graph.links", err)
			}
			out = append(out, l)
		}
		return out, rows.Err()
	})

	ops.Handle("graph.backlinks", func(ctx context.Context, env *ops.Env, r IDReq) ([]BackOut, error) {
		if _, err := title(ctx, env, "graph.backlinks", r.ID); err != nil {
			return nil, err
		}
		rows, err := env.Store.DB.QueryContext(ctx, `SELECT l.src, n.title, l.kind, l.line
			FROM links l JOIN notes n ON n.id = l.src WHERE l.dst_id=? ORDER BY l.src, l.line, l.rowid`, r.ID)
		if err != nil {
			return nil, internal("graph.backlinks", err)
		}
		defer rows.Close()
		out := []BackOut{}
		for rows.Next() {
			var b BackOut
			if err := rows.Scan(&b.ID, &b.Title, &b.Kind, &b.Line); err != nil {
				return nil, internal("graph.backlinks", err)
			}
			out = append(out, b)
		}
		return out, rows.Err()
	})

	ops.Handle("graph.orphans", func(ctx context.Context, env *ops.Env, _ ops.Empty) ([]NoteRef, error) {
		rows, err := env.Store.DB.QueryContext(ctx, `SELECT id, title, kind FROM notes n WHERE kind='note'
			AND NOT EXISTS (SELECT 1 FROM links l WHERE l.src=n.id AND l.dst_id IS NOT NULL AND l.dst_id<>n.id)
			AND NOT EXISTS (SELECT 1 FROM links l WHERE l.dst_id=n.id AND l.src<>n.id)
			ORDER BY id`)
		if err != nil {
			return nil, internal("graph.orphans", err)
		}
		defer rows.Close()
		out := []NoteRef{}
		for rows.Next() {
			var n NoteRef
			if err := rows.Scan(&n.ID, &n.Title, &n.Kind); err != nil {
				return nil, internal("graph.orphans", err)
			}
			out = append(out, n)
		}
		return out, rows.Err()
	})

	ops.Handle("graph.broken", func(ctx context.Context, env *ops.Env, _ ops.Empty) ([]Broken, error) {
		rows, err := env.Store.DB.QueryContext(ctx,
			"SELECT src, dst_raw, line FROM links WHERE dst_id IS NULL AND attach=0 ORDER BY src, line, rowid")
		if err != nil {
			return nil, internal("graph.broken", err)
		}
		defer rows.Close()
		out := []Broken{}
		for rows.Next() {
			var b Broken
			if err := rows.Scan(&b.Src, &b.Raw, &b.Line); err != nil {
				return nil, internal("graph.broken", err)
			}
			out = append(out, b)
		}
		return out, rows.Err()
	})

	ops.Handle("graph.neighbors", neighbors)
}

type edge struct {
	id, title, dir, kind string
}

// adjacent lists distinct neighbors of id, outgoing first (by link order),
// then incoming-only (by source id). Self-links and unresolved links are dropped.
func adjacent(ctx context.Context, env *ops.Env, id string) ([]edge, error) {
	var out []edge
	pos := map[string]int{}
	add := func(e edge) {
		if _, ok := pos[e.id]; ok {
			return
		}
		pos[e.id] = len(out)
		out = append(out, e)
	}
	q := func(query string, dir string) error {
		rows, err := env.Store.DB.QueryContext(ctx, query, id, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			e := edge{dir: dir}
			if err := rows.Scan(&e.id, &e.title, &e.kind); err != nil {
				return err
			}
			add(e)
		}
		return rows.Err()
	}
	if err := q(`SELECT n.id, n.title, l.kind FROM links l JOIN notes n ON n.id=l.dst_id
		WHERE l.src=? AND l.dst_id<>? ORDER BY l.rowid`, "out"); err != nil {
		return nil, err
	}
	if err := q(`SELECT n.id, n.title, l.kind FROM links l JOIN notes n ON n.id=l.src
		WHERE l.dst_id=? AND l.src<>? ORDER BY l.src, l.line, l.rowid`, "in"); err != nil {
		return nil, err
	}
	return out, nil
}

func neighbors(ctx context.Context, env *ops.Env, r NeighborsReq) (Node, error) {
	depth := r.Depth
	if depth == 0 {
		depth = 1
	}
	if depth < 1 || depth > maxDepth {
		return Node{}, ops.Errorf(ops.CodeInvalid, "depth must be 1-%d, got %d", maxDepth, r.Depth)
	}
	rootTitle, err := title(ctx, env, "graph.neighbors", r.ID)
	if err != nil {
		return Node{}, err
	}

	// BFS over a flat arena, then link children by index; the tree is assembled after.
	type item struct {
		node     Node
		parent   string
		level    int
		children []int
	}
	items := []item{{node: Node{ID: r.ID, Title: rootTitle}}}
	seen := map[string]bool{r.ID: true}
	for i := 0; i < len(items); i++ {
		it := items[i]
		if it.level >= depth {
			continue
		}
		adj, err := adjacent(ctx, env, it.node.ID)
		if err != nil {
			return Node{}, internal("graph.neighbors", err)
		}
		for _, e := range adj {
			if e.id == it.parent {
				continue
			}
			child := item{node: Node{ID: e.id, Title: e.title, Dir: e.dir, Kind: e.kind}, parent: it.node.ID, level: it.level + 1}
			if seen[e.id] {
				child.node.Seen = true
				child.level = depth
			}
			seen[e.id] = true
			items = append(items, child)
			items[i].children = append(items[i].children, len(items)-1)
		}
	}
	var build func(i int) Node
	build = func(i int) Node {
		n := items[i].node
		n.Children = make([]Node, 0, len(items[i].children))
		for _, c := range items[i].children {
			n.Children = append(n.Children, build(c))
		}
		return n
	}
	return build(0), nil
}
