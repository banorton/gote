package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/note"
)

const maxDrawingBytes = 64 << 20

// drawingPath validates id as an indexed drawing and returns its file path.
func (s *Server) drawingPath(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	id := r.PathValue("id")
	kind, ok := s.kindOf(id)
	if !ok || kind != note.KindDiagram {
		http.NotFound(w, r)
		return "", "", false
	}
	abs, ok := s.vaultPath(id)
	if !ok {
		http.NotFound(w, r)
		return "", "", false
	}
	return id, abs, true
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	kind, ok := s.kindOf(id)
	if !ok || kind != note.KindDiagram {
		http.NotFound(w, r)
		return
	}
	renderPage(w, editorTmpl, editorPage{ID: id, Title: filepath.Base(id)})
}

func (s *Server) handleDrawingGet(w http.ResponseWriter, r *http.Request) {
	id, abs, ok := s.drawingPath(w, r)
	if !ok {
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		fail(w, http.StatusInternalServerError, "open %s: %v", id, err)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		fail(w, http.StatusInternalServerError, "stat %s: %v", id, err)
		return
	}
	src, err := io.ReadAll(f)
	if err != nil {
		fail(w, http.StatusInternalServerError, "read %s: %v", id, err)
		return
	}
	if !json.Valid(src) {
		fail(w, http.StatusUnprocessableEntity, "%s is not valid JSON", id)
		return
	}
	// mtime_ns is a string: unix nanoseconds exceed 2^53 and JS would round a number.
	writeJSON(w, http.StatusOK, struct {
		Scene   json.RawMessage `json:"scene"`
		MtimeNS string          `json:"mtime_ns"`
	}{src, strconv.FormatInt(fi.ModTime().UnixNano(), 10)})
}

func (s *Server) handleDrawingPut(w http.ResponseWriter, r *http.Request) {
	id, abs, ok := s.drawingPath(w, r)
	if !ok {
		return
	}
	match := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `"`)
	if match == "" {
		fail(w, http.StatusPreconditionRequired, "If-Match header (the mtime_ns from GET) is required")
		return
	}
	want, err := strconv.ParseInt(match, 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "If-Match %q is not an integer mtime_ns", match)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxDrawingBytes))
	if err != nil {
		fail(w, http.StatusRequestEntityTooLarge, "read body: %v", err)
		return
	}
	if _, err := excal.Decode(body); err != nil {
		fail(w, http.StatusBadRequest, "%v", err)
		return
	}

	unlock := s.lockWrites()
	defer unlock()
	fi, err := os.Stat(abs)
	if err != nil {
		fail(w, http.StatusNotFound, "stat %s: %v", id, err)
		return
	}
	if fi.ModTime().UnixNano() != want {
		fail(w, http.StatusConflict, "%s changed on disk (mtime_ns %d, expected %d)", id, fi.ModTime().UnixNano(), want)
		return
	}
	existing, err := os.ReadFile(abs)
	if err != nil {
		fail(w, http.StatusInternalServerError, "read %s: %v", id, err)
		return
	}
	// Excalidraw drops unknown top-level keys on save; keep gote's from disk.
	if merged, err := excal.CarryGote(existing, body); err != nil {
		if _, derr := excal.Decode(existing); derr == nil {
			fail(w, http.StatusInternalServerError, "merge gote metadata for %s: %v", id, err)
			return
		}
	} else {
		body = merged
	}
	if err := writeAtomic(abs, body, fi.Mode().Perm()); err != nil {
		fail(w, http.StatusInternalServerError, "write %s: %v", id, err)
		return
	}
	nfi, err := os.Stat(abs)
	if err != nil {
		fail(w, http.StatusInternalServerError, "stat %s: %v", id, err)
		return
	}
	if err := s.env.Index.IndexFile(id); err != nil {
		fail(w, http.StatusInternalServerError, "index %s: %v", id, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mtime_ns": strconv.FormatInt(nfi.ModTime().UnixNano(), 10)})
}

// writeAtomic replaces path via a hidden temp file in the same directory, so
// a crash never leaves a half-written drawing and watchers ignore the temp.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gote-write-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fail(w, http.StatusInternalServerError, "encode: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintln(w, string(b))
}
