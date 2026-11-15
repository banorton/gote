// Package web serves rendered notes and the embedded Excalidraw editor on a
// loopback-only HTTP listener guarded by a per-start token.
package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/paths"
	gweb "github.com/banorton/gote/web"
)

const cookieName = "gote_t"

type settings struct {
	lock      sync.Locker
	stateFile bool
	poll      time.Duration
}

type Option func(*settings)

// WithLock makes every index write take l first; the server passes its
// single-writer mutex so web writes serialize with ops and reconciles.
func WithLock(l sync.Locker) Option { return func(c *settings) { c.lock = l } }

// NoStateFile skips web.json; serverless commands use it so they never
// overwrite the file of a running server.
func NoStateFile() Option { return func(c *settings) { c.stateFile = false } }

// PollInterval sets how often an SSE stream stats the files it watches.
func PollInterval(d time.Duration) Option { return func(c *settings) { c.poll = d } }

type Server struct {
	env   *ops.Env
	cfg   settings
	root  string
	ln    net.Listener
	srv   *http.Server
	port  int
	token string
	state string

	wmu sync.Mutex

	mu   sync.Mutex
	wake chan struct{}

	closeOnce sync.Once
}

// Start listens on 127.0.0.1:0 and serves until ctx is done or Close is called.
func Start(ctx context.Context, env *ops.Env, opts ...Option) (*Server, error) {
	cfg := settings{stateFile: true, poll: 250 * time.Millisecond}
	for _, o := range opts {
		o(&cfg)
	}
	root, err := filepath.EvalSymlinks(env.Cfg.NotesDir)
	if err != nil {
		return nil, fmt.Errorf("web: resolve notes dir %s: %w", env.Cfg.NotesDir, err)
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, fmt.Errorf("web: generate token: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("web: listen: %w", err)
	}
	s := &Server{
		env: env, cfg: cfg, root: root, ln: ln,
		port:  ln.Addr().(*net.TCPAddr).Port,
		token: hex.EncodeToString(raw[:]),
		wake:  make(chan struct{}),
	}
	if cfg.stateFile {
		dir, err := paths.StateDir()
		if err != nil {
			ln.Close()
			return nil, err
		}
		s.state = filepath.Join(dir, "web.json")
		if err := s.writeState(); err != nil {
			ln.Close()
			return nil, err
		}
	}
	s.srv = &http.Server{Handler: s.handler(), ReadHeaderTimeout: 10 * time.Second}
	go s.srv.Serve(ln)
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	return s, nil
}

func (s *Server) writeState() error {
	data, err := json.Marshal(map[string]any{"port": s.port, "token": s.token})
	if err != nil {
		return err
	}
	tmp := s.state + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("web: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.state); err != nil {
		return fmt.Errorf("web: write %s: %w", s.state, err)
	}
	return nil
}

func (s *Server) Addr() string  { return fmt.Sprintf("127.0.0.1:%d", s.port) }
func (s *Server) Port() int     { return s.port }
func (s *Server) Token() string { return s.token }

// URL is a tokened browser URL for a route such as "/n/a.md".
func (s *Server) URL(route string) string {
	return "http://" + s.Addr() + route + "?t=" + s.token
}

// Notify wakes every SSE stream to re-check its files now. The server calls
// it after a reconcile so streams do not wait for the next poll tick.
func (s *Server) Notify() {
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.wake)
	s.wake = make(chan struct{})
}

func (s *Server) wakeCh() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wake
}

func (s *Server) Close() error {
	var err error
	s.closeOnce.Do(func() {
		err = s.srv.Close()
		if s.state != "" {
			if b, rerr := os.ReadFile(s.state); rerr == nil {
				var got struct{ Token string }
				if json.Unmarshal(b, &got) == nil && got.Token == s.token {
					os.Remove(s.state)
				}
			}
		}
	})
	return err
}

func (s *Server) lockWrites() func() {
	s.wmu.Lock()
	if s.cfg.lock != nil {
		s.cfg.lock.Lock()
	}
	return func() {
		if s.cfg.lock != nil {
			s.cfg.lock.Unlock()
		}
		s.wmu.Unlock()
	}
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("GET /n/{id...}", s.handleNote)
	mux.HandleFunc("GET /d/{id...}", s.handleEditor)
	mux.HandleFunc("GET /f/{path...}", s.handleFile)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("GET /api/drawing/{id...}", s.handleDrawingGet)
	mux.HandleFunc("PUT /api/drawing/{id...}", s.handleDrawingPut)
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", assetsHandler()))
	return s.guard(mux)
}

func assetsHandler() http.Handler {
	files := http.FileServerFS(gweb.Assets())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}

func (s *Server) hostOK(h string) bool {
	h = strings.ToLower(h)
	return h == fmt.Sprintf("127.0.0.1:%d", s.port) || h == fmt.Sprintf("localhost:%d", s.port)
}

func (s *Server) tokenOK(t string) bool {
	return t != "" && subtle.ConstantTimeCompare([]byte(t), []byte(s.token)) == 1
}

func originOK(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return false
	}
	u, err := url.Parse(o)
	return err == nil && u.Scheme == "http" && strings.EqualFold(u.Host, r.Host)
}

func badPath(p string) bool {
	if strings.ContainsAny(p, "\x00\\") {
		return true
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostOK(r.Host) {
			http.Error(w, "forbidden: bad Host", http.StatusForbidden)
			return
		}
		tok, fromQuery := r.URL.Query().Get("t"), false
		if tok != "" {
			fromQuery = true
		} else if c, err := r.Cookie(cookieName); err == nil {
			tok = c.Value
		}
		if !s.tokenOK(tok) {
			http.Error(w, "forbidden: missing or wrong token", http.StatusForbidden)
			return
		}
		if fromQuery {
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: s.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		}
		switch r.Method {
		case http.MethodGet, http.MethodHead:
		default:
			if !originOK(r) {
				http.Error(w, "forbidden: bad Origin", http.StatusForbidden)
				return
			}
		}
		if badPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func fail(w http.ResponseWriter, status int, format string, args ...any) {
	http.Error(w, fmt.Sprintf(format, args...), status)
}

func failErr(w http.ResponseWriter, err error) {
	var oe *ops.Error
	status := http.StatusInternalServerError
	if errors.As(err, &oe) {
		switch oe.Code {
		case ops.CodeNotFound:
			status = http.StatusNotFound
		case ops.CodeInvalid:
			status = http.StatusBadRequest
		case ops.CodeConflict:
			status = http.StatusConflict
		}
	}
	fail(w, status, "%v", err)
}
