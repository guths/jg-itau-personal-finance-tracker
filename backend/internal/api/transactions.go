package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var txKinds = map[string]bool{
	"expense": true, "refund": true, "bill_payment": true, "own_transfer": true,
	"investment": true, "income": true, "ignored": true, "reversed": true,
}

// maxTransactions caps GET /api/transactions (filters narrow it down).
const maxTransactions = 5000

type transactionJSON struct {
	ID                  int64   `json:"id"`
	DocumentID          int64   `json:"document_id"`
	AccountID           int64   `json:"account_id"`
	AccountName         string  `json:"account_name"`
	Date                Date    `json:"date"`
	ReferenceMonth      Month   `json:"reference_month"`
	Description         string  `json:"description"`
	Key                 string  `json:"key"`
	AmountCents         int64   `json:"amount_cents"`
	Kind                string  `json:"kind"`
	PersonTransfer      bool    `json:"person_transfer"`
	CategoryID          *int64  `json:"category_id"`
	CategoryPath        *string `json:"category_path"`
	CategoryColor       *string `json:"category_color"`
	CategorizedBy       *string `json:"categorized_by"`
	QuestionID          *int64  `json:"question_id"`
	InstallmentN        *int    `json:"installment_n"`
	InstallmentTotal    *int    `json:"installment_total"`
	BankCategory        *string `json:"bank_category"`
	City                *string `json:"city"`
	Holder              *string `json:"holder"`
	OriginalAmountCents *int64  `json:"original_amount_cents"`
	OriginalCurrency    *string `json:"original_currency"`
}

// getTransactions filters by month (AAAA-MM), account_id, category_id (a category includes its
// subcategories), kind, q (text in description or key), uncategorized=true (spending without
// category) and categorized_by (rule|llm|user).
func (s *Server) getTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	where := []string{"1 = 1"}
	var args []any
	if v := q.Get("month"); v != "" {
		m, err := parseMonth(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		where, args = append(where, "t.reference_month = ?"), append(args, m)
	}
	for _, f := range []struct{ param, col string }{{"account_id", "t.account_id"}, {"category_id", ""}} {
		v := q.Get(f.param)
		if v == "" {
			continue
		}
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, f.param+" inválido")
			return
		}
		if f.col != "" {
			where, args = append(where, f.col+" = ?"), append(args, id)
		} else {
			where, args = append(where, "(t.category_id = ? OR s.parent_id = ?)"), append(args, id, id)
		}
	}
	if v := q.Get("kind"); v != "" {
		if !txKinds[v] {
			writeError(w, http.StatusBadRequest, "kind inválido")
			return
		}
		where, args = append(where, "t.kind = ?"), append(args, v)
	}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(v) + "%"
		where, args = append(where, "(t.description LIKE ? OR t.merchant_key LIKE ? OR r.alias LIKE ?)"), append(args, like, like, like)
	}
	switch q.Get("uncategorized") {
	case "", "false", "0":
	case "true", "1":
		where = append(where, "t.category_id IS NULL AND t.kind IN ('expense', 'refund')")
	default:
		writeError(w, http.StatusBadRequest, "uncategorized deve ser true ou false")
		return
	}
	if v := q.Get("categorized_by"); v != "" {
		if v != "rule" && v != "llm" && v != "user" {
			writeError(w, http.StatusBadRequest, "categorized_by deve ser rule, llm ou user")
			return
		}
		where, args = append(where, "t.categorized_by = ?"), append(args, v)
	}
	rows, err := s.store.DB.QueryContext(r.Context(), `SELECT t.id, t.document_id, t.account_id, a.name, t.occurred_on,
		t.reference_month, t.description, t.merchant_key, t.amount_cents, t.kind, t.person_transfer, t.category_id,
		CONCAT(c.name, ' › ', s.name), c.color, t.categorized_by, t.question_id, t.installment_n, t.installment_total,
		t.bank_category, t.city, t.holder, t.original_amount_cents, t.original_currency
		FROM transactions t JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories s ON s.id = t.category_id LEFT JOIN categories c ON c.id = s.parent_id
		LEFT JOIN rules r ON r.merchant_key = t.merchant_key
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY t.occurred_on DESC, t.id DESC LIMIT `+strconv.Itoa(maxTransactions), args...)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	defer rows.Close()
	out := []transactionJSON{}
	for rows.Next() {
		var t transactionJSON
		var when, month time.Time
		var cat, qid, orig sql.NullInt64
		var path, color, by, bankCat, city, holder, cur sql.NullString
		var n, total sql.NullInt32
		if err := rows.Scan(&t.ID, &t.DocumentID, &t.AccountID, &t.AccountName, &when, &month, &t.Description, &t.Key,
			&t.AmountCents, &t.Kind, &t.PersonTransfer, &cat, &path, &color, &by, &qid, &n, &total,
			&bankCat, &city, &holder, &orig, &cur); err != nil {
			writeInternal(w, r, err)
			return
		}
		t.Date, t.ReferenceMonth = Date(when), Month(month)
		t.CategoryID, t.QuestionID, t.OriginalAmountCents = optInt64(cat), optInt64(qid), optInt64(orig)
		t.CategoryPath, t.CategoryColor, t.CategorizedBy = optString(path), optString(color), optString(by)
		t.BankCategory, t.City, t.Holder, t.OriginalCurrency = optString(bankCat), optString(city), optString(holder), optString(cur)
		t.InstallmentN, t.InstallmentTotal = optInt(n), optInt(total)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func optInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}

func optInt(v sql.NullInt32) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int32)
	return &n
}

func optString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}

// patchTransaction recategorizes a transaction: {category_id?, kind?, scope: "one"|"key"}.
func (s *Server) patchTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		CategoryID *int64 `json:"category_id"`
		Kind       string `json:"kind"`
		Scope      string `json:"scope"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.CategoryID == nil && body.Kind == "" {
		writeError(w, http.StatusBadRequest, "informe category_id ou kind")
		return
	}
	if err := s.categorize.SetTransactionCategory(r.Context(), id, body.CategoryID, body.Kind, body.Scope); err != nil {
		writeServiceError(w, r, err, "lançamento não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}
