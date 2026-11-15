package main

// Each package self-registers its ops or commands from init(). Phases fill the
// package directories; this file never needs to change.
import (
	_ "github.com/banorton/gote/internal/cli/diagram"
	_ "github.com/banorton/gote/internal/cli/graph"
	_ "github.com/banorton/gote/internal/cli/notes"
	_ "github.com/banorton/gote/internal/cli/search"
	_ "github.com/banorton/gote/internal/cli/semantic"
	_ "github.com/banorton/gote/internal/cli/tasks"
	_ "github.com/banorton/gote/internal/cli/view"
	_ "github.com/banorton/gote/internal/ops/diagram"
	_ "github.com/banorton/gote/internal/ops/graph"
	_ "github.com/banorton/gote/internal/ops/notes"
	_ "github.com/banorton/gote/internal/ops/search"
	_ "github.com/banorton/gote/internal/ops/semantic"
	_ "github.com/banorton/gote/internal/server"
)
