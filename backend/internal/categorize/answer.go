package categorize

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"jgfinance/internal/money"
)

// Answer is the user's answer to a question (spec §9.4). Either CategoryID (a subcategory,
// with Kind "" or "expense") or Kind "own_transfer"/"ignored".
type Answer struct {
	CategoryID *int64
	Kind       string
	Remember   bool // true: rule mode "always"; false: rule mode "ask"
	Alias      string
}

// Interpretation is the LLM's reading of a free-text answer; the UI confirms it before it is
// sent back as an Answer.
type Interpretation struct {
	Kind       string
	CategoryID *int64
	Alias      string
}

// decision is a validated category-or-kind choice.
type decision struct {
	categoryID   int64
	kindOverride string // own_transfer | ignored; empty when categoryID is set
}

func decide(tax taxonomy, categoryID *int64, kind string) (decision, error) {
	switch kind {
	case "", "expense":
		if categoryID == nil || !tax.valid(*categoryID) {
			return decision{}, ErrInvalidCategory
		}
		return decision{categoryID: *categoryID}, nil
	case "own_transfer", "ignored":
		return decision{kindOverride: kind}, nil
	}
	return decision{}, ErrInvalidKind
}

// apply sets the decision on the transactions.
func (c *Categorizer) apply(ctx context.Context, db execer, ids []int64, d decision, by string) error {
	if d.kindOverride != "" {
		return c.applyKind(ctx, db, ids, d.kindOverride, by)
	}
	return c.applyCategory(ctx, db, ids, d.categoryID, by)
}

// upsertUserRule creates or replaces the rule of key with the user's decision. An empty alias
// keeps the current one; recurring is preserved.
func upsertUserRule(ctx context.Context, db execer, key string, d decision, mode, alias string) error {
	var cat, override any
	if d.kindOverride != "" {
		override = d.kindOverride
	} else {
		cat = d.categoryID
	}
	_, err := db.ExecContext(ctx, `INSERT INTO rules (merchant_key, category_id, kind_override, mode, alias, source)
		VALUES (?, ?, ?, ?, ?, 'user') AS new
		ON DUPLICATE KEY UPDATE category_id = new.category_id, kind_override = new.kind_override,
		mode = new.mode, alias = COALESCE(new.alias, rules.alias), source = 'user'`,
		key, cat, override, mode, nullString(clip(alias, 128)))
	return err
}

type questionRow struct {
	ID      int64
	Key     string
	PerTx   bool
	Status  string
	Prompt  string
	Suggest []int64
	TxIDs   []int64
}

func loadQuestion(ctx context.Context, q interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, id int64, forUpdate bool) (questionRow, error) {
	var r questionRow
	var sugg sql.NullString
	var prompt sql.NullString
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, `SELECT id, merchant_key, per_transaction, status, prompt, suggestions FROM questions WHERE id = ?`+lock, id).
		Scan(&r.ID, &r.Key, &r.PerTx, &r.Status, &prompt, &sugg)
	if errors.Is(err, sql.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	r.Prompt = prompt.String
	if sugg.Valid {
		_ = json.Unmarshal([]byte(sugg.String), &r.Suggest)
	}
	rows, err := q.QueryContext(ctx, `SELECT id FROM transactions WHERE question_id = ? ORDER BY occurred_on, id`, id)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var tid int64
		if err := rows.Scan(&tid); err != nil {
			return r, err
		}
		r.TxIDs = append(r.TxIDs, tid)
	}
	return r, rows.Err()
}

