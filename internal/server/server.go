package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/ops"
)

var ErrAlreadyRunning = errors.New("server already running")

type Options struct {
	Version, SocketPath, LockPath, LogPath string
	// Open builds the op environment once the lock is held; the returned func releases it.
	Open     func() (*ops.Env, func(), error)
	Debounce time.Duration
}

type Server struct {
	o       Options
	ready   chan struct{}
	mu      sync.Mutex // single writer: held around every op and reconcile
	env     *ops.Env
	log     *log.Logger
	started time.Time
	stop    chan struct{}
	stopOne sync.Once
}

func New(o Options) *Server {
	if o.Debounce == 0 {
		o.Debounce = 200 * time.Millisecond
	}
	return &Server{o: o, ready: make(chan struct{}), stop: make(chan struct{})}
}

// Ready closes once the socket accepts connections.
func (s *Server) Ready() <-chan struct{} { return s.ready }

func (s *Server) shutdown() { s.stopOne.Do(func() { close(s.stop) }) }

func (s *Server) Run(ctx context.Context) error {
	if len(s.o.SocketPath) > 100 {
		return fmt.Errorf("socket path %s is too long for a unix socket (%d bytes, limit ~104): shorten XDG_STATE_HOME", s.o.SocketPath, len(s.o.SocketPath))
	}
	lock, err := s.lockOrExisting()
	if err != nil {
		return err
	}
	defer lock.Close()

	logf, err := os.OpenFile(s.o.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open server log %s: %w", s.o.LogPath, err)
	}
	defer logf.Close()
	s.log = log.New(logf, "", log.LstdFlags)

	if err := removeStaleSocket(s.o.SocketPath); err != nil {
		return err
	}
	env, release, err := s.o.Open()
	if err != nil {
		return err
	}
	defer release()
	s.env = env
	raiseFDLimit(s.log)

	if _, err := env.Index.Reconcile(ctx); err != nil {
		return fmt.Errorf("initial reconcile: %w", err)
	}
	defer s.startSemantic(ctx)()

	ln, err := net.Listen("unix", s.o.SocketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.o.SocketPath, err)
	}
	if err := os.Chmod(s.o.SocketPath, 0o600); err != nil {
		ln.Close()
		return fmt.Errorf("chmod socket: %w", err)
	}
	defer os.Remove(s.o.SocketPath)

	s.started = time.Now()
	srv := &http.Server{Handler: s.handler(), ErrorLog: s.log}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	w, err := s.startWatch()
	if err != nil {
		ln.Close()
		return fmt.Errorf("watch notes dir: %w", err)
	}
	watchDone := make(chan struct{})
	wctx, wcancel := context.WithCancel(ctx)
	go func() {
		defer close(watchDone)
		defer w.Close()
		s.watchLoop(wctx, w)
	}()

	s.log.Printf("server %s pid %d listening on %s", s.o.Version, os.Getpid(), s.o.SocketPath)
	close(s.ready)

	select {
	case <-ctx.Done():
	case <-s.stop:
	case err := <-serveErr:
		wcancel()
		<-watchDone
		return err
	}
	wcancel()
	<-watchDone
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = srv.Shutdown(sctx)
	s.log.Printf("server stopped")
	return err
}

func acquireLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// lockOrExisting returns ErrAlreadyRunning if a live server holds the lock. A
// holder that is still shutting down (socket gone, lock not yet released) is
// waited out so a version-mismatch respawn can take over.
func (s *Server) lockOrExisting() (*os.File, error) {
	deadline := time.Now().Add(3 * time.Second)
	for {
		f, err := acquireLock(s.o.LockPath)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, err
		}
		if _, perr := client.Probe(s.o.SocketPath); perr == nil {
			return nil, ErrAlreadyRunning
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("lock %s is held but no server answers on %s", s.o.LockPath, s.o.SocketPath)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func removeStaleSocket(path string) error {
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if conn, err := net.DialTimeout("unix", path, time.Second); err == nil {
		conn.Close()
		return fmt.Errorf("socket %s is live but the lock was free", path)
	}
	return os.Remove(path)
}

// raiseFDLimit: kqueue-backed fsnotify holds one fd per watched file.
func raiseFDLimit(l *log.Logger) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		l.Printf("getrlimit: %v", err)
		return
	}
	for _, want := range []uint64{lim.Max, 65536, 10240} {
		if want > lim.Max || want <= lim.Cur {
			continue
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Cur: want, Max: lim.Max}); err == nil {
			return
		}
	}
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, client.ServerInfo{Version: s.o.Version, PID: os.Getpid(), Started: s.started.UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("POST /v1/shutdown", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, struct{}{})
		go s.shutdown()
	})
	mux.HandleFunc("POST /v1/op/{name...}", s.handleOp)
	return mux
}

func (s *Server) handleOp(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	op, ok := ops.Lookup(name)
	if !ok {
		writeErr(w, ops.Errorf(ops.CodeNotFound, "unknown op %q", name))
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, ops.Errorf(ops.CodeInvalid, "%s: read body: %v", name, err))
		return
	}
	s.mu.Lock()
	out, err := op.Run(r.Context(), s.env, body)
	s.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	data, err := json.Marshal(out)
	if err != nil {
		writeErr(w, ops.Errorf(ops.CodeInternal, "%s: encode response: %v", name, err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var oe *ops.Error
	if !errors.As(err, &oe) {
		oe = &ops.Error{Code: ops.CodeInternal, Msg: err.Error()}
	}
	status := http.StatusInternalServerError
	switch oe.Code {
	case ops.CodeNotFound:
		status = http.StatusNotFound
	case ops.CodeInvalid:
		status = http.StatusBadRequest
	case ops.CodeConflict:
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]any{"error": oe})
}

func (s *Server) startWatch() (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	root := s.env.Cfg.NotesDir
	if err := addTree(w, root, root); err != nil {
		w.Close()
		return nil, err
	}
	return w, nil
}

func (s *Server) watchLoop(ctx context.Context, w *fsnotify.Watcher) {
	root := s.env.Cfg.NotesDir
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			s.log.Printf("watch error: %v", err)
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if hidden(root, ev.Name) {
				continue
			}
			if ev.Has(fsnotify.Create) {
				if fi, err := os.Lstat(ev.Name); err == nil && fi.IsDir() {
					if err := addTree(w, root, ev.Name); err != nil {
						s.log.Printf("watch %s: %v", ev.Name, err)
					}
				}
			}
			timer.Reset(s.o.Debounce)
		case <-timer.C:
			s.mu.Lock()
			st, err := s.env.Index.Reconcile(ctx)
			s.mu.Unlock()
			if err != nil && ctx.Err() == nil {
				s.log.Printf("reconcile: %v", err)
			} else if err == nil && (st.Added+st.Updated+st.Removed) > 0 {
				s.log.Printf("reconciled: +%d ~%d -%d", st.Added, st.Updated, st.Removed)
			}
		}
	}
}

func hidden(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return false
	}
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func addTree(w *fsnotify.Watcher, root, dir string) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if hidden(root, p) {
			return filepath.SkipDir
		}
		if err := w.Add(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
}
