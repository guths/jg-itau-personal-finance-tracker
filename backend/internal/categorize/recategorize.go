package categorize

import (
	"context"
	"database/sql"
	"errors"
)

// transaction kinds a user may set on a single transaction.
var userKinds = map[string]bool{
	"expense": true, "refund": true, "bill_payment": true, "own_transfer": true,
	"investment": true, "income": true, "ignored": true,
}

// SetTransactionCategory recategorizes a transaction from the transactions table (spec §9.4).
// scope "one" changes only this transaction (categorized_by = user) and takes it out of an open
// question; scope "key" saves the choice as the key's rule (keeping the mode of an existing rule)
// and reapplies it to the key's transactions categorized by rule/llm and to those waiting in the
// key's open questions, which become answered (manual corrections are never overwritten).
func (c *Categorizer) SetTransactionCategory(ctx context.Context, txID int64, categoryID *int64, kind, scope string) error {
	if scope != "one" && scope != "key" {
		return ErrInvalidScope
	}
	return c.inTx(ctx, func(tx *sql.Tx) error {
		var key, curKind string
		err := tx.QueryRowContext(ctx, `SELECT merchant_key, kind FROM transactions WHERE id = ? FOR UPDATE`, txID).Scan(&key, &curKind)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		tax, err := loadTaxonomy(ctx, tx)
		if err != nil {
			return err
		}
		if scope == "one" {
			return c.setOne(ctx, tx, tax, txID, categoryID, kind)
		}
		ruleKind := kind
		if categoryID != nil && (kind == "expense" || kind == "refund") {
			ruleKind = "" // the sign decides between expense and refund
		}
		d, err := decide(tax, categoryID, ruleKind)
		if err != nil {
			return err
		}
		mode := "always"
		err = tx.QueryRowContext(ctx, `SELECT mode FROM rules WHERE merchant_key = ? FOR UPDATE`, key).Scan(&mode)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := upsertUserRule(ctx, tx, key, d, mode, ""); err != nil {
			return err
		}
		if err := c.apply(ctx, tx, []int64{txID}, d, "rule"); err != nil {
			return err
		}
		if err := c.reapply(ctx, tx, key, d); err != nil {
			return err
		}
		return c.resolveOpenQuestions(ctx, tx, key, 0, d)
	})
}

// setOne changes a single transaction and takes it out of its open question.
func (c *Categorizer) setOne(ctx context.Context, tx *sql.Tx, tax taxonomy, txID int64, categoryID *int64, kind string) error {
	if categoryID != nil {
		if !tax.valid(*categoryID) {
			return ErrInvalidCategory
		}
		var err error
		switch kind {
		case "":
			err = c.applyCategory(ctx, tx, []int64{txID}, *categoryID, "user")
		case "expense", "refund":
			_, err = tx.ExecContext(ctx, `UPDATE transactions SET category_id = ?, kind = ?, categorized_by = 'user' WHERE id = ?`,
				*categoryID, kind, txID)
		default:
			return ErrInvalidKind
		}
		if err != nil {
			return err
		}
		return detachFromOpenQuestion(ctx, tx, txID)
	}
	if !userKinds[kind] {
		return ErrInvalidKind
	}
	by := any("user")
	if kind == "expense" || kind == "refund" {
		by = nil // spending again, but still without a category
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET kind = ?, category_id = NULL, categorized_by = ? WHERE id = ?`, kind, by, txID); err != nil {
		return err
	}
	return detachFromOpenQuestion(ctx, tx, txID)
}

// detachFromOpenQuestion unlinks a transaction from its pending/skipped question; a question
// left without transactions is marked answered. Links to answered questions are history and stay.
func detachFromOpenQuestion(ctx context.Context, tx *sql.Tx, txID int64) error {
	var qid sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT q.id FROM transactions t JOIN questions q ON q.id = t.question_id
		WHERE t.id = ? AND q.status IN ('pending', 'skipped')`, txID).Scan(&qid)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET question_id = NULL WHERE id = ?`, txID); err != nil {
		return err
	}
	return closeIfEmpty(ctx, tx, qid.Int64)
}

// closeIfEmpty marks an open question answered when no transaction is linked to it anymore.
func closeIfEmpty(ctx context.Context, tx *sql.Tx, questionID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE questions SET status = 'answered', answered_at = UTC_TIMESTAMP()
		WHERE id = ? AND status IN ('pending', 'skipped')
		AND NOT EXISTS (SELECT 1 FROM transactions WHERE question_id = ?)`, questionID, questionID)
	return err
}

// reapply applies a rule decision to every transaction of key categorized by rule or llm.
func (c *Categorizer) reapply(ctx context.Context, tx *sql.Tx, key string, d decision) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM transactions WHERE merchant_key = ? AND categorized_by IN ('rule', 'llm')`, key)
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
	if err := rows.Err(); err != nil {
		return err
	}
	return c.apply(ctx, tx, ids, d, "rule")
}

// RuleUpdate is a partial change to a rule; nil fields are kept. Setting KindOverride to
// "own_transfer"/"ignored" clears the category; setting CategoryID clears the kind override.
type RuleUpdate struct {
	CategoryID   *int64
	KindOverride *string // "", "own_transfer" or "ignored"
	Mode         *string // "always" | "ask"
	Alias        *string
	Recurring    *string // "auto" | "yes" | "no"
}

// UpdateRule edits a rule (it becomes source = user) and reapplies it to the key's
// transactions categorized by rule/llm (spec §9.4). In mode "always" it also answers the key's
// open questions with the rule.
func (c *Categorizer) UpdateRule(ctx context.Context, ruleID int64, u RuleUpdate) error {
	return c.inTx(ctx, func(tx *sql.Tx) error {
		r, err := scanRule(tx.QueryRowContext(ctx, `SELECT `+ruleColumns+` FROM rules WHERE id = ? FOR UPDATE`, ruleID))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		tax, err := loadTaxonomy(ctx, tx)
		if err != nil {
			return err
		}
		if u.CategoryID != nil {
			r.CategoryID, r.KindOverride = u.CategoryID, ""
		}
		if u.KindOverride != nil {
			r.KindOverride = *u.KindOverride
			if r.KindOverride != "" {
				r.CategoryID = nil
			}
		}
		if u.Mode != nil {
			r.Mode = *u.Mode
		}
		if u.Alias != nil {
			r.Alias = clip(*u.Alias, 128)
		}
		if u.Recurring != nil {
			r.Recurring = *u.Recurring
		}
		d, err := decide(tax, r.CategoryID, r.KindOverride)
		if err != nil {
			return err
		}
		if (r.Mode != "always" && r.Mode != "ask") || (r.Recurring != "auto" && r.Recurring != "yes" && r.Recurring != "no") {
			return ErrInvalidRule
		}
		var cat, override any
		if d.kindOverride != "" {
			override = d.kindOverride
		} else {
			cat = d.categoryID
		}
		if _, err := tx.ExecContext(ctx, `UPDATE rules SET category_id = ?, kind_override = ?, mode = ?, alias = ?,
			recurring = ?, source = 'user' WHERE id = ?`, cat, override, r.Mode, nullString(r.Alias), r.Recurring, ruleID); err != nil {
			return err
		}
		if err := c.reapply(ctx, tx, r.Key, d); err != nil {
			return err
		}
		if r.Mode != "always" {
			return nil
		}
		return c.resolveOpenQuestions(ctx, tx, r.Key, 0, d)
	})
}

// DeleteRule removes a rule; transactions keep their categories.
func (c *Categorizer) DeleteRule(ctx context.Context, ruleID int64) error {
	r, err := c.store.DB.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, ruleID)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
