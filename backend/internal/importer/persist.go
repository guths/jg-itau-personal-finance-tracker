package importer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-sql-driver/mysql"

	"jgfinance/internal/provider"
)

// savedDocument is what persist reports back about a stored statement.
type savedDocument struct {
	accountName    string
	referenceMonth time.Time
}

func (im *Importer) documentExists(ctx context.Context, sha string) (bool, error) {
	var n int
	err := im.store.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE file_sha256 = ?`, sha).Scan(&n)
	return n > 0, err
}

// persist stores one parsed statement in a single MySQL transaction (spec §8.1 step 5) and
// returns the ids of the transactions that did not exist yet.
func (im *Importer) persist(ctx context.Context, f File, sha, providerID string, st *provider.Statement) (saved savedDocument, ids []int64, err error) {
	tx, err := im.store.DB.BeginTx(ctx, nil)
	if err != nil {
		return saved, nil, err
	}
	stored := filepath.Join("uploads", sha+".pdf")
	wroteFile := false
	defer func() {
		if err != nil {
			_ = tx.Rollback()
			var dup errDuplicate
			if wroteFile && !errors.As(err, &dup) { // on a duplicate the file belongs to the winner
				_ = os.Remove(filepath.Join(im.dataDir, stored))
			}
		}
	}()

	accountID, name, err := upsertAccount(ctx, tx, providerID, st.Account)
	if err != nil {
		return saved, nil, err
	}
	saved.accountName = name
	saved.referenceMonth = documentMonth(st)

	if st.Kind == provider.CardStatement {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE account_id = ? AND closing_date = ?`,
			accountID, st.ClosingDate).Scan(&n); err != nil {
			return saved, nil, err
		}
		if n > 0 {
			return saved, nil, errDuplicate{msgDuplicateInvoice}
		}
	}

	if err = im.saveFile(stored, f.Data); err != nil {
		return saved, nil, fmt.Errorf("gravar PDF: %w", err)
	}
	wroteFile = true

	docID, err := insertDocument(ctx, tx, accountID, f.Name, sha, stored, st)
	if err != nil {
		return saved, nil, err
	}
	if ids, err = insertTransactions(ctx, tx, docID, accountID, st); err != nil {
		return saved, nil, err
	}
	if st.Kind == provider.CardStatement {
		if err = insertInstallments(ctx, tx, docID, accountID, projectInstallments(documentMonth(st), st.Upcoming)); err != nil {
			return saved, nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return saved, nil, err
	}
	return saved, ids, nil
}

// upsertAccount finds or creates the account and returns its id and current name (which the
// user may have renamed).
func upsertAccount(ctx context.Context, tx *sql.Tx, providerID string, a provider.AccountRef) (int64, string, error) {
	_, err := tx.ExecContext(ctx, `INSERT INTO accounts (provider, kind, external_id, name) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE id = id`, providerID, string(a.Kind), a.ExternalID, clip(a.DefaultName, 128))
	if err != nil {
		return 0, "", err
	}
	var id int64
	var name string
	err = tx.QueryRowContext(ctx, `SELECT id, name FROM accounts WHERE provider = ? AND kind = ? AND external_id = ?`,
		providerID, string(a.Kind), a.ExternalID).Scan(&id, &name)
	return id, name, err
}

func insertDocument(ctx context.Context, tx *sql.Tx, accountID int64, name, sha, stored string, st *provider.Statement) (int64, error) {
	var total sql.NullInt64
	if st.Kind == provider.CardStatement {
		total = sql.NullInt64{Int64: st.TotalCents, Valid: true}
	}
	r, err := tx.ExecContext(ctx, `INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path,
		period_start, period_end, closing_date, due_date, reference_month, total_cents)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, string(st.Kind), sha, clip(name, 255), stored,
		nullDate(st.PeriodStart), nullDate(st.PeriodEnd), nullDate(st.ClosingDate), nullDate(st.DueDate),
		nullDate(documentMonth(st)), total)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == 1062 { // a concurrent upload of the same file or invoice won
			if st.Kind == provider.CardStatement {
				return 0, errDuplicate{msgDuplicateInvoice}
			}
			return 0, errDuplicate{msgDuplicateFile}
		}
		return 0, err
	}
	return r.LastInsertId()
}

func insertTransactions(ctx context.Context, tx *sql.Tx, docID, accountID int64, st *provider.Statement) ([]int64, error) {
	stmt, err := tx.PrepareContext(ctx, `INSERT IGNORE INTO transactions (document_id, account_id, seq, occurred_on,
		reference_month, description, merchant_key, amount_cents, kind, person_transfer, installment_n,
		installment_total, bank_category, city, holder, original_amount_cents, original_currency, dedupe_key)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	keys := dedupeKeys(accountID, st)
	var ids []int64
	for i, t := range st.Transactions {
		r, err := stmt.ExecContext(ctx, docID, accountID, t.Seq, t.Date, referenceMonth(st, t),
			clip(t.Description, 255), clip(t.Key, 191), t.AmountCents, string(t.Kind), t.PersonTransfer,
			nullInt(int64(t.InstallmentN)), nullInt(int64(t.InstallmentTotal)),
			nullString(t.BankCategory, 64), nullString(t.City, 64), nullString(t.Holder, 128),
			nullInt(t.OriginalAmountCents), nullString(t.OriginalCurrency, 3), keys[i])
		if err != nil {
			return nil, err
		}
		if n, err := r.RowsAffected(); err != nil {
			return nil, err
		} else if n == 0 {
			continue // already imported from an overlapping statement
		}
		id, err := r.LastInsertId()
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func insertInstallments(ctx context.Context, tx *sql.Tx, docID, accountID int64, items []projectedInstallment) error {
	for _, it := range items {
		if _, err := tx.ExecContext(ctx, `INSERT INTO installments (account_id, document_id, merchant_key, description,
			purchase_date, due_month, installment_n, installment_total, amount_cents) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			accountID, docID, clip(it.Key, 191), clip(it.Description, 255), nullDate(it.PurchaseDate),
			it.DueMonth, it.N, it.Total, it.AmountCents); err != nil {
			return err
		}
	}
	return nil
}

func nullDate(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullInt(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullString(s string, max int) any {
	if s == "" {
		return nil
	}
	return clip(s, max)
}

// clip truncates s to at most max characters (runes), matching VARCHAR limits.
func clip(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
