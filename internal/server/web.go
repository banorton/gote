package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/banorton/gote/internal/web"
)

// startWeb serves the web view next to the socket, sharing the single-writer
// mutex. SSE streams poll file stamps; once index.OnReconcile exists, call
// ws.Notify() from it here so streams wake on each reconcile.
func (s *Server) startWeb(ctx context.Context) (func(), error) {
	ws, err := web.Start(ctx, s.env, web.WithLock(&s.mu), web.WithStateDir(filepath.Dir(s.o.SocketPath)))
	if err != nil {
		return nil, fmt.Errorf("start web server: %w", err)
	}
	s.log.Printf("web view on %s", ws.Addr())
	return func() { ws.Close() }, nil
}
