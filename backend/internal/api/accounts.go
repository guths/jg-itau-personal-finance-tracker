package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

type documentJSON struct {
	ID               int64  `json:"id"`
	AccountID        int64  `json:"account_id"`
	AccountName      string `json:"account_name"`
	Kind             string `json:"kind"`
	FileName         string `json:"file_name"`
	PeriodStart      *Date  `json:"period_start"`
	PeriodEnd        *Date  `json:"period_end"`
	ClosingDate      *Date  `json:"closing_date"`
	DueDate          *Date  `json:"due_date"`
	ReferenceMonth   *Month `json:"reference_month"`
	TotalCents       *int64 `json:"total_cents"`
	TransactionCount int    `json:"transaction_count"`
	ImportedAt       string `json:"imported_at"`
}

func (s *Server) getDocuments(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT d.id, d.account_id, a.name, d.kind, d.file_name,
		d.period_start, d.period_end, d.closing_date, d.due_date, d.reference_month, d.total_cents, d.imported_at,
		(SELECT COUNT(*) FROM transactions t WHERE t.document_id = d.id)
		FROM documents d JOIN accounts a ON a.id = d.account_id ORDER BY d.imported_at DESC, d.id DESC`)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	defer rows.Close()
	out := []documentJSON{}
	for rows.Next() {
		var d documentJSON
		var ps, pe, cd, dd, rm sql.NullTime
		var total sql.NullInt64
		var imported time.Time
		if err := rows.Scan(&d.ID, &d.AccountID, &d.AccountName, &d.Kind, &d.FileName, &ps, &pe, &cd, &dd, &rm,
			&total, &imported, &d.TransactionCount); err != nil {
			writeInternal(w, r, err)
			return
		}
		d.PeriodStart, d.PeriodEnd, d.ClosingDate, d.DueDate = nullDate(ps), nullDate(pe), nullDate(cd), nullDate(dd)
		if rm.Valid {
			m := Month(rm.Time)
			d.ReferenceMonth = &m
		}
		if total.Valid {
			d.TotalCents = &total.Int64
		}
		d.ImportedAt = imported.UTC().Format(time.RFC3339)
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func nullDate(t sql.NullTime) *Date {
	if !t.Valid {
		return nil
	}
	d := Date(t.Time)
	return &d
}

type accountJSON struct {
	ID         int64  `json:"id"`
	Provider   string `json:"provider"`
	Kind       string `json:"kind"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
}

func (s *Server) getAccounts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT id, provider, kind, external_id, name FROM accounts ORDER BY kind, name, id`)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	defer rows.Close()
	out := []accountJSON{}
	for rows.Next() {
		var a accountJSON
		if err := rows.Scan(&a.ID, &a.Provider, &a.Kind, &a.ExternalID, &a.Name); err != nil {
			writeInternal(w, r, err)
			return
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) patchAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len([]rune(name)) > 128 {
		writeError(w, http.StatusUnprocessableEntity, "o nome deve ter entre 1 e 128 caracteres")
		return
	}
	res, err := s.store.DB.ExecContext(r.Context(), `UPDATE accounts SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists int
		if err := s.store.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM accounts WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
			writeError(w, http.StatusNotFound, "conta não encontrada")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "name": name})
}
