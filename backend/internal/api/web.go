package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// WithWeb serves the built web app from dir next to the API: /api/ goes to apiHandler, any
// other path is a static file, and unknown paths fall back to index.html so client-side
// routes (/perguntas, /lancamentos, ...) survive a page reload. Empty dir = API only.
func WithWeb(apiHandler http.Handler, dir string) http.Handler {
	if dir == "" {
		return apiHandler
	}
	files := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			apiHandler.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "método não permitido")
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); clean == "/" || err != nil || info.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, index)
			return
		}
		if strings.HasPrefix(clean, "/assets/") { // Vite fingerprints these file names
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
