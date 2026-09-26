package insights

import (
	"context"
	"fmt"
	"testing"
	"time"

	"jgfinance/internal/store"
	"jgfinance/internal/store/storetest"
)

type fx struct {
	t    *testing.T
	s    *store.Store
	card int64
	bank int64
	doc  int64
	n    int
}

func newFx(t *testing.T) *fx {
	t.Helper()
	f := &fx{t: t, s: storetest.New(t)}
	f.card = f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'credit_card', '8778', 'Platinum')`)
	f.bank = f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'checking', '1/2', 'Conta')`)
	f.doc = f.exec(`INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path) VALUES (?, 'bank_statement', 'base', 'x.pdf', 'p')`, f.bank)
	return f
}

func (f *fx) exec(q string, args ...any) int64 {
	f.t.Helper()
	r, err := f.s.DB.Exec(q, args...)
	if err != nil {
		f.t.Fatal(err)
	}
	id, _ := r.LastInsertId()
	return id
}

func month(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

// tx inserts a transaction counting in month m; cat 0 = uncategorized.
func (f *fx) tx(m time.Time, key string, cents int64, kind string, cat int64) {
	f.t.Helper()
	f.n++
	var c any
	if cat != 0 {
		c = cat
	}
	f.exec(`INSERT INTO transactions (document_id, account_id, seq, occurred_on, reference_month, description, merchant_key,
		amount_cents, kind, category_id, dedupe_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.doc, f.bank, f.n, m.AddDate(0, 0, f.n%27), m, key+" desc", key, cents, kind, c, fmt.Sprintf("k%d", f.n))
}

func (f *fx) sub(cat, sub string) int64 {
	f.t.Helper()
	var id int64
	if err := f.s.DB.QueryRow(`SELECT s.id FROM categories s JOIN categories c ON c.id = s.parent_id WHERE c.name = ? AND s.name = ?`, cat, sub).Scan(&id); err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fx) dashboard(m time.Time, today time.Time) Dashboard {
	f.t.Helper()
	d, err := New(f.s).Dashboard(context.Background(), m, today)
	if err != nil {
		f.t.Fatal(err)
	}
	return d
}

var today = time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)

func TestSummaryAndByCategory(t *testing.T) {
	f := newFx(t)
	sep, aug := month(2026, 9), month(2026, 8)
	market, bakery, app := f.sub("Alimentação", "Supermercado"), f.sub("Alimentação", "Padaria e café"), f.sub("Transporte", "Aplicativo")
	f.tx(sep, "MERCADO", 30000, "expense", market)
	f.tx(sep, "MERCADO", -5000, "refund", market)
	f.tx(sep, "PADARIA", 2000, "expense", bakery)
	f.tx(sep, "UBER", 4000, "expense", app)
	f.tx(sep, "LOJA", 1500, "expense", 0)
	f.tx(sep, "SALARIO", -900000, "income", 0)
	f.tx(sep, "FATURA PAGA", 100000, "bill_payment", 0)
	f.tx(sep, "PIX TRANSF EU", 50000, "own_transfer", 0)
	f.tx(aug, "MERCADO", 99999, "expense", market) // another month: never mixed in

	d := f.dashboard(sep, today)
	if d.Summary.Total != 32500 {
		t.Errorf("total = %d, want 32500", d.Summary.Total)
	}
	if d.Summary.Uncategorized != (Uncategorized{Count: 1, Total: 1500}) {
		t.Errorf("uncategorized = %+v", d.Summary.Uncategorized)
	}
	if len(d.ByCategory) != 2 {
		t.Fatalf("by category = %+v", d.ByCategory)
	}
	food := d.ByCategory[0]
	if food.Name != "Alimentação" || food.Total != 27000 || food.Color == "" || len(food.Subcategories) != 2 ||
		food.Subcategories[0].Name != "Supermercado" || food.Subcategories[0].Total != 25000 || food.Subcategories[0].Count != 2 {
		t.Errorf("food = %+v", food)
	}
	if d.ByCategory[1].Name != "Transporte" || d.ByCategory[1].Total != 4000 {
		t.Errorf("transport = %+v", d.ByCategory[1])
	}
	if len(d.TopMerchants) != 4 || d.TopMerchants[0].Key != "MERCADO" || d.TopMerchants[0].Total != 25000 ||
		d.TopMerchants[0].Count != 2 || d.TopMerchants[0].CategoryPath != "Alimentação › Supermercado" {
		t.Errorf("top = %+v", d.TopMerchants)
	}
	if len(d.Evolution) != 12 || !d.Evolution[11].Month.Equal(sep) || d.Evolution[11].Total != 32500 ||
		d.Evolution[11].Uncategorized != 1500 || d.Evolution[10].Total != 99999 || !d.Evolution[0].Month.Equal(month(2025, 10)) {
		t.Errorf("evolution = %+v", d.Evolution)
	}
	if ev := d.Evolution[11].Categories; len(ev) != 2 || ev[0].Name != "Alimentação" || ev[0].Total != 27000 {
		t.Errorf("evolution categories = %+v", ev)
	}
}

func TestPrev3AvgUsesOnlyMonthsWithData(t *testing.T) {
	f := newFx(t)
	f.tx(month(2026, 9), "A", 1000, "expense", 0)
	if d := f.dashboard(month(2026, 9), today); d.Summary.Prev3Avg != nil {
		t.Errorf("no previous data: %v", *d.Summary.Prev3Avg)
	}
	f.tx(month(2026, 8), "A", 3000, "expense", 0)
	f.tx(month(2026, 6), "A", 2001, "expense", 0)
	f.tx(month(2026, 5), "A", 99999, "expense", 0) // outside M-3..M-1
	d := f.dashboard(month(2026, 9), today)
	if d.Summary.Prev3Avg == nil || *d.Summary.Prev3Avg != 2501 {
		t.Errorf("prev3_avg = %v, want 2501 (July has no data)", d.Summary.Prev3Avg)
	}
}

func TestRecurring(t *testing.T) {
	f := newFx(t)
	stream := f.sub("Assinaturas", "Streaming")
	m := month(2026, 9)
	for k := 3; k >= 0; k-- {
		mm := m.AddDate(0, -k, 0)
		f.tx(mm, "NETFLIX", 5590, "expense", stream)           // steady: ok, 4 months
		f.tx(mm, "MERCADO", 10000+int64(k)*9000, "expense", 0) // varies too much: not recurring
		for range 3 {
			f.tx(mm, "UBER", 1000, "expense", 0) // 3 per month: not recurring
		}
	}
	f.tx(m.AddDate(0, -1, 0), "ACADEMIA", 10000, "expense", 0) // increased: 100 → 115
	f.tx(m.AddDate(0, -2, 0), "ACADEMIA", 10000, "expense", 0)
	f.tx(m, "ACADEMIA", 11500, "expense", 0)
	f.tx(m.AddDate(0, -1, 0), "SPOTIFY", 2190, "expense", 0) // absent in M
	f.tx(m.AddDate(0, -2, 0), "SPOTIFY", 2190, "expense", 0)
	f.tx(m, "CURSO", 30000, "expense", 0) // rule yes: recurring from the first month
	f.exec(`INSERT INTO rules (merchant_key, alias, recurring, source) VALUES ('CURSO', 'Curso de inglês', 'yes', 'user')`)
	f.tx(m.AddDate(0, -1, 0), "ALUGUEL", 250000, "expense", 0) // rule no: never
	f.tx(m, "ALUGUEL", 250000, "expense", 0)
	f.exec(`INSERT INTO rules (merchant_key, recurring, source) VALUES ('ALUGUEL', 'no', 'user')`)
	f.exec(`INSERT INTO rules (merchant_key, category_id, alias, source) VALUES ('NETFLIX', ?, 'Netflix', 'llm')`, stream)

	d := f.dashboard(m, today)
	got := map[string]RecurringItem{}
	for _, r := range d.Recurring {
		got[r.Key] = r
	}
	if len(got) != 4 {
		t.Errorf("recurring keys = %v", d.Recurring)
	}
	if n := got["NETFLIX"]; n.Status != StatusOK || n.Months != 4 || n.Total != 5590 || n.Average != 5590 ||
		n.Alias != "Netflix" || n.CategoryPath != "Assinaturas › Streaming" || n.ChangePct == nil || *n.ChangePct != 0 {
		t.Errorf("netflix = %+v", n)
	}
	if a := got["ACADEMIA"]; a.Status != StatusIncreased || a.Months != 3 || a.ChangePct == nil || *a.ChangePct != 15 {
		t.Errorf("academia = %+v", a)
	}
	if s := got["SPOTIFY"]; s.Status != StatusAbsent || s.Total != 0 || s.Months != 2 || s.Average != 2190 {
		t.Errorf("spotify = %+v", s)
	}
	if c := got["CURSO"]; c.Status != StatusOK || c.Months != 1 || c.Alias != "Curso de inglês" {
		t.Errorf("curso = %+v", c)
	}
	if d.Summary.Fixed != 5590+11500+30000 {
		t.Errorf("fixed = %d", d.Summary.Fixed)
	}
	if last := d.Recurring[len(d.Recurring)-1]; last.Key != "SPOTIFY" {
		t.Errorf("absent items go last: %+v", d.Recurring)
	}
}

func TestUpcomingUsesOnlyLatestInvoicePerCard(t *testing.T) {
	f := newFx(t)
	other := f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'credit_card', '4229', 'Infinite')`)
	doc := func(acc int64, sha string, closing, due time.Time, total int64) int64 {
		return f.exec(`INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path, closing_date, due_date, reference_month, total_cents)
			VALUES (?, 'card_statement', ?, 'f.pdf', 'p', ?, ?, ?, ?)`, acc, sha, closing, due, monthOf(closing), total)
	}
	inst := func(acc, d int64, desc string, due time.Time, n, total int, amount int64) {
		f.exec(`INSERT INTO installments (account_id, document_id, merchant_key, description, purchase_date, due_month,
			installment_n, installment_total, amount_cents) VALUES (?, ?, ?, ?, '2026-05-10', ?, ?, ?, ?)`, acc, d, desc, desc, due, n, total, amount)
	}
	// Newest invoice imported first, then an older one: only the newest counts.
	newest := doc(f.card, "new", time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), 400000)
	older := doc(f.card, "old", time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC), 350000)
	for k := range 3 {
		inst(f.card, newest, "LOJA X 08/10", month(2026, 10).AddDate(0, k, 0), 8+k, 10, 2500)
		inst(f.card, older, "LOJA X 07/10", month(2026, 9).AddDate(0, k, 0), 7+k, 10, 2500)
	}
	inst(f.card, older, "LOJA X 10/10", month(2026, 12), 10, 10, 2500)
	infinite := doc(other, "inf", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC), 100000)
	inst(other, infinite, "CURSO 02/03", month(2026, 10), 2, 3, 10000)
	inst(other, infinite, "CURSO 02/03", month(2026, 11), 3, 3, 10000)

	u := f.dashboard(month(2026, 9), today).Upcoming
	if len(u.Bills) != 2 || u.BillsTotal != 500000 || u.Bills[0].AccountName != "Infinite" {
		t.Errorf("bills = %+v (due today counts, due 03/09 does not)", u.Bills)
	}
	if u.InstallmentsTotal != 3*2500+2*10000 || u.Total != 500000+27500 {
		t.Errorf("installments total = %d, total = %d", u.InstallmentsTotal, u.Total)
	}
	if len(u.ByMonth) != 12 || !u.ByMonth[0].Month.Equal(month(2026, 9)) || u.ByMonth[0].Total != 0 ||
		u.ByMonth[1].Total != 12500 || len(u.ByMonth[1].ByAccount) != 2 || u.ByMonth[3].Total != 2500 {
		t.Errorf("by month = %+v", u.ByMonth)
	}
	if len(u.Purchases) != 2 {
		t.Fatalf("purchases = %+v", u.Purchases)
	}
	p := u.Purchases[0]
	if p.Description != "CURSO 02/03" || p.Remaining != 2 || p.InstallmentTotal != 3 || p.RemainingTotal != 20000 ||
		!p.NextMonth.Equal(month(2026, 10)) || p.PurchaseDate == nil {
		t.Errorf("purchase = %+v", p)
	}
	if q := u.Purchases[1]; q.Remaining != 3 || q.RemainingTotal != 7500 || q.AccountName != "Platinum" {
		t.Errorf("purchase = %+v", q)
	}
	d := f.dashboard(month(2026, 9), today)
	if d.Summary.OpenBills != 500000 || d.Summary.FutureInstallments != 27500 {
		t.Errorf("summary = %+v", d.Summary)
	}
}

