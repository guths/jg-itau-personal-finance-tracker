package categorize

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// pendingTx is a spending transaction that still needs a category.
type pendingTx struct {
	ID             int64
	Key            string
	Description    string
	AmountCents    int64
	Date           time.Time
	PersonTransfer bool
	BankCategory   string
	City           string
	AccountKind    string
}

// rule is a row of the rules table.
type rule struct {
	ID           int64
	Key          string
	CategoryID   *int64
	KindOverride string
	Mode         string
	Alias        string
	Recurring    string
	Source       string
}

// Run categorizes pending spending (spec §9.2): expense/refund transactions without category,
// without categorized_by and without question. Besides txIDs (the batch just imported) it sweeps
// every other pending transaction, so rows left behind by an interrupted run are picked up again.
// LLM failures never fail the run: the affected keys become questions without suggestions.
func (c *Categorizer) Run(ctx context.Context, txIDs []int64) (Result, error) {
	return c.sweep(ctx)
}

// sweep categorizes every pending transaction; runs are serialized so that concurrent sweeps
// (import and "sugerir de novo") never create duplicate questions.
func (c *Categorizer) sweep(ctx context.Context) (Result, error) {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	var res Result
	txs, err := c.loadPending(ctx)
	if err != nil || len(txs) == 0 {
		return res, err
	}
	keys := map[string]bool{}
	for _, t := range txs {
		keys[t.Key] = true
	}
	rules, err := c.loadRules(ctx, sortedKeys(keys))
	if err != nil {
		return res, err
	}
	open, err := c.openQuestions(ctx, sortedKeys(keys))
	if err != nil {
		return res, err
	}

	toLLM := map[string][]pendingTx{}
	for _, t := range txs {
		r, hasRule := rules[t.Key]
		switch {
		case hasRule && r.Mode == "ask": // asks again, suggesting the previous answer
			var sugg []int64
			if r.CategoryID != nil {
				sugg = []int64{*r.CategoryID}
			}
			if _, err := c.createQuestion(ctx, t.Key, true, sugg, "", []int64{t.ID}); err != nil {
				return res, err
			}
			res.Questions++
		case hasRule && r.KindOverride != "":
			if err := c.applyKind(ctx, c.store.DB, []int64{t.ID}, r.KindOverride, "rule"); err != nil {
				return res, err
			}
			res.Auto++
		case hasRule && r.CategoryID != nil:
			if err := c.applyCategory(ctx, c.store.DB, []int64{t.ID}, *r.CategoryID, "rule"); err != nil {
				return res, err
			}
			res.Auto++
		default:
			if qid, ok := open[t.Key]; ok { // the key is already waiting for an answer
				if _, err := c.store.DB.ExecContext(ctx, `UPDATE transactions SET question_id = ? WHERE id = ?`, qid, t.ID); err != nil {
					return res, err
				}
				continue
			}
			toLLM[t.Key] = append(toLLM[t.Key], t)
		}
	}
	if len(toLLM) == 0 {
		return res, nil
	}

	tax, err := loadTaxonomy(ctx, c.store.DB)
	if err != nil {
		return res, err
	}
	chunks, answers, errs := c.askBatches(ctx, tax, toLLM)
	for i, chunk := range chunks {
		if errs[i] != nil {
			slog.Warn("categorização: GPT indisponível; itens viram perguntas sem sugestão", "err", errs[i])
		}
		for _, key := range chunk {
			auto, q, err := c.resolveKey(ctx, tax, key, toLLM[key], answers[i][key])
			if err != nil {
				return res, err
			}
			res.Auto += auto
			res.Questions += q
		}
	}
	return res, nil
}

// askBatches splits the keys (sorted) into batches of batchSize and asks the LLM about them
// concurrently, at most llmConcurrency calls at a time. answers[i] and errs[i] belong to
// chunks[i].
func (c *Categorizer) askBatches(ctx context.Context, tax taxonomy, byKey map[string][]pendingTx) (chunks [][]string, answers []map[string]*llmItem, errs []error) {
	keys := sortedKeys(byKey)
	for start := 0; start < len(keys); start += batchSize {
		chunks = append(chunks, keys[start:min(start+batchSize, len(keys))])
	}
	answers = make([]map[string]*llmItem, len(chunks))
	errs = make([]error, len(chunks))
	sem := make(chan struct{}, llmConcurrency)
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		groups := make([][]pendingTx, len(chunk))
		for j, k := range chunk {
			groups[j] = byKey[k]
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			answers[i], errs[i] = c.askLLM(ctx, tax, groups)
		}()
	}
	wg.Wait()
	return chunks, answers, errs
}

