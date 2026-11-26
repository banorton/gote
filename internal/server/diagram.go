package server

import (
	"context"

	opdiagram "github.com/banorton/gote/internal/ops/diagram"
)

// startDiagramPNG keeps drawing PNGs current; the returned func stops it and
// must run before the store is released.
func (s *Server) startDiagramPNG(ctx context.Context) func() {
	return opdiagram.StartBackground(ctx, s.env, s.log.Printf)
}