// Answer applies the answer to every transaction of the question, saves the rule and marks the
// question answered (spec §9.4). With Remember, other open questions of the same key are
// resolved by the new rule as well.
func (c *Categorizer) Answer(ctx context.Context, questionID int64, a Answer) error {
	return c.inTx(ctx, func(tx *sql.Tx) error {
		q, err := loadQuestion(ctx, tx, questionID, true)
		if err != nil {
			return err
		}
		if q.Status == "answered" {
			return ErrAlreadyAnswered
		}
		tax, err := loadTaxonomy(ctx, tx)
		if err != nil {
			return err
		}
		d, err := decide(tax, a.CategoryID, a.Kind)
		if err != nil {
			return err
		}
		if err := c.apply(ctx, tx, q.TxIDs, d, "user"); err != nil {
			return err
		}
		mode := "ask"
		if a.Remember {
			mode = "always"
		}
		if err := upsertUserRule(ctx, tx, q.Key, d, mode, a.Alias); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE questions SET status = 'answered', answered_at = UTC_TIMESTAMP() WHERE id = ?`, q.ID); err != nil {
			return err
		}
		if !a.Remember {
			return nil
		}
		return c.resolveSiblings(ctx, tx, q, d)
	})
}

// resolveSiblings applies a remembered answer to the other open questions of the same key.
func (c *Categorizer) resolveSiblings(ctx context.Context, tx *sql.Tx, q questionRow, d decision) error {
	rows, err := tx.QueryContext(ctx, `SELECT t.id FROM transactions t JOIN questions q ON q.id = t.question_id
		WHERE q.merchant_key = ? AND q.id <> ? AND q.status IN ('pending', 'skipped')`, q.Key, q.ID)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := c.apply(ctx, tx, ids, d, "rule"); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE questions SET status = 'answered', answered_at = UTC_TIMESTAMP()
		WHERE merchant_key = ? AND id <> ? AND status IN ('pending', 'skipped')`, q.Key, q.ID)
	return err
}

// Skip moves a pending question to the end of the queue.
func (c *Categorizer) Skip(ctx context.Context, questionID int64) error {
	q, err := loadQuestion(ctx, c.store.DB, questionID, false)
	if err != nil {
		return err
	}
	switch q.Status {
	case "answered":
		return ErrAlreadyAnswered
	case "skipped":
		return nil
	}
	_, err = c.store.DB.ExecContext(ctx, `UPDATE questions SET status = 'skipped' WHERE id = ? AND status = 'pending'`, questionID)
	return err
}

// Interpret asks the LLM what a free-text answer means (spec §9.3). Nothing is saved.
func (c *Categorizer) Interpret(ctx context.Context, questionID int64, text string) (Interpretation, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Interpretation{}, ErrEmptyText
	}
	q, err := loadQuestion(ctx, c.store.DB, questionID, false)
	if err != nil {
		return Interpretation{}, err
	}
	if c.llm == nil {
		return Interpretation{}, ErrNoLLM
	}
	tax, err := loadTaxonomy(ctx, c.store.DB)
	if err != nil {
		return Interpretation{}, err
	}
	qctx, err := c.questionContext(ctx, q, tax)
	if err != nil {
		return Interpretation{}, err
	}
	req, err := interpretRequest(tax, qctx, text)
	if err != nil {
		return Interpretation{}, err
	}
	var out interpretation
	if err := c.llm.JSON(ctx, req, &out); err != nil {
		return Interpretation{}, fmt.Errorf("%w: %v", ErrLLMFailed, err)
	}
	res := Interpretation{Kind: out.Kind, Alias: strings.TrimSpace(out.Alias)}
	switch out.Kind {
	case "expense":
		if out.SubcategoryID != nil && tax.valid(*out.SubcategoryID) {
			id := *out.SubcategoryID
			res.CategoryID = &id
		}
	case "own_transfer", "ignored":
	default:
		return Interpretation{}, fmt.Errorf("%w: tipo %q", ErrLLMFailed, out.Kind)
	}
	return res, nil
}