// resolveKey applies a certain LLM answer or turns the key into one grouped question.
func (c *Categorizer) resolveKey(ctx context.Context, tax taxonomy, key string, group []pendingTx, item *llmItem) (auto, questions int, err error) {
	ids := make([]int64, len(group))
	person := false
	for i, t := range group {
		ids[i] = t.ID
		person = person || t.PersonTransfer
	}
	if item != nil && item.Certain && !person && tax.valid(item.SubcategoryID) {
		err = c.inTx(ctx, func(tx *sql.Tx) error {
			if err := c.applyCategory(ctx, tx, ids, item.SubcategoryID, "llm"); err != nil {
				return err
			}
			recurring := "auto"
			if item.LikelySubscription {
				recurring = "yes"
			}
			// Never clobbers a rule that appeared meanwhile.
			_, err := tx.ExecContext(ctx, `INSERT INTO rules (merchant_key, category_id, mode, alias, recurring, source)
				VALUES (?, ?, 'always', ?, ?, 'llm') ON DUPLICATE KEY UPDATE id = id`,
				key, item.SubcategoryID, nullString(clip(item.Alias, 128)), recurring)
			return err
		})
		return len(ids), 0, err
	}
	var sugg []int64
	prompt := ""
	if item != nil {
		sugg = suggestionsOf(tax, item)
		prompt = item.Question
	}
	_, err = c.createQuestion(ctx, key, false, sugg, prompt, ids)
	return 0, 1, err
}

// suggestionsOf lists up to 3 valid suggestions: the LLM's best guess first, then its ordered
// suggestions.
func suggestionsOf(tax taxonomy, it *llmItem) []int64 {
	return tax.validIDs(append([]int64{it.SubcategoryID}, it.Suggestions...), 3)
}

// askLLM returns the LLM answers by key, keeping only keys that were sent (first answer wins).
// It returns nil answers without error when there is no LLM.
func (c *Categorizer) askLLM(ctx context.Context, tax taxonomy, groups [][]pendingTx) (map[string]*llmItem, error) {
	if c.llm == nil {
		return nil, nil
	}
	examples, err := c.ruleExamples(ctx, tax)
	if err != nil {
		slog.Error("categorização: carregar exemplos", "err", err)
	}
	req, err := categorizeRequest(tax, examples, groups)
	if err != nil {
		return nil, err
	}
	var out llmBatch
	if err := c.llm.JSON(ctx, req, &out); err != nil {
		return nil, err
	}
	sent := map[string]bool{}
	for _, g := range groups {
		sent[g[0].Key] = true
	}
	answers := map[string]*llmItem{}
	for i := range out.Items {
		it := &out.Items[i]
		if !sent[it.Key] || answers[it.Key] != nil { // invented or repeated key
			continue
		}
		answers[it.Key] = it
	}
	return answers, nil
}

