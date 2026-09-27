package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWithWeb(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644))
	must(os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	must(os.WriteFile(filepath.Join(dir, "assets", "app-123.js"), []byte("console.log(1)"), 0o644))
	apiHit := false
	h := WithWeb(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { apiHit = true }), dir)

	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		return rec
	}
	if get("/api/months"); !apiHit {
		t.Error("/api/ must reach the API handler")
	}
	for _, p := range []string{"/", "/perguntas", "/lancamentos/123"} {
		if rec := get(p); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "app") {
			t.Errorf("GET %s = %d %q, want index.html", p, rec.Code, rec.Body.String())
		}
	}
	rec := get("/assets/app-123.js")
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("asset: %d %q %q", rec.Code, rec.Body.String(), rec.Header().Get("Cache-Control"))
	}
	if rec := get("/../../etc/passwd"); strings.Contains(rec.Body.String(), "root:") {
		t.Error("path traversal must not escape dir")
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/perguntas", nil))
	if post.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST static = %d", post.Code)
	}
	if WithWeb(http.NotFoundHandler(), "") == nil {
		t.Error("empty dir must return the API handler")
	}
}
