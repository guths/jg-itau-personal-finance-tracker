package api

import (
	"database/sql"
	"net/http"
	"time"

	"jgfinance/internal/categorize"
)

type ruleJSON struct {
	ID               int64   `json:"id"`
	Key              string  `json:"key"`
	CategoryID       *int64  `json:"category_id"`
	CategoryPath     *string `json:"category_path"`
	KindOverride     *string `json:"kind_override"`
	Mode             string  `json:"mode"`
	Alias            *string `json:"alias"`
	Recurring        string  `json:"recurring"`
	Source           string  `json:"source"`
	TransactionCount int     `json:"transaction_count"`
	UpdatedAt        string  `json:"updated_at"`
}

// getRules lists the rules, most recently changed first; ?source=user|llm filters.
func (s *Server) getRules(w http.ResponseWriter, r *http.Request) {
	where, args := "", []any{}
	switch src := r.URL.Query().Get("source"); src {
	case "":
	case "user", "llm":
		where, args = "WHERE r.source = ?", append(args, src)
	default:
		writeError(w, http.StatusBadRequest, "source deve ser user ou llm")
		return
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT r.id, r.merchant_key, r.category_id, CONCAT(c.name, ' › ', s.name),
		r.kind_override, r.mode, r.alias, r.recurring, r.source, r.updated_at,
		(SELECT COUNT(*) FROM transactions t WHERE t.merchant_key = r.merchant_key)
		FROM rules r LEFT JOIN categories s ON s.id = r.category_id LEFT JOIN categories c ON c.id = s.parent_id
		`+where+` ORDER BY r.updated_at DESC, r.id DESC`, args...)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	defer rows.Close()
	out := []ruleJSON{}
	for rows.Next() {
		var x ruleJSON
		var cat sql.NullInt64
		var path, override, alias sql.NullString
		var updated time.Time
		if err := rows.Scan(&x.ID, &x.Key, &cat, &path, &override, &x.Mode, &alias, &x.Recurring, &x.Source, &updated,
			&x.TransactionCount); err != nil {
			writeInternal(w, r, err)
			return
		}
		x.CategoryID, x.CategoryPath, x.KindOverride, x.Alias = optInt64(cat), optString(path), optString(override), optString(alias)
		x.UpdatedAt = updated.UTC().Format(time.RFC3339)
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// patchRule edits a rule and reapplies it (see categorize.UpdateRule). kind_override "" clears it.
func (s *Server) patchRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		CategoryID   *int64  `json:"category_id"`
		KindOverride *string `json:"kind_override"`
		Mode         *string `json:"mode"`
		Alias        *string `json:"alias"`
		Recurring    *string `json:"recurring"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	err := s.categorize.UpdateRule(r.Context(), id, categorize.RuleUpdate{
		CategoryID: body.CategoryID, KindOverride: body.KindOverride, Mode: body.Mode, Alias: body.Alias, Recurring: body.Recurring,
	})
	if err != nil {
		writeServiceError(w, r, err, "regra não encontrada")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Server) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.categorize.DeleteRule(r.Context(), id); err != nil {
		writeServiceError(w, r, err, "regra não encontrada")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