func TestMonths(t *testing.T) {
	f := newFx(t)
	s := New(f.s)
	got, err := s.Months(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	f.tx(month(2026, 8), "A", 1, "expense", 0)
	f.tx(month(2026, 9), "A", 1, "income", 0)
	f.tx(month(2026, 8), "B", 1, "expense", 0)
	got, _ = s.Months(context.Background())
	if len(got) != 2 || !got[0].Equal(month(2026, 9)) || !got[1].Equal(month(2026, 8)) {
		t.Errorf("months = %v", got)
	}
}

func TestWithinMedianAndSequence(t *testing.T) {
	cases := []struct {
		totals []int64
		want   bool
	}{
		{[]int64{100, 100}, true},
		{[]int64{100, 120}, true},  // median 110: both within 20%
		{[]int64{100, 151}, false}, // median 125.5: 100 is 20.3% below
		{[]int64{100, 100, 100, 300}, false},
		{[]int64{0, 0}, false},
	}
	for _, c := range cases {
		if got := withinMedian(c.totals); got != c.want {
			t.Errorf("withinMedian(%v) = %v", c.totals, got)
		}
	}
	m := month(2026, 9)
	h := &keyHistory{months: map[time.Time]keyMonth{
		m: {Total: 300, Count: 1}, m.AddDate(0, -1, 0): {Total: 290, Count: 2}, m.AddDate(0, -2, 0): {Total: 100, Count: 1},
	}}
	if n, avg, ok := sequence(h, m, false); !ok || n != 2 || avg != 295 {
		t.Errorf("sequence = %d %d %v (the longest qualifying suffix)", n, avg, ok)
	}
}
