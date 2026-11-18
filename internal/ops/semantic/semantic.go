package semantic

import (
	"context"

	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/ops/search"
	"github.com/banorton/gote/internal/semantic"
)

type SyncReq struct {
	Confirm bool `json:"confirm"`
}

type SyncResp struct {
	Chunks    int     `json:"chunks"`
	Embedded  int     `json:"embedded"`
	Pending   int     `json:"pending"`
	Tokens    int     `json:"tokens"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
	ElapsedMS float64 `json:"elapsed_ms"`
}

type StatusResp struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Chunks     int    `json:"chunks"`
	Embedded   int    `json:"embedded"`
	Pending    int    `json:"pending"`
}

func init() {
	search.SemanticHook = semantic.SearchKind
	ops.Handle("semantic.sync", func(ctx context.Context, env *ops.Env, req SyncReq) (SyncResp, error) {
		s, err := semantic.Sync(ctx, env, req.Confirm)
		if err != nil {
			return SyncResp{}, err
		}
		cost, _ := semantic.Cost(env.Cfg.Semantic, s.Tokens)
		return SyncResp{s.Chunks, s.Embedded, s.Pending, s.Tokens, cost, float64(s.Elapsed.Microseconds()) / 1000}, nil
	})
	ops.Handle("semantic.status", func(ctx context.Context, env *ops.Env, _ ops.Empty) (StatusResp, error) {
		s, err := semantic.Status(ctx, env)
		return StatusResp(s), err
	})
}
