// Package api serves the JSON HTTP API under /api (spec §12) with net/http route patterns.
package api

import (
	"log/slog"
	"net/http"
	"time"

	"jgfinance/internal/categorize"
	"jgfinance/internal/importer"
	"jgfinance/internal/insights"
	"jgfinance/internal/store"
)

// Server holds the services behind the handlers.
type Server struct {
	store      *store.Store
	importer   *importer.Importer
	categorize *categorize.Categorizer
	insights   *insights.Service
	now        func() time.Time
}

// New returns the API handler.
func New(st *store.Store, im *importer.Importer, cat *categorize.Categorizer, ins *insights.Service) http.Handler {
	s := &Server{store: st, importer: im, categorize: cat, insights: ins, now: time.Now}
	return s.routes()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/imports", s.postImports)
	mux.HandleFunc("GET /api/documents", s.getDocuments)
	mux.HandleFunc("GET /api/accounts", s.getAccounts)
	mux.HandleFunc("PATCH /api/accounts/{id}", s.patchAccount)
	mux.HandleFunc("GET /api/categories", s.getCategories)
	mux.HandleFunc("POST /api/categories", s.postCategory)
	mux.HandleFunc("PATCH /api/categories/{id}", s.patchCategory)
	mux.HandleFunc("GET /api/questions", s.getQuestions)
	mux.HandleFunc("POST /api/questions/suggest", s.postSuggest)
	mux.HandleFunc("POST /api/questions/{id}/answer", s.postAnswer)
	mux.HandleFunc("POST /api/questions/{id}/skip", s.postSkip)
	mux.HandleFunc("POST /api/questions/{id}/interpret", s.postInterpret)
	mux.HandleFunc("GET /api/transactions", s.getTransactions)
	mux.HandleFunc("PATCH /api/transactions/{id}", s.patchTransaction)
	mux.HandleFunc("GET /api/rules", s.getRules)
	mux.HandleFunc("PATCH /api/rules/{id}", s.patchRule)
	mux.HandleFunc("DELETE /api/rules/{id}", s.deleteRule)
	mux.HandleFunc("GET /api/months", s.getMonths)
	mux.HandleFunc("GET /api/dashboard", s.getDashboard)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "rota não encontrada")
	})
	return logRequests(recoverPanics(mux))
}

// statusWriter remembers the status code for the access log.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}

func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic no handler", "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, msgUnexpected)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
