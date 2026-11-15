package ops

import (
	"context"

	"github.com/banorton/gote/internal/index"
)

type Empty struct{}

type StatsResp struct {
	Scanned   int     `json:"scanned"`
	Added     int     `json:"added"`
	Updated   int     `json:"updated"`
	Touched   int     `json:"touched"`
	Removed   int     `json:"removed"`
	ElapsedMS float64 `json:"elapsed_ms"`
}

func statsResp(s index.Stats) StatsResp {
	return StatsResp{s.Scanned, s.Added, s.Updated, s.Touched, s.Removed, float64(s.Elapsed.Microseconds()) / 1000}
}

type StatusResp struct {
	Notes      int       `json:"notes"`
	Links      int       `json:"links"`
	Unresolved int       `json:"unresolved"`
	DB         string    `json:"db"`
	Last       StatsResp `json:"last_reconcile"`
}

func init() {
	Handle("index.reconcile", func(ctx context.Context, env *Env, _ Empty) (StatsResp, error) {
		s, err := env.Index.Reconcile(ctx)
		if err != nil {
			return StatsResp{}, Errorf(CodeInternal, "reconcile: %v", err)
		}
		return statsResp(s), nil
	})
	Handle("index.rebuild", func(ctx context.Context, env *Env, _ Empty) (StatsResp, error) {
		if err := env.Index.Rebuild(ctx); err != nil {
			return StatsResp{}, Errorf(CodeInternal, "rebuild: %v", err)
		}
		return statsResp(env.Index.LastStats()), nil
	})
	Handle("index.status", func(ctx context.Context, env *Env, _ Empty) (StatusResp, error) {
		r := StatusResp{DB: env.Store.Path, Last: statsResp(env.Index.LastStats())}
		err := env.Store.DB.QueryRowContext(ctx, `SELECT
			(SELECT count(*) FROM notes), (SELECT count(*) FROM links),
			(SELECT count(*) FROM links WHERE dst_id IS NULL)`).Scan(&r.Notes, &r.Links, &r.Unresolved)
		if err != nil {
			return StatusResp{}, Errorf(CodeInternal, "status: %v", err)
		}
		return r, nil
	})
}