// questionContext describes a question to the LLM (descriptions, amounts, dates only).
func (c *Categorizer) questionContext(ctx context.Context, q questionRow, tax taxonomy) (map[string]any, error) {
	rows, err := c.store.DB.QueryContext(ctx, `SELECT description, amount_cents, occurred_on,
		COALESCE(bank_category, ''), COALESCE(city, '') FROM transactions WHERE question_id = ? ORDER BY occurred_on, id`, q.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var descs, dates []string
	var total int64
	var bankCat, city string
	n := 0
	for rows.Next() {
		var d, bc, ct string
		var amount int64
		var when time.Time
		if err := rows.Scan(&d, &amount, &when, &bc, &ct); err != nil {
			return nil, err
		}
		n++
		total += amount
		if len(descs) < 3 {
			descs = append(descs, d)
		}
		if len(dates) < 5 {
			dates = append(dates, when.Format("2006-01-02"))
		}
		bankCat, city = bc, ct
	}
	var sugg []string
	for _, id := range q.Suggest {
		if p, ok := tax.byID[id]; ok {
			sugg = append(sugg, p)
		}
	}
	return map[string]any{
		"key": q.Key, "descriptions": descs, "count": n, "total": money.FormatBRL(total), "dates": dates,
		"bank_category": bankCat, "city": city, "question": q.Prompt, "suggestions": sugg,
	}, rows.Err()
}

// Suggest first sweeps pending transactions left without category nor question (e.g. by an
// interrupted import), then asks the LLM again for open grouped questions that have no
// suggestions (e.g. created while the LLM was unavailable) and stores the new suggestions and
// question text. It never categorizes a question by itself. It returns how many questions got
// suggestions.
func (c *Categorizer) Suggest(ctx context.Context) (int, error) {
	if _, err := c.sweep(ctx); err != nil {
		return 0, err
	}
	if c.llm == nil {
		return 0, ErrNoLLM
	}
	rows, err := c.store.DB.QueryContext(ctx, `SELECT q.id, q.merchant_key, t.id, t.description, t.amount_cents,
		t.occurred_on, t.person_transfer, COALESCE(t.bank_category, ''), COALESCE(t.city, ''), a.kind
		FROM questions q JOIN transactions t ON t.question_id = q.id JOIN accounts a ON a.id = t.account_id
		WHERE q.status IN ('pending', 'skipped') AND NOT q.per_transaction
		AND (q.suggestions IS NULL OR JSON_LENGTH(q.suggestions) = 0)
		ORDER BY q.id, t.occurred_on, t.id`)
	if err != nil {
		return 0, err
	}
	byKey := map[string][]pendingTx{}
	qidByKey := map[string]int64{}
	for rows.Next() {
		var qid int64
		var t pendingTx
		if err := rows.Scan(&qid, &t.Key, &t.ID, &t.Description, &t.AmountCents, &t.Date, &t.PersonTransfer,
			&t.BankCategory, &t.City, &t.AccountKind); err != nil {
			rows.Close()
			return 0, err
		}
		if prev, ok := qidByKey[t.Key]; ok && prev != qid {
			continue // one question per key is enough to ask about
		}
		qidByKey[t.Key] = qid
		byKey[t.Key] = append(byKey[t.Key], t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(byKey) == 0 {
		return 0, nil
	}
	tax, err := loadTaxonomy(ctx, c.store.DB)
	if err != nil {
		return 0, err
	}
	chunks, answers, errs := c.askBatches(ctx, tax, byKey)
	updated := 0
	var llmErr error
	for i, chunk := range chunks {
		if errs[i] != nil {
			llmErr = errs[i]
			continue
		}
		for _, k := range chunk {
			it := answers[i][k]
			if it == nil {
				continue
			}
			sugg := suggestionsOf(tax, it)
			if len(sugg) == 0 {
				continue
			}
			raw, _ := json.Marshal(sugg)
			if _, err := c.store.DB.ExecContext(ctx, `UPDATE questions SET suggestions = ?, prompt = COALESCE(?, prompt) WHERE id = ?`,
				string(raw), nullString(clip(it.Question, 512)), qidByKey[k]); err != nil {
				return updated, err
			}
			updated++
		}
	}
	if llmErr != nil {
		return updated, fmt.Errorf("%w: %v", ErrLLMFailed, llmErr)
	}
	return updated, nil
}
