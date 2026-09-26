package insights

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Upcoming is "falta pagar" as of today (spec §10).
type Upcoming struct {
	Bills             []Bill
	ByMonth           []InstallmentMonth // next 12 months, starting at today's month
	Purchases         []InstallmentPurchase
	BillsTotal        int64
	InstallmentsTotal int64 // every projected installment from today's month on
	Total             int64
}

// Bill is a card invoice not yet due.
type Bill struct {
	AccountID   int64
	AccountName string
	DueDate     time.Time
	TotalCents  int64
}

// InstallmentMonth is the sum of installments falling in one month, split by card.
type InstallmentMonth struct {
	Month     time.Time
	Total     int64
	ByAccount []AccountAmount
}

// AccountAmount is a card's share of a month.
type AccountAmount struct {
	AccountID   int64
	AccountName string
	Total       int64
}

// InstallmentPurchase is one purchase still being paid in installments.
type InstallmentPurchase struct {
	AccountID         int64
	AccountName       string
	Key               string
	Description       string
	PurchaseDate      *time.Time
	InstallmentAmount int64
	Remaining         int // installments still to come ("faltam X de N")
	InstallmentTotal  int
	RemainingTotal    int64
	NextMonth         time.Time
}

func (s *Service) upcoming(ctx context.Context, today time.Time) (Upcoming, error) {
	u := Upcoming{Bills: []Bill{}, Purchases: []InstallmentPurchase{}}
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	rows, err := s.store.DB.QueryContext(ctx, `SELECT d.account_id, a.name, d.due_date, COALESCE(d.total_cents, 0)
		FROM documents d JOIN accounts a ON a.id = d.account_id
		WHERE d.kind = 'card_statement' AND d.due_date >= ? ORDER BY d.due_date, d.account_id`, day)
	if err != nil {
		return u, err
	}
	for rows.Next() {
		var b Bill
		if err := rows.Scan(&b.AccountID, &b.AccountName, &b.DueDate, &b.TotalCents); err != nil {
			rows.Close()
			return u, err
		}
		u.Bills = append(u.Bills, b)
		u.BillsTotal += b.TotalCents
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return u, err
	}

	// Only the projections of each card's most recent invoice (largest closing date) count, so
	// importing old invoices in any order changes nothing.
	start := monthOf(day)
	rows, err = s.store.DB.QueryContext(ctx, `SELECT i.account_id, a.name, i.document_id, i.merchant_key, i.description,
		i.purchase_date, i.due_month, i.installment_total, i.amount_cents
		FROM installments i JOIN accounts a ON a.id = i.account_id
		JOIN (SELECT d.account_id, d.id FROM documents d
		      JOIN (SELECT account_id, MAX(closing_date) AS closing FROM documents
		            WHERE kind = 'card_statement' GROUP BY account_id) latest
		        ON latest.account_id = d.account_id AND latest.closing = d.closing_date) doc ON doc.id = i.document_id
		WHERE i.due_month >= ?
		ORDER BY i.due_month, i.account_id, i.id`, start)
	if err != nil {
		return u, err
	}
	defer rows.Close()
	months := make([]InstallmentMonth, 12)
	for k := range months {
		months[k] = InstallmentMonth{Month: start.AddDate(0, k, 0), ByAccount: []AccountAmount{}}
	}
	type purchaseKey struct {
		doc      int64
		desc     string
		purchase time.Time
		total    int
		amount   int64
	}
	purchases := map[purchaseKey]*InstallmentPurchase{}
	var order []purchaseKey
	for rows.Next() {
		var acc, doc int64
		var name, key, desc string
		var purchase sql.NullTime
		var due time.Time
		var total int
		var amount int64
		if err := rows.Scan(&acc, &name, &doc, &key, &desc, &purchase, &due, &total, &amount); err != nil {
			return u, err
		}
		u.InstallmentsTotal += amount
		if k := monthsBetween(start, due); k < 12 {
			addToMonth(&months[k], acc, name, amount)
		}
		pk := purchaseKey{doc: doc, desc: desc, purchase: purchase.Time, total: total, amount: amount}
		p := purchases[pk]
		if p == nil {
			p = &InstallmentPurchase{AccountID: acc, AccountName: name, Key: key, Description: desc,
				InstallmentAmount: amount, InstallmentTotal: total, NextMonth: due}
			if purchase.Valid {
				t := purchase.Time
				p.PurchaseDate = &t
			}
			purchases[pk] = p
			order = append(order, pk)
		}
		p.Remaining++
		p.RemainingTotal += amount
	}
	if err := rows.Err(); err != nil {
		return u, err
	}
	u.ByMonth = months
	for _, pk := range order {
		u.Purchases = append(u.Purchases, *purchases[pk])
	}
	sort.SliceStable(u.Purchases, func(i, j int) bool { return u.Purchases[i].RemainingTotal > u.Purchases[j].RemainingTotal })
	u.Total = u.BillsTotal + u.InstallmentsTotal
	return u, nil
}

func addToMonth(m *InstallmentMonth, acc int64, name string, amount int64) {
	m.Total += amount
	for i := range m.ByAccount {
		if m.ByAccount[i].AccountID == acc {
			m.ByAccount[i].Total += amount
			return
		}
	}
	m.ByAccount = append(m.ByAccount, AccountAmount{AccountID: acc, AccountName: name, Total: amount})
}

// monthsBetween counts whole months from a to b (both first days of a month).
func monthsBetween(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}
