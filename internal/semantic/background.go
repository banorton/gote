package semantic

import (
	"context"
	"time"

	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
)

// Debounce is how long index changes settle before a background sync runs.
var Debounce = 2 * time.Second

// StartBackground syncs embeddings after index changes until ctx ends or the
// returned stop func is called. Errors go to logf and never stop the loop.
// It is a no-op when semantic search is not configured.
func StartBackground(ctx context.Context, env *ops.Env, logf func(format string, args ...any)) (stop func()) {
	if !env.Cfg.Semantic.Enabled() {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	trigger := make(chan struct{}, 1)
	unsub := env.Index.OnReconcile(func(index.Stats) {
		select {
		case trigger <- struct{}{}:
		default:
		}
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
			}
			timer := time.NewTimer(Debounce)
		settle:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-trigger:
					timer.Reset(Debounce)
				case <-timer.C:
					break settle
				}
			}
			st, err := Sync(ctx, env, false)
			switch {
			case ctx.Err() != nil:
				return
			case err != nil:
				logf("semantic sync: %v", err)
			case st.Embedded > 0:
				logf("semantic sync: embedded %d chunks (~%d tokens)", st.Embedded, st.Tokens)
			}
		}
	}()
	return func() {
		unsub()
		cancel()
		<-done
	}
}
