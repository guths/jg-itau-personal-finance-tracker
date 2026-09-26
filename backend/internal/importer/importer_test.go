package importer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"jgfinance/internal/categorize"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
	"jgfinance/internal/store"
	"jgfinance/internal/store/storetest"
)

// fakeProvider recognizes the synthetic fixture (it contains "Topo") and returns a fixed statement.
type fakeProvider struct {
	st  *provider.Statement
	err error
}

func (f *fakeProvider) ID() string { return "fake" }

func (f *fakeProvider) Detect(doc *pdftext.Document) (provider.DocKind, bool) {
	for _, p := range doc.Pages {
		for _, r := range p.Runs {
			if len(r.Text) >= 4 && r.Text[:4] == "Topo" {
				return f.st.Kind, true
			}
		}
	}
	return "", false
}

func (f *fakeProvider) Parse(*pdftext.Document, provider.DocKind) (*provider.Statement, error) {
	if f.err != nil {
		return nil, f.err
	}
	cp := *f.st
	return &cp, nil
}

type fakeCategorizer struct {
	calls [][]int64
	err   error
}

func (c *fakeCategorizer) Run(_ context.Context, ids []int64) (categorize.Result, error) {
	c.calls = append(c.calls, ids)
	return categorize.Result{Auto: len(ids)}, c.err
}

type env struct {
	im      *Importer
	st      *store.Store
	prov    *fakeProvider
	cat     *fakeCategorizer
	dataDir string
}

func newEnv(t *testing.T, st *provider.Statement) *env {
	t.Helper()
	s := storetest.New(t)
	e := &env{st: s, prov: &fakeProvider{st: st}, cat: &fakeCategorizer{}, dataDir: t.TempDir()}
	e.im = New(s, provider.NewRegistry(e.prov), nil, e.dataDir, e.cat)
	return e
}

// pdf returns the synthetic fixture; different variants have different bytes (a trailing
// comment after %%EOF) but the same content.
func pdf(t *testing.T, variant int) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "pdftext", "testdata", "plain.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if variant > 0 {
		data = append(data, []byte(fmt.Sprintf("\n%% variante %d\n", variant))...)
	}
	return data
}

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func cardStatement() *provider.Statement {
	return &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{Provider: "fake", Kind: provider.CreditCard, ExternalID: "8778", DefaultName: "Cartão •••• 8778"},
		ClosingDate: d(2026, 2, 24),
		DueDate:     d(2026, 3, 3),
		TotalCents:  15000,
		Transactions: []provider.Transaction{
			{Seq: 1, Date: d(2026, 2, 2), Description: "PAGAMENTO", Key: "PAGAMENTO", AmountCents: -9000, Kind: provider.BillPayment},
			{Seq: 2, Date: d(2026, 1, 28), Description: "PADARIA EXEMPLO", Key: "PADARIA EXEMPLO", AmountCents: 1250, Kind: provider.Expense, BankCategory: "restaurante", City: "SAO PAULO"},
			{Seq: 3, Date: d(2026, 2, 10), Description: "LOJA X 04/10", Key: "LOJA X", AmountCents: 10000, Kind: provider.Expense, InstallmentN: 4, InstallmentTotal: 10, Holder: "FULANO"},
			{Seq: 4, Date: d(2026, 2, 11), Description: "AMAZON US", Key: "AMAZON US", AmountCents: 3750, Kind: provider.Expense, OriginalAmountCents: 700, OriginalCurrency: "USD"},
		},
		Upcoming: []provider.UpcomingInstallment{
			{PurchaseDate: d(2025, 11, 10), Description: "LOJA X 05/10", Key: "LOJA X", AmountCents: 10000, N: 5, Total: 10},
		},
		Warnings: []string{"aviso de teste"},
	}
}

func bankStatement(txs ...provider.Transaction) *provider.Statement {
	for i := range txs {
		txs[i].Seq = i + 1
	}
	return &provider.Statement{
		Kind:         provider.BankStatement,
		Account:      provider.AccountRef{Provider: "fake", Kind: provider.Checking, ExternalID: "7293/030595-4", DefaultName: "Conta 030595-4"},
		PeriodStart:  d(2026, 8, 25),
		PeriodEnd:    d(2026, 9, 5),
		Transactions: txs,
	}
}