// loadPending returns every spending transaction still waiting for categorization.
func (c *Categorizer) loadPending(ctx context.Context) ([]pendingTx, error) {
	rows, err := c.store.DB.QueryContext(ctx, `SELECT t.id, t.merchant_key, t.description, t.amount_cents,
		t.occurred_on, t.person_transfer, COALESCE(t.bank_category, ''), COALESCE(t.city, ''), a.kind
		FROM transactions t JOIN accounts a ON a.id = t.account_id
		WHERE t.kind IN ('expense', 'refund')
		AND t.category_id IS NULL AND t.categorized_by IS NULL AND t.question_id IS NULL
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []pendingTx
	for rows.Next() {
		var t pendingTx
		if err := rows.Scan(&t.ID, &t.Key, &t.Description, &t.AmountCents, &t.Date, &t.PersonTransfer,
			&t.BankCategory, &t.City, &t.AccountKind); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

const ruleColumns = `id, merchant_key, category_id, COALESCE(kind_override, ''), mode, COALESCE(alias, ''), recurring, source`

func scanRule(s interface{ Scan(...any) error }) (rule, error) {
	var r rule
	var cat sql.NullInt64
	err := s.Scan(&r.ID, &r.Key, &cat, &r.KindOverride, &r.Mode, &r.Alias, &r.Recurring, &r.Source)
	if cat.Valid {
		r.CategoryID = &cat.Int64
	}
	return r, err
}

func (c *Categorizer) loadRules(ctx context.Context, keys []string) (map[string]rule, error) {
	out := map[string]rule{}
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := c.store.DB.QueryContext(ctx, `SELECT `+ruleColumns+` FROM rules WHERE merchant_key IN (`+
		placeholders(len(keys))+`)`, stringArgs(keys)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out[r.Key] = r
	}
	return out, rows.Err()
}

// openQuestions maps keys to their open grouped question (pending or skipped).
func (c *Categorizer) openQuestions(ctx context.Context, keys []string) (map[string]int64, error) {
	out := map[string]int64{}
	if len(keys) == 0 {
		return out, nil
	}
	rows, err := c.store.DB.QueryContext(ctx, `SELECT merchant_key, MIN(id) FROM questions
		WHERE status IN ('pending', 'skipped') AND NOT per_transaction AND merchant_key IN (`+
		placeholders(len(keys))+`) GROUP BY merchant_key`, stringArgs(keys)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var id int64
		if err := rows.Scan(&k, &id); err != nil {
			return nil, err
		}
		out[k] = id
	}
	return out, rows.Err()
}

// ruleExamples returns up to 40 recent user rules as "key → path (alias)" examples.
func (c *Categorizer) ruleExamples(ctx context.Context, tax taxonomy) ([]ruleExample, error) {
	rows, err := c.store.DB.QueryContext(ctx, `SELECT merchant_key, category_id, COALESCE(alias, '') FROM rules
		WHERE source = 'user' AND category_id IS NOT NULL ORDER BY updated_at DESC, id DESC LIMIT 40`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ruleExample
	for rows.Next() {
		var key, alias string
		var cat int64
		if err := rows.Scan(&key, &cat, &alias); err != nil {
			return nil, err
		}
		if path, ok := tax.byID[cat]; ok {
			out = append(out, ruleExample{Key: key, Category: path, Alias: alias})
		}
	}
	return out, rows.Err()
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// applyCategory sets the subcategory of the transactions; transactions that had been excluded
// (own_transfer/ignored) become spending again, by the sign of their amount.
func (c *Categorizer) applyCategory(ctx context.Context, db execer, ids []int64, categoryID int64, by string) error {
	if len(ids) == 0 {
		return nil
	}
	args := append([]any{categoryID, by}, int64Args(ids)...)
	_, err := db.ExecContext(ctx, `UPDATE transactions SET category_id = ?, categorized_by = ?,
		kind = CASE WHEN kind IN ('expense', 'refund') THEN kind WHEN amount_cents < 0 THEN 'refund' ELSE 'expense' END
		WHERE id IN (`+placeholders(len(ids))+`)`, args...)
	return err
}

// applyKind sets a non-spending kind (own_transfer/ignored) and clears the category.
func (c *Categorizer) applyKind(ctx context.Context, db execer, ids []int64, kind, by string) error {
	if len(ids) == 0 {
		return nil
	}
	args := append([]any{kind, by}, int64Args(ids)...)
	_, err := db.ExecContext(ctx, `UPDATE transactions SET kind = ?, category_id = NULL, categorized_by = ?
		WHERE id IN (`+placeholders(len(ids))+`)`, args...)
	return err
}

// createQuestion stores a question and links the transactions to it.
func (c *Categorizer) createQuestion(ctx context.Context, key string, perTx bool, suggestions []int64, prompt string, txIDs []int64) (int64, error) {
	var qid int64
	err := c.inTx(ctx, func(tx *sql.Tx) error {
		var sugg any
		if len(suggestions) > 0 {
			raw, _ := json.Marshal(suggestions)
			sugg = string(raw)
		}
		r, err := tx.ExecContext(ctx, `INSERT INTO questions (merchant_key, per_transaction, suggestions, prompt)
			VALUES (?, ?, ?, ?)`, key, perTx, sugg, nullString(clip(prompt, 512)))
		if err != nil {
			return err
		}
		if qid, err = r.LastInsertId(); err != nil {
			return err
		}
		args := append([]any{qid}, int64Args(txIDs)...)
		_, err = tx.ExecContext(ctx, `UPDATE transactions SET question_id = ? WHERE id IN (`+placeholders(len(txIDs))+`)`, args...)
		return err
	})
	return qid, err
}

func (c *Categorizer) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := c.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func int64Args(ids []int64) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func stringArgs(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// clip truncates s to at most max runes.
func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
