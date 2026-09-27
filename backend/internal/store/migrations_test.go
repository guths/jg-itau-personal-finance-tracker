package store_test

import (
	"context"
	"fmt"
	"io/fs"
	"testing"
	"time"

	"github.com/pressly/goose/v3"

	"jgfinance/internal/store"
	"jgfinance/internal/store/storetest"
)

// 00006 moves card lines to the month of their date (D1 changed on 2026-09-26), except
// installments 02/NN or later, which stay in the closing month; Down restores the closing month.
func TestMigrationCardReferenceMonthFollowsDate(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	sub, err := fs.Sub(store.Migrations, "migrations")
	must(t, err)
	p, err := goose.NewProvider(goose.DialectMySQL, s.DB, sub)
	must(t, err)
	t.Cleanup(func() { _, _ = p.Up(ctx) })

	exec := func(q string, args ...any) int64 {
		t.Helper()
		r, err := s.DB.ExecContext(ctx, q, args...)
		must(t, err)
		id, _ := r.LastInsertId()
		return id
	}
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	card := exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'credit_card', '8778', 'Cartão')`)
	bank := exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'checking', '1/2', 'Conta')`)
	invoice := exec(`INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path, closing_date, reference_month)
		VALUES (?, 'card_statement', 'a', 'f.pdf', 'p', '2026-09-24', '2026-09-01')`, card)
	extract := exec(`INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path, period_end, reference_month)
		VALUES (?, 'bank_statement', 'b', 'e.pdf', 'q', '2026-09-05', '2026-09-01')`, bank)
	seq := 0
	tx := func(doc, acc int64, date time.Time, month time.Time, n int) int64 {
		seq++
		var inst any
		if n > 0 {
			inst = n
		}
		return exec(`INSERT INTO transactions (document_id, account_id, seq, occurred_on, reference_month, description,
			merchant_key, amount_cents, kind, installment_n, dedupe_key) VALUES (?, ?, ?, ?, ?, 'X', 'X', 100, 'expense', ?, ?)`,
			doc, acc, seq, date, month, inst, fmt.Sprintf("dedupe-%d", seq))
	}
	sep, aug := day(2026, 9, 1), day(2026, 8, 1)
	previous := tx(invoice, card, day(2026, 8, 27), sep, 0)
	same := tx(invoice, card, day(2026, 9, 3), sep, 0)
	first := tx(invoice, card, day(2026, 8, 27), sep, 1)
	fourth := tx(invoice, card, day(2026, 6, 10), sep, 4)
	iof := tx(invoice, card, day(2026, 9, 24), sep, 0)
	pix := tx(extract, bank, day(2026, 8, 31), aug, 0)

	month := func(id int64) time.Time {
		t.Helper()
		var m time.Time
		must(t, s.DB.QueryRowContext(ctx, `SELECT reference_month FROM transactions WHERE id = ?`, id).Scan(&m))
		return m
	}
	check := func(stage string, want map[int64]time.Time) {
		t.Helper()
		for id, w := range want {
			if got := month(id); !got.Equal(w) {
				t.Errorf("%s: tx %d = %s, want %s", stage, id, got.Format("2006-01"), w.Format("2006-01"))
			}
		}
	}

	_, err = p.DownTo(ctx, 5)
	must(t, err)
	_, err = p.UpTo(ctx, 6)
	must(t, err)
	check("up", map[int64]time.Time{previous: aug, same: sep, first: aug, fourth: sep, iof: sep, pix: aug})

	_, err = p.DownTo(ctx, 5)
	must(t, err)
	check("down", map[int64]time.Time{previous: sep, same: sep, first: sep, fourth: sep, iof: sep, pix: aug})
}
