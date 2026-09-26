package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"jgfinance/internal/categorize"
)

type suggestionJSON struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

type questionTxJSON struct {
	ID           int64  `json:"id"`
	Date         Date   `json:"date"`
	Description  string `json:"description"`
	AmountCents  int64  `json:"amount_cents"`
	AccountName  string `json:"account_name"`
	BankCategory string `json:"bank_category,omitempty"`
	City         string `json:"city,omitempty"`
}

type questionJSON struct {
	ID             int64            `json:"id"`
	Key            string           `json:"key"`
	Alias          string           `json:"alias,omitempty"`
	PerTransaction bool             `json:"per_transaction"`
	Status         string           `json:"status"`
	Prompt         string           `json:"prompt"`
	Suggestions    []suggestionJSON `json:"suggestions"`
	TotalCents     int64            `json:"total_cents"`
	Count          int              `json:"count"`
	PersonTransfer bool             `json:"person_transfer"`
	Transactions   []questionTxJSON `json:"transactions"`
}

// getQuestions lists open questions with their transactions: ?status=pending|skipped, or both
// (pending first, then skipped) when omitted.
func (s *Server) getQuestions(w http.ResponseWriter, r *http.Request) {
	statuses := []any{"pending", "skipped"}
	switch st := r.URL.Query().Get("status"); st {
	case "":
	case "pending", "skipped":
		statuses = []any{st}
	default:
		writeError(w, http.StatusBadRequest, "status inválido: use pending ou skipped")
		return
	}
	out, err := s.listQuestions(r.Context(), statuses)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) listQuestions(ctx context.Context, statuses []any) ([]questionJSON, error) {
	paths, err := subcategoryPaths(ctx, s.store.DB)
	if err != nil {
		return nil, err
	}
	in := "?"
	if len(statuses) == 2 {
		in = "?, ?"
	}
	rows, err := s.store.DB.QueryContext(ctx, `SELECT q.id, q.merchant_key, COALESCE(r.alias, ''), q.per_transaction,
		q.status, COALESCE(q.prompt, ''), q.suggestions
		FROM questions q LEFT JOIN rules r ON r.merchant_key = q.merchant_key
		WHERE q.status IN (`+in+`) ORDER BY q.status = 'skipped', q.id`, statuses...)
	if err != nil {
		return nil, err
	}
	out := []questionJSON{}
	index := map[int64]int{}
	for rows.Next() {
		var q questionJSON
		var sugg sql.NullString
		if err := rows.Scan(&q.ID, &q.Key, &q.Alias, &q.PerTransaction, &q.Status, &q.Prompt, &sugg); err != nil {
			rows.Close()
			return nil, err
		}
		q.Suggestions = []suggestionJSON{}
		if sugg.Valid {
			var ids []int64
			_ = json.Unmarshal([]byte(sugg.String), &ids)
			for _, id := range ids {
				if p, ok := paths[id]; ok {
					q.Suggestions = append(q.Suggestions, suggestionJSON{ID: id, Path: p})
				}
			}
		}
		q.Transactions = []questionTxJSON{}
		index[q.ID] = len(out)
		out = append(out, q)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = s.store.DB.QueryContext(ctx, `SELECT t.question_id, t.id, t.occurred_on, t.description, t.amount_cents,
		a.name, COALESCE(t.bank_category, ''), COALESCE(t.city, ''), t.person_transfer
		FROM transactions t JOIN questions q ON q.id = t.question_id JOIN accounts a ON a.id = t.account_id
		WHERE q.status IN (`+in+`) ORDER BY t.occurred_on, t.id`, statuses...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var qid int64
		var t questionTxJSON
		var when time.Time
		var person bool
		if err := rows.Scan(&qid, &t.ID, &when, &t.Description, &t.AmountCents, &t.AccountName, &t.BankCategory, &t.City, &person); err != nil {
			return nil, err
		}
		t.Date = Date(when)
		i, ok := index[qid]
		if !ok {
			continue
		}
		q := &out[i]
		q.Transactions = append(q.Transactions, t)
		q.TotalCents += t.AmountCents
		q.Count++
		q.PersonTransfer = q.PersonTransfer || person
	}
	return out, rows.Err()
}

// subcategoryPaths maps every subcategory id to "Categoria › Subcategoria".
func subcategoryPaths(ctx context.Context, db *sql.DB) (map[int64]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT s.id, CONCAT(c.name, ' › ', s.name) FROM categories s JOIN categories c ON c.id = s.parent_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var p string
		if err := rows.Scan(&id, &p); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

func (s *Server) postAnswer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		CategoryID *int64 `json:"category_id"`
		Kind       string `json:"kind"`
		Remember   bool   `json:"remember"`
		Alias      string `json:"alias"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	err := s.categorize.Answer(r.Context(), id, categorize.Answer{
		CategoryID: body.CategoryID, Kind: body.Kind, Remember: body.Remember, Alias: body.Alias,
	})
	if err != nil {
		writeServiceError(w, r, err, "pergunta não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "answered"})
}

func (s *Server) postSkip(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.categorize.Skip(r.Context(), id); err != nil {
		writeServiceError(w, r, err, "pergunta não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "skipped"})
}

func (s *Server) postInterpret(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	in, err := s.categorize.Interpret(r.Context(), id, body.Text)
	if err != nil {
		writeServiceError(w, r, err, "pergunta não encontrada")
		return
	}
	resp := map[string]any{"kind": in.Kind, "category_id": in.CategoryID, "category_path": nil, "alias": in.Alias}
	if in.CategoryID != nil {
		paths, err := subcategoryPaths(r.Context(), s.store.DB)
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		resp["category_path"] = paths[*in.CategoryID]
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) postSuggest(w http.ResponseWriter, r *http.Request) {
	n, err := s.categorize.Suggest(r.Context())
	if err != nil {
		writeServiceError(w, r, err, "não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"updated": n})
}
