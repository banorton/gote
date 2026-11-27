package notes

import (
	"context"

	"github.com/banorton/gote/internal/ops"
)

type PinsListReq struct {
	All bool `json:"all"`
}

func init() {
	ops.Handle("pins.add", func(ctx context.Context, env *ops.Env, r IDReq) (ops.Empty, error) {
		if _, _, err := noteRow(ctx, env, r.ID); err != nil {
			return ops.Empty{}, err
		}
		_, err := env.Store.DB.ExecContext(ctx, "INSERT OR IGNORE INTO pins VALUES(?,?)", r.ID, env.Now().UnixNano())
		return ops.Empty{}, dbErr("pin", err)
	})
	ops.Handle("pins.remove", func(ctx context.Context, env *ops.Env, r IDReq) (ops.Empty, error) {
		_, err := env.Store.DB.ExecContext(ctx, "DELETE FROM pins WHERE note_id=?", r.ID)
		return ops.Empty{}, dbErr("unpin", err)
	})
	ops.Handle("pins.list", func(ctx context.Context, env *ops.Env, r PinsListReq) ([]Ref, error) {
		return queryRefs(ctx, env, `SELECT n.id, n.title, n.kind FROM pins p JOIN notes n ON n.id=p.note_id
			WHERE `+ops.Visible("n", r.All)+` ORDER BY p.pinned_at DESC, n.id`)
	})
}
