package server

import (
	"context"

	"github.com/banorton/gote/internal/semantic"
)

// startSemantic runs background embedding sync; the returned func stops it and
// must run before the store is released.
func (s *Server) startSemantic(ctx context.Context) func() {
	return semantic.StartBackground(ctx, s.env, s.log.Printf)
}
