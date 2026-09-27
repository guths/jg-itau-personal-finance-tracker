package importer

import (
	"context"
	"fmt"
	"testing"
	"time"

	"jgfinance/internal/categorize"
	"jgfinance/internal/insights"
	"jgfinance/internal/provider"
	"jgfinance/internal/store"
)

const ali = "ALIEXPRESSSAO PAULOBR"

// aliStatement is a card statement with the real AliExpress case (a purchase and two refunds on
// 22/09 that cancel out) and one ordinary purchase.
func aliStatement() *provider.Statement {
	st := cardStatement()
	st.ClosingDate, st.DueDate = d(2026, 9, 24), d(2026, 10, 1)
	st.Upcoming = nil
	st.Transactions = []provider.Transaction{
		{Seq: 1, Date: d(2026, 9, 22), Description: "ALIEXPRESSSAO PAULOBR", Key: ali, AmountCents: 41986, Kind: provider.Expense},
		{Seq: 2, Date: d(2026, 9, 22), Description: "ALIEXPRESSSAO PAULOBR", Key: ali, AmountCents: -1742, Kind: provider.Refund},
		{Seq: 3, Date: d(2026, 9, 22), Description: "ALIEXPRESSSAO PAULOBR", Key: ali, AmountCents: -40244, Kind: provider.Refund},
		{Seq: 4, Date: d(2026, 9, 23), Description: "PADARIA EXEMPLO", Key: "PADARIA EXEMPLO", AmountCents: 1250, Kind: provider.Expense},
	}
	return st
}

type storedTx struct {
	Kind       string
	CategoryID int64
	By         string
	QuestionID int64
}

func storedBySeq(t *testing.T, s *store.Store, docID int64) map[int]storedTx {
	t.Helper()
	rows, err := s.DB.Query(`SELECT seq, kind, COALESCE(category_id, 0), COALESCE(categorized_by, ''), COALESCE(question_id, 0)
		FROM transactions WHERE document_id = ?`, docID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[int]storedTx{}
	for rows.Next() {
		var seq int
		var st storedTx
		if err := rows.Scan(&seq, &st.Kind, &st.CategoryID, &st.By, &st.QuestionID); err != nil {
			t.Fatal(err)
		}
		out[seq] = st
	}
	return out
}

func TestImportStoresReversalsWithoutQuestions(t *testing.T) {
	e := newEnv(t, aliStatement())
	e.im.categorizer = categorize.New(e.st, nil) // no AI: every pending item becomes a question
	r := importOne(t, e, "fatura.pdf", pdf(t, 0))
	if r.Status != StatusImported || r.NewCount != 4 {
		t.Fatalf("result = %+v", r)
	}
	var docID int64
	var checked bool
	if err := e.st.DB.QueryRow(`SELECT id, reversals_checked FROM documents`).Scan(&docID, &checked); err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Error("imported document should be marked reversals_checked")
	}
	got := storedBySeq(t, e.st, docID)
	for seq := 1; seq <= 3; seq++ {
		if got[seq] != (storedTx{Kind: "reversed"}) {
			t.Errorf("seq %d = %+v, want reversed without category or question", seq, got[seq])
		}
	}
	if got[4].Kind != "expense" || got[4].QuestionID == 0 {
		t.Errorf("seq 4 = %+v, want an expense waiting in a question", got[4])
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM questions`); n != 1 {
		t.Errorf("questions = %d, want 1 (only the bakery)", n)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM questions WHERE merchant_key = ?`, ali); n != 0 {
		t.Errorf("AliExpress got %d questions", n)
	}
}

// sweepFx builds documents imported before reversal detection existed.
type sweepFx struct {
	t    *testing.T
	s    *store.Store
	card int64
	bank int64
	n    int
}

func newSweepFx(t *testing.T) (*sweepFx, *Importer) {
	t.Helper()
	e := newEnv(t, cardStatement())
	f := &sweepFx{t: t, s: e.st}
	f.card = f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'credit_card', '1234', 'Cartão')`)
	f.bank = f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'checking', '1/2', 'Conta')`)
	return f, e.im
}

func (f *sweepFx) exec(q string, args ...any) int64 {
	f.t.Helper()
	r, err := f.s.DB.Exec(q, args...)
	if err != nil {
		f.t.Fatal(err)
	}
	id, _ := r.LastInsertId()
	return id
}

func (f *sweepFx) doc(acc int64, kind string) int64 {
	f.n++
	return f.exec(`INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path) VALUES (?, ?, ?, 'x.pdf', 'p')`,
		acc, kind, fmt.Sprintf("sha-%d", f.n))
}

