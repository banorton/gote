package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/web"
)

// startWeb serves the web view next to the socket, sharing the single-writer
// mutex. Reconciles wake SSE streams immediately instead of waiting for the poll.
func (s *Server) startWeb(ctx context.Context) (func(), error) {
	ws, err := web.Start(ctx, s.env, web.WithLock(&s.mu), web.WithStateDir(filepath.Dir(s.o.SocketPath)))
	if err != nil {
		return nil, fmt.Errorf("start web server: %w", err)
	}
	s.log.Printf("web view on %s", ws.Addr())
	unhook := s.env.Index.OnReconcile(func(index.Stats) { ws.Notify() })
	return func() { unhook(); ws.Close() }, nil
}