func count(t *testing.T, s *store.Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func importOne(t *testing.T, e *env, name string, data []byte) FileResult {
	t.Helper()
	res, err := e.im.Import(context.Background(), []File{{Name: name, Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("results = %d", len(res.Results))
	}
	return res.Results[0]
}

func TestImportCardStatement(t *testing.T) {
	e := newEnv(t, cardStatement())
	data := pdf(t, 0)
	r := importOne(t, e, "fatura.pdf", data)

	if r.Status != StatusImported || r.Error != "" {
		t.Fatalf("result = %+v", r)
	}
	if r.AccountName != "Cartão •••• 8778" || r.Kind != provider.CardStatement || !r.ReferenceMonth.Equal(d(2026, 2, 1)) {
		t.Errorf("result = %+v", r)
	}
	if r.NewCount != 4 || r.ExistingCount != 0 || r.TotalCents != 15000 || len(r.Warnings) != 1 {
		t.Errorf("counts = %+v", r)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM transactions WHERE reference_month = '2026-02-01'`); n != 4 {
		t.Errorf("transactions in 2026-02 = %d, want 4 (card lines count in the closing month)", n)
	}
	var occurred time.Time
	var city, currency string
	var orig int64
	if err := e.st.DB.QueryRow(`SELECT occurred_on, city FROM transactions WHERE seq = 2`).Scan(&occurred, &city); err != nil {
		t.Fatal(err)
	}
	if !occurred.Equal(d(2026, 1, 28)) || city != "SAO PAULO" {
		t.Errorf("seq 2: %v %q", occurred, city)
	}
	if err := e.st.DB.QueryRow(`SELECT original_amount_cents, original_currency FROM transactions WHERE seq = 4`).Scan(&orig, &currency); err != nil {
		t.Fatal(err)
	}
	if orig != 700 || currency != "USD" {
		t.Errorf("seq 4: %d %q", orig, currency)
	}
	var stored string
	var total int64
	if err := e.st.DB.QueryRow(`SELECT stored_path, total_cents FROM documents`).Scan(&stored, &total); err != nil {
		t.Fatal(err)
	}
	if saved, err := os.ReadFile(filepath.Join(e.dataDir, stored)); err != nil || len(saved) != len(data) {
		t.Errorf("stored PDF %q: %v", stored, err)
	}
	if total != 15000 {
		t.Errorf("total_cents = %d", total)
	}
	if len(e.cat.calls) != 1 || len(e.cat.calls[0]) != 4 {
		t.Errorf("categorizer calls = %v", e.cat.calls)
	}
}

func TestImportSameFileTwiceIsDuplicate(t *testing.T) {
	e := newEnv(t, cardStatement())
	importOne(t, e, "a.pdf", pdf(t, 0))
	res, err := e.im.Import(context.Background(), []File{{Name: "a.pdf", Data: pdf(t, 0)}, {Name: "b.pdf", Data: pdf(t, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Results {
		if r.Status != StatusDuplicate || r.Error != "arquivo já importado" {
			t.Errorf("%s: %+v", r.Name, r)
		}
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM transactions`); n != 4 {
		t.Errorf("transactions = %d", n)
	}
	if len(e.cat.calls) != 1 {
		t.Errorf("categorizer must not run without new transactions: %v", e.cat.calls)
	}
}

func TestImportSameInvoiceWithDifferentBytesIsDuplicate(t *testing.T) {
	e := newEnv(t, cardStatement())
	importOne(t, e, "a.pdf", pdf(t, 0))
	r := importOne(t, e, "a-de-novo.pdf", pdf(t, 1))
	if r.Status != StatusDuplicate || r.Error != "fatura já importada" {
		t.Fatalf("result = %+v", r)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM documents`); n != 1 {
		t.Errorf("documents = %d", n)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM installments`); n != 6 {
		t.Errorf("installments = %d, want 6 (not duplicated)", n)
	}
	entries, _ := os.ReadDir(filepath.Join(e.dataDir, "uploads"))
	if pdfs := countFiles(entries); pdfs != 1 {
		t.Errorf("stored PDFs = %d, want 1", pdfs)
	}
}

func countFiles(entries []os.DirEntry) int {
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			n++
		}
	}
	return n
}

func TestImportOverlappingBankStatements(t *testing.T) {
	pix := provider.Transaction{Date: d(2026, 8, 31), Description: "PIX TRANSF ANA 31/08", Key: "PIX TRANSF ANA", AmountCents: 5000, Kind: provider.Expense, PersonTransfer: true}
	coffee := provider.Transaction{Date: d(2026, 9, 1), Description: "CAFE", Key: "CAFE", AmountCents: 800, Kind: provider.Expense}
	salary := provider.Transaction{Date: d(2026, 9, 5), Description: "SALARIO", Key: "SALARIO", AmountCents: -100000, Kind: provider.Income}
	market := provider.Transaction{Date: d(2026, 9, 6), Description: "MERCADO", Key: "MERCADO", AmountCents: 4200, Kind: provider.Expense}

	e := newEnv(t, bankStatement(pix, coffee, coffee, salary))
	r1 := importOne(t, e, "extrato-1.pdf", pdf(t, 0))
	if r1.Status != StatusImported || r1.NewCount != 4 || r1.ExistingCount != 0 {
		t.Fatalf("first = %+v", r1)
	}
	if r1.TotalCents != 6600 || !r1.ReferenceMonth.Equal(d(2026, 9, 1)) {
		t.Errorf("first total/month = %d %v", r1.TotalCents, r1.ReferenceMonth)
	}
	// Each transaction counts in the month of its own date.
	if n := count(t, e.st, `SELECT COUNT(*) FROM transactions WHERE reference_month = '2026-08-01'`); n != 1 {
		t.Errorf("august = %d, want 1", n)
	}

	e.prov.st = bankStatement(coffee, coffee, coffee, salary, market)
	r2 := importOne(t, e, "extrato-2.pdf", pdf(t, 1))
	if r2.Status != StatusImported || r2.NewCount != 2 || r2.ExistingCount != 3 {
		t.Fatalf("second = %+v", r2)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM transactions WHERE description = 'CAFE'`); n != 3 {
		t.Errorf("CAFE rows = %d, want 3", n)
	}
	if len(e.cat.calls) != 2 || len(e.cat.calls[1]) != 2 {
		t.Errorf("categorizer calls = %v", e.cat.calls)
	}
}

func TestImportReconciliationFailureKeepsPDFInFailed(t *testing.T) {
	e := newEnv(t, cardStatement())
	e.prov.err = &provider.ReconciliationError{Rule: "compras = Lançamentos no cartão", Computed: 100, Printed: 200}
	data := pdf(t, 0)
	r := importOne(t, e, "ruim.pdf", data)
	if r.Status != StatusFailed || r.Error != e.prov.err.Error() {
		t.Fatalf("result = %+v", r)
	}
	entries, err := os.ReadDir(filepath.Join(e.dataDir, "uploads", "failed"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed dir: %v %v", entries, err)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM documents`) + count(t, e.st, `SELECT COUNT(*) FROM accounts`); n != 0 {
		t.Errorf("rows written for a failed file: %d", n)
	}
	if len(e.cat.calls) != 0 {
		t.Errorf("categorizer called: %v", e.cat.calls)
	}
}

func TestImportUnknownAndInvalidFiles(t *testing.T) {
	e := newEnv(t, cardStatement())
	e.im.registry = provider.NewRegistry() // no provider recognizes anything
	res, err := e.im.Import(context.Background(), []File{{Name: "x.pdf", Data: pdf(t, 0)}, {Name: "y.pdf", Data: []byte("not a pdf")}})
	if err != nil {
		t.Fatal(err)
	}
	if r := res.Results[0]; r.Status != StatusFailed || r.Error != "Nenhum banco reconheceu este arquivo" {
		t.Errorf("unknown: %+v", r)
	}
	if r := res.Results[1]; r.Status != StatusFailed || r.Error != pdftext.ErrNotPDF.Error() {
		t.Errorf("invalid: %+v", r)
	}
}

func TestImportInstallmentProjection(t *testing.T) {
	st := cardStatement()
	st.Upcoming = []provider.UpcomingInstallment{{PurchaseDate: d(2025, 11, 10), Description: "LOJA X 04/10", Key: "LOJA X", AmountCents: 2599, N: 4, Total: 10}}
	e := newEnv(t, st)
	importOne(t, e, "fatura.pdf", pdf(t, 0))

	rows, err := e.st.DB.Query(`SELECT due_month, installment_n, installment_total, amount_cents FROM installments ORDER BY due_month`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var m time.Time
		var n, total int
		var amount int64
		if err := rows.Scan(&m, &n, &total, &amount); err != nil {
			t.Fatal(err)
		}
		got = append(got, fmt.Sprintf("%s %d/%d %d", m.Format("2006-01"), n, total, amount))
	}
	want := []string{"2026-03 4/10 2599", "2026-04 5/10 2599", "2026-05 6/10 2599", "2026-06 7/10 2599",
		"2026-07 8/10 2599", "2026-08 9/10 2599", "2026-09 10/10 2599"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("installments:\n got %v\nwant %v", got, want)
	}
}

func TestImportCategorizationErrorKeepsImport(t *testing.T) {
	e := newEnv(t, cardStatement())
	e.cat.err = errors.New("banco fora do ar")
	res, err := e.im.Import(context.Background(), []File{{Name: "f.pdf", Data: pdf(t, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Results[0].Status != StatusImported || len(res.Warnings) != 1 {
		t.Errorf("res = %+v", res)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM transactions`); n != 4 {
		t.Errorf("transactions = %d", n)
	}
}

func TestImportKeepsRenamedAccount(t *testing.T) {
	e := newEnv(t, cardStatement())
	importOne(t, e, "a.pdf", pdf(t, 0))
	if _, err := e.st.DB.Exec(`UPDATE accounts SET name = 'Meu cartão'`); err != nil {
		t.Fatal(err)
	}
	st := cardStatement()
	st.ClosingDate = d(2026, 3, 24)
	e.prov.st = st
	r := importOne(t, e, "b.pdf", pdf(t, 1))
	if r.AccountName != "Meu cartão" || count(t, e.st, `SELECT COUNT(*) FROM accounts`) != 1 {
		t.Errorf("result = %+v", r)
	}
}

func TestDedupeKeysAndProjectionUnit(t *testing.T) {
	st := bankStatement(
		provider.Transaction{Date: d(2026, 9, 1), Description: "CAFE", AmountCents: 800},
		provider.Transaction{Date: d(2026, 9, 1), Description: "CAFE", AmountCents: 800},
	)
	k := dedupeKeys(7, st)
	if k[0] == k[1] || len(k[0]) != 64 {
		t.Errorf("identical same-day lines must differ: %v", k)
	}
	if again := dedupeKeys(7, st); again[0] != k[0] || again[1] != k[1] {
		t.Error("keys must be deterministic")
	}
	if other := dedupeKeys(8, st); other[0] == k[0] {
		t.Error("keys must depend on the account")
	}
	p := projectInstallments(d(2026, 12, 1), []provider.UpcomingInstallment{{N: 2, Total: 3, AmountCents: 10}})
	if len(p) != 2 || !p[0].DueMonth.Equal(d(2027, 1, 1)) || !p[1].DueMonth.Equal(d(2027, 2, 1)) || p[1].N != 3 {
		t.Errorf("projection = %+v", p)
	}
}

func TestConcurrentUploadsOfSameInvoice(t *testing.T) {
	e := newEnv(t, cardStatement())
	// Two importers (as two API processes would) bypass the in-process lock; the database
	// constraints still decide a single winner.
	other := New(e.st, provider.NewRegistry(e.prov), nil, e.dataDir, e.cat)
	results := make(chan FileResult, 2)
	for i, im := range []*Importer{e.im, other} {
		go func() {
			res, err := im.Import(context.Background(), []File{{Name: "f.pdf", Data: pdf(t, i)}})
			if err != nil {
				t.Error(err)
			}
			results <- res.Results[0]
		}()
	}
	statuses := map[string]int{}
	for range 2 {
		statuses[(<-results).Status]++
	}
	if statuses[StatusImported] != 1 || statuses[StatusDuplicate] != 1 {
		t.Errorf("statuses = %v", statuses)
	}
	if n := count(t, e.st, `SELECT COUNT(*) FROM transactions`); n != 4 {
		t.Errorf("transactions = %d", n)
	}
}
