package web

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// vaultPath maps a notes-dir-relative id to an absolute path, refusing
// anything outside the notes dir: invalid or hidden segments, symlinks, and
// non-regular files.
func (s *Server) vaultPath(rel string) (string, bool) {
	if !fs.ValidPath(rel) || rel == "." {
		return "", false
	}
	for _, seg := range strings.Split(rel, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", false
		}
	}
	abs := filepath.Join(s.root, filepath.FromSlash(rel))
	fi, err := os.Lstat(abs)
	if err != nil || !fi.Mode().IsRegular() {
		return "", false
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil || real != abs {
		return "", false
	}
	return abs, true
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	abs, ok := s.vaultPath(r.PathValue("path"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	// Vault files are untrusted: sandbox them so an html or svg attachment
	// cannot script this origin and reach the write API.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; media-src 'self'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// lookupFile finds an attachment named by a wikilink embed: an exact
// notes-dir-relative path, else the shortest path with that basename.
func (s *Server) lookupFile(name string) (string, bool) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if _, ok := s.vaultPath(name); ok {
		return name, true
	}
	base := strings.ToLower(path.Base(name))
	var hits []string
	filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") && p != s.root {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && strings.ToLower(d.Name()) == base {
			if rel, err := filepath.Rel(s.root, p); err == nil {
				hits = append(hits, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	sort.Slice(hits, func(i, j int) bool {
		if len(hits[i]) != len(hits[j]) {
			return len(hits[i]) < len(hits[j])
		}
		return hits[i] < hits[j]
	})
	if len(hits) == 0 {
		return "", false
	}
	return hits[0], true
}

// kindOf validates id against the index. A file that exists on disk but is
// not indexed yet (created since the last reconcile) is indexed on the spot.
func (s *Server) kindOf(id string) (string, bool) {
	kind, ok := s.indexedKind(id)
	if ok {
		return kind, true
	}
	ext := strings.ToLower(path.Ext(id))
	if ext != ".md" && ext != ".excalidraw" {
		return "", false
	}
	if _, ok := s.vaultPath(id); !ok {
		return "", false
	}
	unlock := s.lockWrites()
	err := s.env.Index.IndexFile(id)
	unlock()
	if err != nil {
		return "", false
	}
	return s.indexedKind(id)
}

func (s *Server) indexedKind(id string) (string, bool) {
	var kind string
	err := s.env.Store.DB.QueryRow("SELECT kind FROM notes WHERE id=?", id).Scan(&kind)
	return kind, err == nil
}