func (f *sweepFx) tx(doc, acc int64, seq int, date time.Time, key string, cents int64, kind string) int64 {
	f.n++
	return f.exec(`INSERT INTO transactions (document_id, account_id, seq, occurred_on, reference_month, description,
		merchant_key, amount_cents, kind, dedupe_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		doc, acc, seq, date, monthOf(date), key, key, cents, kind, fmt.Sprintf("dk-%d", f.n))
}

func (f *sweepFx) question(key string, txIDs ...int64) int64 {
	q := f.exec(`INSERT INTO questions (merchant_key) VALUES (?)`, key)
	for _, id := range txIDs {
		f.exec(`UPDATE transactions SET question_id = ? WHERE id = ?`, q, id)
	}
	return q
}

func (f *sweepFx) status(q int64) string {
	f.t.Helper()
	var s string
	if err := f.s.DB.QueryRow(`SELECT status FROM questions WHERE id = ?`, q).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *sweepFx) state(id int64) storedTx {
	f.t.Helper()
	var st storedTx
	if err := f.s.DB.QueryRow(`SELECT kind, COALESCE(category_id, 0), COALESCE(categorized_by, ''), COALESCE(question_id, 0)
		FROM transactions WHERE id = ?`, id).Scan(&st.Kind, &st.CategoryID, &st.By, &st.QuestionID); err != nil {
		f.t.Fatal(err)
	}
	return st
}

func TestSweepReversalsFixesStoredDocuments(t *testing.T) {
	f, im := newSweepFx(t)
	ctx := context.Background()
	var sub int64
	if err := f.s.DB.QueryRow(`SELECT id FROM categories WHERE parent_id IS NOT NULL ORDER BY id LIMIT 1`).Scan(&sub); err != nil {
		t.Fatal(err)
	}

	// Card invoice: the AliExpress trio waits in one question, a purchase + refund pair of the
	// same key on another day stays, and an unrelated purchase shares nothing.
	card := f.doc(f.card, "card_statement")
	a1 := f.tx(card, f.card, 1, d(2026, 9, 22), ali, 41986, "expense")
	a2 := f.tx(card, f.card, 2, d(2026, 9, 22), ali, -1742, "refund")
	a3 := f.tx(card, f.card, 3, d(2026, 9, 22), ali, -40244, "refund")
	a4 := f.tx(card, f.card, 4, d(2026, 9, 21), ali, 5000, "expense")
	qAli := f.question(ali, a1, a2, a3, a4)
	lojaBig := f.tx(card, f.card, 5, d(2026, 9, 10), "LOJA", 5000, "expense")
	lojaSmall := f.tx(card, f.card, 6, d(2026, 9, 10), "LOJA", 3000, "expense")
	lojaRefund := f.tx(card, f.card, 7, d(2026, 9, 10), "LOJA", -3000, "refund")
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'llm' WHERE id IN (?, ?, ?)`, sub, lojaBig, lojaSmall, lojaRefund)

	// Bank statement: the PIX was categorized by hand, its refund waits alone in a question.
	bank := f.doc(f.bank, "bank_statement")
	pix := f.tx(bank, f.bank, 1, d(2026, 9, 2), "PIX QRS DAL SOTO SO", 4200, "expense")
	dev := f.tx(bank, f.bank, 2, d(2026, 9, 2), "DEV PIX DAL SOTO SO", -4200, "refund")
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'user' WHERE id = ?`, sub, pix)
	qDev := f.question("DEV PIX DAL SOTO SO", dev)
	salary := f.tx(bank, f.bank, 3, d(2026, 9, 2), "PIX TRANSF DAL SOTO SO", -4200, "income")

	n, err := im.SweepReversals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Errorf("reversed = %d, want 7", n)
	}
	for _, id := range []int64{a1, a2, a3, lojaSmall, lojaRefund, pix, dev} {
		if got := f.state(id); got != (storedTx{Kind: "reversed"}) {
			t.Errorf("tx %d = %+v, want reversed without category or question", id, got)
		}
	}
	if got := f.state(a4); got.Kind != "expense" || got.QuestionID != qAli {
		t.Errorf("AliExpress of another day = %+v, want expense still in question %d", got, qAli)
	}
	if got := f.state(lojaBig); got.Kind != "expense" || got.CategoryID != sub || got.By != "llm" {
		t.Errorf("unpaired purchase = %+v", got)
	}
	if got := f.state(salary); got.Kind != "income" {
		t.Errorf("income = %+v", got)
	}
	if s := f.status(qAli); s != "pending" {
		t.Errorf("AliExpress question = %s, want pending (still has a transaction)", s)
	}
	if s := f.status(qDev); s != "answered" {
		t.Errorf("DEV PIX question = %s, want answered (no transaction left)", s)
	}
	if n := count(t, f.s, `SELECT COUNT(*) FROM documents WHERE NOT reversals_checked`); n != 0 {
		t.Errorf("%d documents not checked", n)
	}

	// Idempotent: nothing left to do, even when every document is evaluated again.
	if n, err := im.SweepReversals(ctx); err != nil || n != 0 {
		t.Errorf("second sweep = %d, %v", n, err)
	}
	f.exec(`UPDATE documents SET reversals_checked = FALSE`)
	if n, err := im.SweepReversals(ctx); err != nil || n != 0 {
		t.Errorf("sweep after reset = %d, %v", n, err)
	}
}

func TestSweepReversalsRespectsCheckedDocuments(t *testing.T) {
	f, im := newSweepFx(t)
	card := f.doc(f.card, "card_statement")
	f.exec(`UPDATE documents SET reversals_checked = TRUE WHERE id = ?`, card)
	// The user turned a reversed purchase back into spending after the document was checked.
	buy := f.tx(card, f.card, 1, d(2026, 9, 22), ali, 41986, "expense")
	back := f.tx(card, f.card, 2, d(2026, 9, 22), ali, -41986, "reversed")
	if n, err := im.SweepReversals(context.Background()); err != nil || n != 0 {
		t.Fatalf("sweep = %d, %v", n, err)
	}
	if got := f.state(buy); got.Kind != "expense" {
		t.Errorf("purchase = %+v, want the user's choice kept", got)
	}
	if got := f.state(back); got.Kind != "reversed" {
		t.Errorf("refund = %+v", got)
	}
}

func TestImportOverlappingStatementReversesStoredPayment(t *testing.T) {
	pay := provider.Transaction{Date: d(2026, 9, 2), Description: "PIX QRS DAL SOTO SO02/09", Key: "PIX QRS DAL SOTO SO", AmountCents: 4200, Kind: provider.Expense}
	dev := provider.Transaction{Date: d(2026, 9, 2), Description: "DEV PIX DAL SOTO SO02/09", Key: "DEV PIX DAL SOTO SO", AmountCents: -4200, Kind: provider.Refund}
	coffee := provider.Transaction{Date: d(2026, 9, 1), Description: "CAFE", Key: "CAFE", AmountCents: 800, Kind: provider.Expense}

	// Statement A was exported before the refund: the payment is stored and waits in a question.
	e := newEnv(t, bankStatement(coffee, pay))
	e.im.categorizer = categorize.New(e.st, nil)
	if r := importOne(t, e, "extrato-a.pdf", pdf(t, 0)); r.Status != StatusImported || r.NewCount != 2 {
		t.Fatalf("A = %+v", r)
	}
	var payID, qid int64
	if err := e.st.DB.QueryRow(`SELECT id, COALESCE(question_id, 0) FROM transactions WHERE merchant_key = 'PIX QRS DAL SOTO SO'`).Scan(&payID, &qid); err != nil {
		t.Fatal(err)
	}
	if qid == 0 {
		t.Fatal("payment should wait in a question after A")
	}

	// Statement B overlaps A and has the payment again plus its refund.
	e.prov.st = bankStatement(coffee, pay, dev)
	if r := importOne(t, e, "extrato-b.pdf", pdf(t, 1)); r.Status != StatusImported || r.NewCount != 1 || r.ExistingCount != 2 {
		t.Fatalf("B = %+v", r)
	}
	rows, err := e.st.DB.Query(`SELECT merchant_key, kind, COALESCE(category_id, 0), COALESCE(categorized_by, ''), COALESCE(question_id, 0)
		FROM transactions WHERE merchant_key LIKE '%DAL SOTO%'`)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for rows.Next() {
		var key string
		var st storedTx
		if err := rows.Scan(&key, &st.Kind, &st.CategoryID, &st.By, &st.QuestionID); err != nil {
			t.Fatal(err)
		}
		n++
		if st != (storedTx{Kind: "reversed"}) {
			t.Errorf("%s = %+v, want reversed without category or question", key, st)
		}
	}
	rows.Close()
	if n != 2 {
		t.Errorf("DAL SOTO rows = %d, want 2", n)
	}
	var status string
	if err := e.st.DB.QueryRow(`SELECT status FROM questions WHERE id = ?`, qid).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "answered" {
		t.Errorf("payment question = %s, want answered (no transaction left)", status)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM questions WHERE status = 'pending' AND merchant_key LIKE '%DAL SOTO%'`); n != 0 {
		t.Errorf("%d DAL SOTO questions still pending", n)
	}
	dash, err := insights.New(e.st).Dashboard(context.Background(), d(2026, 9, 1))
	if err != nil {
		t.Fatal(err)
	}
	if dash.Summary.Total != 800 {
		t.Errorf("september total = %d, want 800 (only the coffee)", dash.Summary.Total)
	}
}
