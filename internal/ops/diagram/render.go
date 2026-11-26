package diagram

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/excal/render"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/paths"
)

type RenderResp struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// Fresh is true when this call produced the PNG, false when a current one existed.
	Fresh bool `json:"fresh"`
}

func init() {
	ops.Handle("diagram.render", func(ctx context.Context, env *ops.Env, req IDReq) (RenderResp, error) {
		return Ensure(env, req.ID)
	})
}

func pngMode(env *ops.Env) string {
	if env.Cfg.DiagramPNG == "" {
		return config.PNGCache
	}
	return env.Cfg.DiagramPNG
}

func cacheDir() (string, error) {
	st, err := paths.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(st, "renders"), nil
}

func target(env *ops.Env, abs, hash string) (string, error) {
	if pngMode(env) == config.PNGSidecar {
		return strings.TrimSuffix(abs, filepath.Ext(abs)) + ".png", nil
	}
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, hash+".png"), nil
}

// Ensure returns the current render of a drawing, rendering it when the PNG is
// missing or was made from different content or by another renderer version.
func Ensure(env *ops.Env, id string) (RenderResp, error) {
	abs, err := validate(env, id)
	if err != nil {
		return RenderResp{}, err
	}
	src, err := os.ReadFile(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return RenderResp{}, ops.Errorf(ops.CodeNotFound, "diagram %s not found", id)
	}
	if err != nil {
		return RenderResp{}, ops.Errorf(ops.CodeInternal, "read %s: %v", id, err)
	}
	hash := render.Hash(src)
	out, err := target(env, abs, hash)
	if err != nil {
		return RenderResp{}, ops.Errorf(ops.CodeInternal, "%v", err)
	}
	if f, err := os.Open(out); err == nil {
		h, w, ht, ierr := render.Info(f)
		f.Close()
		if ierr == nil && h == hash {
			return RenderResp{ID: id, Path: out, Width: w, Height: ht}, nil
		}
	}
	data, w, h, err := render.PNG(src)
	if err != nil {
		return RenderResp{}, ops.Errorf(ops.CodeInvalid, "%s: %v", id, err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return RenderResp{}, ops.Errorf(ops.CodeInternal, "create %s: %v", filepath.Dir(out), err)
	}
	if err := writeAtomic(out, data, 0o644); err != nil {
		return RenderResp{}, ops.Errorf(ops.CodeInternal, "write %s: %v", out, err)
	}
	return RenderResp{ID: id, Path: out, Width: w, Height: h, Fresh: true}, nil
}

func validate(env *ops.Env, id string) (string, error) {
	if !fs.ValidPath(id) || id == "." {
		return "", ops.Errorf(ops.CodeInvalid, "invalid diagram id %q", id)
	}
	if !strings.EqualFold(path.Ext(id), ".excalidraw") {
		return "", ops.Errorf(ops.CodeInvalid, "%s is not a drawing", id)
	}
	return filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id)), nil
}

var cacheName = regexp.MustCompile(`^[0-9a-f]{32}\.png$`)

func listDrawings(ctx context.Context, env *ops.Env) (map[string]int64, error) {
	rows, err := env.Store.DB.QueryContext(ctx, "SELECT id, mtime_ns FROM notes WHERE kind = 'diagram'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]int64{}
	for rows.Next() {
		var id string
		var ns int64
		if err := rows.Scan(&id, &ns); err != nil {
			return nil, err
		}
		m[id] = ns
	}
	return m, rows.Err()
}

// Prune deletes cached renders that no indexed drawing currently hashes to:
// old versions and renders of drawings that no longer exist. In sidecar mode
// the cache is not used, so every entry goes. Only <32 hex>.png files are touched.
func Prune(ctx context.Context, env *ops.Env) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	if pngMode(env) != config.PNGSidecar {
		ids, err := listDrawings(ctx, env)
		if err != nil {
			return err
		}
		for id := range ids {
			src, err := os.ReadFile(filepath.Join(env.Cfg.NotesDir, filepath.FromSlash(id)))
			if err != nil {
				continue
			}
			keep[render.Hash(src)+".png"] = true
		}
	}
	for _, e := range entries {
		if cacheName.MatchString(e.Name()) && !keep[e.Name()] {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

// Debounce is how long index changes settle before drawings are re-rendered.
var Debounce = 500 * time.Millisecond

// StartBackground keeps PNGs current after index changes until ctx ends or the
// returned stop func is called. Errors go to logf and never stop the loop. It
// is a no-op when diagram_png is off.
func StartBackground(ctx context.Context, env *ops.Env, logf func(format string, args ...any)) (stop func()) {
	if pngMode(env) == config.PNGOff {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	trigger := make(chan struct{}, 1)
	poke := func() {
		select {
		case trigger <- struct{}{}:
		default:
		}
	}
	unsub := env.Index.OnReconcile(func(index.Stats) { poke() })
	poke()
	done := make(chan struct{})
	go func() {
		defer close(done)
		seen := map[string]int64{}
		first := true
		for {
			select {
			case <-ctx.Done():
				return
			case <-trigger:
			}
			timer := time.NewTimer(Debounce)
		settle:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-trigger:
					timer.Reset(Debounce)
				case <-timer.C:
					break settle
				}
			}
			changed, err := syncRenders(ctx, env, seen, logf)
			if err != nil {
				if ctx.Err() == nil {
					logf("diagram render: %v", err)
				}
				continue
			}
			if changed || first {
				first = false
				if err := Prune(ctx, env); err != nil && ctx.Err() == nil {
					logf("diagram render prune: %v", err)
				}
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			unsub()
			cancel()
			<-done
		})
	}
}

// syncRenders renders every drawing whose indexed mtime differs from the last
// pass. It reports whether any render was produced or a drawing disappeared.
func syncRenders(ctx context.Context, env *ops.Env, seen map[string]int64, logf func(string, ...any)) (bool, error) {
	ids, err := listDrawings(ctx, env)
	if err != nil {
		return false, err
	}
	changed := false
	for id := range seen {
		if _, ok := ids[id]; !ok {
			delete(seen, id)
			changed = true
		}
	}
	for id, ns := range ids {
		if ctx.Err() != nil {
			return changed, nil
		}
		if prev, ok := seen[id]; ok && prev == ns {
			continue
		}
		seen[id] = ns
		r, err := Ensure(env, id)
		if err != nil {
			logf("diagram render %s: %v", id, err)
			continue
		}
		changed = changed || r.Fresh
	}
	return changed, nil
}
