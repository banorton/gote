package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/paths"
	"github.com/banorton/gote/internal/store"
)

// Version is overridden at build time with -ldflags -X.
var Version = "dev"

type Command struct {
	Name, Summary, Usage string
	Aliases              []string
	SkipReconcile        bool
	// NoConfig skips config loading and the index; Ctx.Client and Ctx.Cfg are nil.
	NoConfig bool
	Run      func(c *Ctx, args []string) error
}

type Ctx struct {
	Client   client.Client
	Cfg      *config.Config
	Out, Err io.Writer
	JSON     bool
	NoServer bool
}

// UsageError makes Main exit 2.
type UsageError struct {
	Msg   string
	quiet bool
}

func (e *UsageError) Error() string { return e.Msg }

func Usagef(format string, args ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, args...)}
}

var errHelp = errors.New("help requested")

func (c *Ctx) FlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.Err)
	return fs
}

// ParseFlags maps flag errors to a usage error; -h is a clean exit.
func ParseFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelp
		}
		return &UsageError{Msg: err.Error(), quiet: true}
	}
	return nil
}

func (c *Ctx) PrintJSON(v any) error {
	enc := json.NewEncoder(c.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type registry struct {
	byName map[string]*Command
	cmds   []*Command
}

func newRegistry() *registry { return &registry{byName: map[string]*Command{}} }

var std = newRegistry()

// Register adds a command; call from init(). Panics on name or alias collision.
func Register(cmd Command) { std.register(cmd) }

func (r *registry) register(cmd Command) {
	if cmd.Name == "" || cmd.Run == nil {
		panic("cli: command needs a name and Run")
	}
	c := cmd
	for _, n := range append([]string{cmd.Name}, cmd.Aliases...) {
		if prev, dup := r.byName[n]; dup {
			panic(fmt.Sprintf("cli: %q (command %s) collides with command %s", n, cmd.Name, prev.Name))
		}
		r.byName[n] = &c
	}
	r.cmds = append(r.cmds, &c)
}

func Main(argv []string) int { return std.run(argv, os.Stdout, os.Stderr) }

func (r *registry) run(args []string, out, errw io.Writer) int {
	var rest []string
	jsonOut, noServer, leading := false, false, true
	for i, a := range args {
		if a == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--no-server":
			noServer = true
		case leading && a == "--version":
			fmt.Fprintln(out, "gote "+Version)
			return 0
		case leading && (a == "--help" || a == "-h"):
			r.printHelp(out)
			return 0
		default:
			leading = false
			rest = append(rest, a)
		}
	}

	c := &Ctx{Out: out, Err: errw, JSON: jsonOut, NoServer: noServer}
	err := r.dispatch(c, rest)
	switch {
	case err == nil, errors.Is(err, errHelp):
		return 0
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		if !ue.quiet {
			fmt.Fprintf(errw, "gote: %s\nrun `gote help` for usage\n", ue.Msg)
		}
		return 2
	}
	if jsonOut {
		var oe *ops.Error
		if !errors.As(err, &oe) {
			oe = &ops.Error{Code: ops.CodeInternal, Msg: err.Error()}
		}
		json.NewEncoder(errw).Encode(map[string]any{"error": oe})
	} else {
		fmt.Fprintf(errw, "gote: %v\n", err)
	}
	return 1
}

func (r *registry) dispatch(c *Ctx, rest []string) error {
	var cmd *Command
	var args []string
	switch {
	case len(rest) == 0:
		cmd = r.byName["quick"]
		if cmd == nil {
			cmd = r.byName["help"]
		}
		if cmd == nil {
			return Usagef("no command given")
		}
	case r.byName[rest[0]] != nil:
		cmd, args = r.byName[rest[0]], rest[1:]
	case r.byName["open"] != nil:
		cmd, args = r.byName["open"], rest
	default:
		return Usagef("unknown command %q", rest[0])
	}

	if !cmd.NoConfig {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		closeClient, err := c.connect(cfg)
		if err != nil {
			return err
		}
		defer closeClient()
		if !cmd.SkipReconcile {
			if err := c.Client.Call(context.Background(), "index.reconcile", nil, nil); err != nil {
				return err
			}
		}
	}
	return cmd.Run(c, args)
}

// spawnServer builds the function that launches a detached server; tests replace it.
var spawnServer = client.SpawnServer

// connect sets c.Cfg and c.Client: a server client when enabled and reachable,
// otherwise an in-process one.
func (c *Ctx) connect(cfg *config.Config) (func(), error) {
	c.Cfg = cfg
	if !c.NoServer && cfg.Server {
		sock, err := paths.SocketPath()
		if err != nil {
			return nil, err
		}
		state, _ := paths.StateDir()
		cl, err := client.Connect(client.ConnectOpts{
			Socket: sock, Version: Version,
			Spawn: spawnServer(filepath.Join(state, "server.log")),
		})
		if err == nil {
			c.Client = cl
			return func() {}, nil
		}
		fmt.Fprintf(c.Err, "gote: server unavailable (%v); running in-process\n", err)
	}
	dbPath, err := paths.DBPath()
	if err != nil {
		return nil, err
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	env := &ops.Env{Cfg: cfg, Store: st, Index: index.New(st, cfg.NotesDir), Now: time.Now}
	c.Client = client.Local(env)
	return func() { st.Close() }, nil
}

func (r *registry) sorted() []*Command {
	out := append([]*Command(nil), r.cmds...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
