package server

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

type tdir struct{ notes, state string }

func newDirs(t *testing.T) tdir {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "gt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	if root, err = filepath.EvalSymlinks(root); err != nil {
		t.Fatal(err)
	}
	d := tdir{filepath.Join(root, "notes"), filepath.Join(root, "s")}
	for _, p := range []string{d.notes, d.state} {
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func (d tdir) opts(version string) Options {
	return Options{
		Version:    version,
		SocketPath: filepath.Join(d.state, "gote.sock"),
		LockPath:   filepath.Join(d.state, "server.lock"),
		LogPath:    filepath.Join(d.state, "server.log"),
		Debounce:   20 * time.Millisecond,
		Open:       d.open,
	}
}

func (d tdir) open() (*ops.Env, func(), error) {
	st, err := store.Open(filepath.Join(d.state, "gote.db"))
	if err != nil {
		return nil, nil, err
	}
	env := &ops.Env{Cfg: &config.Config{NotesDir: d.notes}, Store: st, Index: index.New(st, d.notes), Now: time.Now}
	return env, func() { st.Close() }, nil
}

type running struct {
	s      *Server
	cancel context.CancelFunc
	done   chan error
}

func start(t *testing.T, o Options) *running {
	t.Helper()
	s := New(o)
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{s, cancel, make(chan error, 1)}
	go func() { r.done <- s.Run(ctx); close(r.done) }()
	select {
	case <-s.Ready():
	case err := <-r.done:
		t.Fatalf("server exited before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server not ready")
	}
	t.Cleanup(func() { r.stop(t) })
	return r
}

func (r *running) stop(t *testing.T) {
	r.cancel()
	select {
	case <-r.done:
	case <-time.After(5 * time.Second):
		t.Error("server did not stop")
	}
}

func TestRemoteMatchesLocal(t *testing.T) {
	d := newDirs(t)
	os.WriteFile(filepath.Join(d.notes, "a.md"), []byte("[[b]] [[nope]]"), 0o644)
	os.WriteFile(filepath.Join(d.notes, "b.md"), []byte("x"), 0o644)
	o := d.opts("v1")
	start(t, o)
	remote := client.Remote(o.SocketPath)

	ld := newDirs(t)
	os.WriteFile(filepath.Join(ld.notes, "a.md"), []byte("[[b]] [[nope]]"), 0o644)
	os.WriteFile(filepath.Join(ld.notes, "b.md"), []byte("x"), 0o644)
	env, closeEnv, err := ld.open()
	if err != nil {
		t.Fatal(err)
	}
	defer closeEnv()
	local := client.Local(env)

	ctx := context.Background()
	if err := local.Call(ctx, "index.reconcile", nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.reconcile", "index.status", "index.rebuild"} {
		var r, l map[string]any
		if err := remote.Call(ctx, name, nil, &r); err != nil {
			t.Fatalf("remote %s: %v", name, err)
		}
		if err := local.Call(ctx, name, nil, &l); err != nil {
			t.Fatalf("local %s: %v", name, err)
		}
		delete(r, "db")
		delete(l, "db")
		for _, m := range []map[string]any{r, l} {
			delete(m, "elapsed_ms")
			delete(m, "last_reconcile")
		}
		if len(r) != len(l) {
			t.Fatalf("%s: %v vs %v", name, r, l)
		}
		for k, v := range l {
			if r[k] != v {
				t.Fatalf("%s.%s: remote %v local %v", name, k, r[k], v)
			}
		}
	}

	for _, tc := range []struct {
		op, code string
		req      any
	}{
		{"no.such.op", ops.CodeNotFound, nil},
		{"index.status", ops.CodeInvalid, map[string]any{"bogus": 1}},
	} {
		re := remote.Call(ctx, tc.op, tc.req, nil)
		le := local.Call(ctx, tc.op, tc.req, nil)
		var rE, lE *ops.Error
		if !errors.As(re, &rE) || !errors.As(le, &lE) || rE.Code != tc.code || *rE != *lE {
			t.Fatalf("%s: remote %v local %v", tc.op, re, le)
		}
	}
}

func TestErrorStatusCodes(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	start(t, o)
	c := client.Remote(o.SocketPath)
	for name, code := range map[string]string{
		"test.srv.notfound": ops.CodeNotFound, "test.srv.invalid": ops.CodeInvalid,
		"test.srv.conflict": ops.CodeConflict, "test.srv.internal": ops.CodeInternal, "test.srv.plain": ops.CodeInternal,
	} {
		err := c.Call(context.Background(), name, nil, nil)
		var e *ops.Error
		if !errors.As(err, &e) || e.Code != code || e.Msg != "m" && name != "test.srv.plain" {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func init() {
	for name, code := range map[string]string{
		"test.srv.notfound": ops.CodeNotFound, "test.srv.invalid": ops.CodeInvalid,
		"test.srv.conflict": ops.CodeConflict, "test.srv.internal": ops.CodeInternal,
	} {
		code := code
		ops.Handle(name, func(context.Context, *ops.Env, ops.Empty) (ops.Empty, error) {
			return ops.Empty{}, &ops.Error{Code: code, Msg: "m"}
		})
	}
	ops.Handle("test.srv.plain", func(context.Context, *ops.Env, ops.Empty) (ops.Empty, error) {
		return ops.Empty{}, errors.New("boom")
	})
}

func TestSocketMode0600(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	start(t, o)
	fi, err := os.Stat(o.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}
}

func TestStaleSocketCleaned(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	l, err := net.Listen("unix", o.SocketPath)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	if _, err := os.Stat(o.SocketPath); err != nil {
		t.Fatal("setup: stale socket missing")
	}
	start(t, o)
	if err := client.Remote(o.SocketPath).Call(context.Background(), "index.status", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSecondInstanceDoesNotStart(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	first := start(t, o)
	err := New(o).Run(context.Background())
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("got %v", err)
	}
	if err := client.Remote(o.SocketPath).Call(context.Background(), "index.status", nil, nil); err != nil {
		t.Fatalf("first server damaged: %v", err)
	}
	_ = first
}

func TestShutdownEndpointStopsServerAndRemovesSocket(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	r := start(t, o)
	if err := client.Shutdown(o.SocketPath); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-r.done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no exit")
	}
	if _, err := os.Stat(o.SocketPath); !os.IsNotExist(err) {
		t.Fatalf("socket remains: %v", err)
	}
}

func TestVersionEndpoint(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v7")
	start(t, o)
	info, err := client.Probe(o.SocketPath)
	if err != nil || info.Version != "v7" || info.PID != os.Getpid() || info.Started == "" {
		t.Fatalf("%v %+v", err, info)
	}
}

func TestWatcherIndexesNewFilesAndDirs(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	start(t, o)
	c := client.Remote(o.SocketPath)
	count := func() int {
		var s ops.StatusResp
		if err := c.Call(context.Background(), "index.status", nil, &s); err != nil {
			t.Fatal(err)
		}
		return s.Notes
	}
	waitFor := func(want int, what string) {
		t.Helper()
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if count() == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("%s: notes=%d want %d", what, count(), want)
	}
	os.WriteFile(filepath.Join(d.notes, "a.md"), []byte("x"), 0o644)
	waitFor(1, "top-level file")
	os.MkdirAll(filepath.Join(d.notes, "sub", "deep"), 0o755)
	time.Sleep(100 * time.Millisecond)
	os.WriteFile(filepath.Join(d.notes, "sub", "deep", "b.md"), []byte("x"), 0o644)
	waitFor(2, "file in new nested dir")
	os.Remove(filepath.Join(d.notes, "a.md"))
	waitFor(1, "removed file")
}

func TestConnectMismatchShutsDownAndRespawns(t *testing.T) {
	d := newDirs(t)
	old := start(t, d.opts("old"))
	var spawned int
	var fresh *running
	c, err := client.Connect(client.ConnectOpts{
		Socket: d.opts("").SocketPath, Version: "new",
		Spawn: func() error {
			spawned++
			s := New(d.opts("new"))
			ctx, cancel := context.WithCancel(context.Background())
			fresh = &running{s, cancel, make(chan error, 1)}
			go func() { fresh.done <- s.Run(ctx) }()
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fresh.stop(t) })
	if spawned != 1 {
		t.Fatalf("spawned %d", spawned)
	}
	select {
	case <-old.done:
	case <-time.After(5 * time.Second):
		t.Fatal("old server still running")
	}
	info, err := client.Probe(d.opts("").SocketPath)
	if err != nil || info.Version != "new" {
		t.Fatalf("%v %+v", err, info)
	}
	if err := c.Call(context.Background(), "index.status", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestConnectMatchingVersionDoesNotSpawn(t *testing.T) {
	d := newDirs(t)
	o := d.opts("v1")
	start(t, o)
	_, err := client.Connect(client.ConnectOpts{Socket: o.SocketPath, Version: "v1",
		Spawn: func() error { return errors.New("must not spawn") }})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConnectSpawnFailureIsError(t *testing.T) {
	d := newDirs(t)
	_, err := client.Connect(client.ConnectOpts{Socket: d.opts("").SocketPath, Version: "v1",
		Spawn: func() error { return errors.New("no fork") }})
	if err == nil || err.Error() != "no fork" {
		t.Fatalf("%v", err)
	}
}

func TestConnectSpawnThatNeverListens(t *testing.T) {
	d := newDirs(t)
	_, err := client.Connect(client.ConnectOpts{Socket: d.opts("").SocketPath, Version: "v1",
		Spawn: func() error { return nil }, StartWait: 100 * time.Millisecond})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestListensDuringInitialReconcile(t *testing.T) {
	d := newDirs(t)
	os.WriteFile(filepath.Join(d.notes, "a.md"), []byte("[[b]]"), 0o644)
	os.WriteFile(filepath.Join(d.notes, "b.md"), []byte("x"), 0o644)
	o := d.opts("v1")
	entered, release := make(chan struct{}), make(chan struct{})
	o.beforeReconcile = func() { close(entered); <-release }
	s := New(o)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("initial reconcile never started")
	}

	if _, err := client.Probe(o.SocketPath); err != nil {
		t.Fatalf("version probe during initial reconcile: %v", err)
	}
	select {
	case <-s.Ready():
		t.Fatal("ready closed before initial reconcile finished")
	default:
	}

	res := make(chan error, 1)
	go func() {
		res <- client.Remote(o.SocketPath).Call(context.Background(), "index.status", nil, nil)
	}()
	select {
	case err := <-res:
		t.Fatalf("op returned during initial reconcile: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-res:
		if err != nil {
			t.Fatalf("op after initial reconcile: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("op never completed")
	}
	select {
	case <-s.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("never ready")
	}
}
