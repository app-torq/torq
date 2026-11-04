package httpapi

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// setStaticHeaders sets cache headers for the embedded files. The service
// worker and the manifest are revalidated on every load, so a new build is
// picked up at once (specs/004-installable-pwa, research W4).
func setStaticHeaders(h http.Header, name string) {
	switch {
	case strings.HasPrefix(name, "assets/"):
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	case name == "sw.js":
		h.Set("Cache-Control", "no-cache")
	case name == "manifest.webmanifest":
		// Go's MIME table lacks .webmanifest and the distroless image has no
		// /etc/mime.types.
		h.Set("Content-Type", "application/manifest+json")
		h.Set("Cache-Control", "no-cache")
	case strings.HasPrefix(name, "icons/") || name == "apple-touch-icon.png":
		h.Set("Cache-Control", "public, max-age=86400")
	}
}

// spaHandler serves the embedded SPA: real files directly (hashed assets are
// immutable), and index.html for every other path so client routing works.
func spaHandler(dist fs.FS) http.Handler {
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(dist, name); err == nil && !st.IsDir() {
				setStaticHeaders(w.Header(), name)
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "frontend not built: run `make build-web`", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
	})
}
