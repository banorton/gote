package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type stamp struct {
	mtime, size int64
}

func (s *Server) watched(id string) ([]string, error) {
	ids := []string{id}
	rows, err := s.env.Store.DB.Query(`SELECT DISTINCT dst_id FROM links WHERE src=? AND kind='embed' AND dst_id IS NOT NULL`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		ids = append(ids, d)
	}
	return ids, rows.Err()
}

func (s *Server) snapshot(ids []string) []stamp {
	out := make([]stamp, len(ids))
	for i, id := range ids {
		if fi, err := os.Stat(filepath.Join(s.root, filepath.FromSlash(id))); err == nil {
			out[i] = stamp{fi.ModTime().UnixNano(), fi.Size()}
		}
	}
	return out
}

func same(a, b []stamp) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Clients is the number of open event streams; every note and editor page holds one.
func (s *Server) Clients() int { return int(s.clients.Load()) }

// handleEvents streams "reload" when the note or a drawing it embeds changes.
// It polls file stamps; Notify wakes it early. After Phase 7 lands, the main
// session should call Notify from an index.OnReconcile hook.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if _, ok := s.kindOf(id); !ok {
		http.NotFound(w, r)
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		fail(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	ids, err := s.watched(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "watch %s: %v", id, err)
		return
	}
	snap := s.snapshot(ids)
	s.clients.Add(1)
	defer s.clients.Add(-1)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	fmt.Fprint(w, ": connected\n\n")
	fl.Flush()

	tick := time.NewTicker(s.cfg.poll)
	defer tick.Stop()
	for {
		wake := s.wakeCh()
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		case <-wake:
		}
		if now := s.snapshot(ids); !same(snap, now) {
			snap = now
			fmt.Fprint(w, "event: reload\ndata: {}\n\n")
			fl.Flush()
		}
	}
}
