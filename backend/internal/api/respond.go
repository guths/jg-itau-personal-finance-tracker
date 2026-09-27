package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"jgfinance/internal/categorize"
)

const msgUnexpected = "erro inesperado; veja o log da API"

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("escrever resposta", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeInternal logs err (never shown to the client) and answers 500.
func writeInternal(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("erro inesperado", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, msgUnexpected)
}

// writeServiceError maps the errors of categorize to HTTP statuses.
func writeServiceError(w http.ResponseWriter, r *http.Request, err error, notFound string) {
	switch {
	case errors.Is(err, categorize.ErrNotFound):
		writeError(w, http.StatusNotFound, notFound)
	case errors.Is(err, categorize.ErrAlreadyAnswered):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, categorize.ErrInvalidCategory), errors.Is(err, categorize.ErrInvalidKind),
		errors.Is(err, categorize.ErrInvalidScope), errors.Is(err, categorize.ErrRestoreScope), errors.Is(err, categorize.ErrInvalidRule),
		errors.Is(err, categorize.ErrNoLLM), errors.Is(err, categorize.ErrEmptyText):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, categorize.ErrLLMFailed):
		slog.Warn("IA falhou", "method", r.Method, "path", r.URL.Path, "err", err)
		writeError(w, http.StatusBadGateway, categorize.ErrLLMFailed.Error())
	default:
		writeInternal(w, r, err)
	}
}

// decodeJSON reads a JSON body into v, rejecting unknown fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return false
	}
	return true
}

// pathID parses the {id} wildcard.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id inválido")
		return 0, false
	}
	return id, true
}

// Month is a reference month, serialized as "AAAA-MM".
type Month time.Time

func (m Month) MarshalJSON() ([]byte, error) {
	if time.Time(m).IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + time.Time(m).Format("2006-01") + `"`), nil
}

// Date is a civil date, serialized as "AAAA-MM-DD".
type Date time.Time

func (d Date) MarshalJSON() ([]byte, error) {
	if time.Time(d).IsZero() {
		return []byte("null"), nil
	}
	return []byte(`"` + time.Time(d).Format("2006-01-02") + `"`), nil
}

// parseMonth parses "AAAA-MM".
func parseMonth(s string) (time.Time, error) {
	t, err := time.Parse("2006-01", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("mês inválido %q: use AAAA-MM", s)
	}
	return t, nil
}
