package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/paths"
	"github.com/banorton/gote/internal/store"
)

func init() {
	cli.Register(cli.Command{
		Name:     "serve",
		Summary:  "run the gote server in the foreground (normally auto-spawned)",
		Usage:    "gote serve",
		NoConfig: true,
		Run:      runServe,
	})
	cli.Register(cli.Command{
		Name:     "server",
		Summary:  "manage the background server: status | stop",
		Usage:    "gote server status|stop",
		NoConfig: true,
		Run:      runServerCmd,
	})
}

func runServe(c *cli.Ctx, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	state, err := paths.StateDir()
	if err != nil {
		return err
	}
	sock, err := paths.SocketPath()
	if err != nil {
		return err
	}
	dbPath, err := paths.DBPath()
	if err != nil {
		return err
	}
	s := New(Options{
		Version:    cli.Version,
		SocketPath: sock,
		LockPath:   filepath.Join(state, "server.lock"),
		LogPath:    filepath.Join(state, "server.log"),
		Open: func() (*ops.Env, func(), error) {
			st, err := store.Open(dbPath)
			if err != nil {
				return nil, nil, err
			}
			return &ops.Env{Cfg: cfg, Store: st, Index: index.New(st, cfg.NotesDir), Now: time.Now}, func() { st.Close() }, nil
		},
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := s.Run(ctx); err != nil && !errors.Is(err, ErrAlreadyRunning) {
		return err
	}
	return nil
}

func runServerCmd(c *cli.Ctx, args []string) error {
	if len(args) != 1 || (args[0] != "status" && args[0] != "stop") {
		return cli.Usagef("usage: gote server status|stop")
	}
	sock, err := paths.SocketPath()
	if err != nil {
		return err
	}
	info, probeErr := client.Probe(sock)
	if args[0] == "status" {
		if c.JSON {
			if probeErr != nil {
				return c.PrintJSON(map[string]any{"running": false, "socket": sock})
			}
			return c.PrintJSON(map[string]any{"running": true, "version": info.Version, "pid": info.PID, "started": info.Started, "socket": sock})
		}
		if probeErr != nil {
			fmt.Fprintln(c.Out, "server not running")
			return nil
		}
		fmt.Fprintf(c.Out, "server running: version %s, pid %d, started %s\nsocket: %s\n", info.Version, info.PID, info.Started, sock)
		return nil
	}
	if probeErr != nil {
		if c.JSON {
			return c.PrintJSON(map[string]any{"stopped": false, "running": false})
		}
		fmt.Fprintln(c.Out, "server not running")
		return nil
	}
	if err := client.Shutdown(sock); err != nil {
		return fmt.Errorf("stop server: %w", err)
	}
	if err := client.WaitGone(sock, 3*time.Second); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(map[string]any{"stopped": true})
	}
	fmt.Fprintln(c.Out, "server stopped")
	return nil
}
