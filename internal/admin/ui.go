package admin

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// UIHandler serves a static admin web UI directory and falls back to index.html for SPA routes.
type UIHandler struct {
	Root string
}

func (h UIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	clean := path.Clean("/" + r.URL.Path)
	rel := strings.TrimPrefix(clean, "/")
	if rel == "." || rel == "" {
		h.serveIndex(w, r)
		return
	}
	full := filepath.Join(h.Root, filepath.FromSlash(rel))
	if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	// SPA fallback.
	h.serveIndex(w, r)
}

func (h UIHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	index := filepath.Join(h.Root, "index.html")
	if fi, err := os.Stat(index); err == nil && !fi.IsDir() {
		http.ServeFile(w, r, index)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("admin UI build not found. expected web/admin-ui/dist/index.html"))
}
