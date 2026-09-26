package importer

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	"jgfinance/internal/provider"
)

// SweepReversals applies provider.MarkReversals to the stored transactions of every document
// not yet evaluated (documents imported before reversal detection existed) and marks them
// evaluated, so it is idempotent and never undoes a later manual change. A row that becomes
// reversed loses its category, categorized_by and question; a pending/skipped question left
// without transactions is marked answered. Rows categorized by the user are reversed as well
// (a purchase cancelled by its refund is not spending), and each one is logged. It returns
// how many rows were reversed.
func (im *Importer) SweepReversals(ctx context.Context) (int, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	rows, err := im.store.DB.QueryContext(ctx, `SELECT id FROM documents WHERE NOT reversals_checked ORDER BY id`)
	if err != nil {
		return 0, err
	}
	var docs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		docs = append(docs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	total := 0
	for _, doc := range docs {
		n, err := im.sweepDocument(ctx, doc)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// storedSpending is a stored expense/refund row as the sweep sees it.
type storedSpending struct {
	id            int64
	key           string
	categoryID    sql.NullInt64
	categorizedBy sql.NullString
	questionID    sql.NullInt64
}

// sweepDocument evaluates one document in a single MySQL transaction.
func (im *Importer) sweepDocument(ctx context.Context, docID int64) (n int, err error) {
	tx, err := im.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	rows, err := tx.QueryContext(ctx, `SELECT id, seq, occurred_on, merchant_key, amount_cents, kind,
		category_id, categorized_by, question_id
		FROM transactions WHERE document_id = ? AND kind IN ('expense', 'refund') ORDER BY seq, id FOR UPDATE`, docID)
	if err != nil {
		return 0, err
	}
	var stored []storedSpending
	var txs []provider.Transaction
	for rows.Next() {
		var s storedSpending
		var t provider.Transaction
		var date time.Time
		var kind string
		if err = rows.Scan(&s.id, &t.Seq, &date, &s.key, &t.AmountCents, &kind,
			&s.categoryID, &s.categorizedBy, &s.questionID); err != nil {
			rows.Close()
			return 0, err
		}
		t.Date, t.Key, t.Kind = date, s.key, provider.TxKind(kind)
		stored = append(stored, s)
		txs = append(txs, t)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}

	provider.MarkReversals(txs)
	var ids []any
	questions := map[int64]bool{}
	for i, t := range txs {
		if t.Kind != provider.Reversed {
			continue
		}
		s := stored[i]
		ids = append(ids, s.id)
		if s.questionID.Valid {
			questions[s.questionID.Int64] = true
		}
		if s.categorizedBy.String == "user" {
			slog.Warn("estorno integral: lançamento categorizado por você deixou de contar como gasto",
				"transaction_id", s.id, "key", s.key, "category_id", s.categoryID.Int64)
		}
	}
	if len(ids) > 0 {
		in := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		if _, err = tx.ExecContext(ctx, `UPDATE transactions SET kind = 'reversed', category_id = NULL,
			categorized_by = NULL, question_id = NULL WHERE id IN (`+in+`)`, ids...); err != nil {
			return 0, err
		}
		for q := range questions {
			if _, err = tx.ExecContext(ctx, `UPDATE questions SET status = 'answered', answered_at = UTC_TIMESTAMP()
				WHERE id = ? AND status IN ('pending', 'skipped')
				AND NOT EXISTS (SELECT 1 FROM transactions WHERE question_id = ?)`, q, q); err != nil {
				return 0, err
			}
		}
		slog.Info("estornos integrais anulados", "document_id", docID, "transactions", len(ids))
	}
	if _, err = tx.ExecContext(ctx, `UPDATE documents SET reversals_checked = TRUE WHERE id = ?`, docID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}
