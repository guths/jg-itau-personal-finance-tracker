package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"jgfinance/internal/provider"
	"jgfinance/internal/provider/itau"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func cardStatement() *provider.Statement {
	return &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{DefaultName: "Itaú Platinum •••• 1234"},
		ClosingDate: day(2026, 2, 24),
		DueDate:     day(2026, 3, 3),
		TotalCents:  116489,
		Transactions: []provider.Transaction{
			{Date: day(2026, 2, 10), Description: "MERCADO CENTRAL", Key: "MERCADO CENTRAL", Kind: provider.Expense, AmountCents: 15000},
			{Date: day(2026, 2, 12), Description: "aliexpressLOJA", Key: "ALIEXPRESSLOJA", Kind: provider.Refund, AmountCents: -4000},
			{Date: day(2026, 2, 1), Description: "Pagamento via conta", Key: "PAGAMENTO VIA CONTA", Kind: provider.BillPayment, AmountCents: -100000},
		},
		Upcoming: []provider.UpcomingInstallment{{AmountCents: 15000}},
		Warnings: []string{"projeção difere"},
	}
}

func TestSummary(t *testing.T) {
	out := summary("fatura.pdf", cardStatement(), false)
	for _, want := range []string{
		"✓ fatura.pdf — Itaú Platinum •••• 1234",
		"fechamento 24/02/2026",
		"vencimento 03/03/2026",
		"R$ 1.164,89",
		"gastos",
		"R$ 150,00",
		"estornos",
		"-R$ 40,00",
		"parcelas na próxima fatura: 1 · R$ 150,00",
		"⚠ projeção difere",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "MERCADO CENTRAL") {
		t.Error("non-verbose summary must not list transactions")
	}
	if !strings.Contains(summary("fatura.pdf", cardStatement(), true), "MERCADO CENTRAL") {
		t.Error("verbose summary must list transactions")
	}
}

func TestParseFileUnknownDocument(t *testing.T) {
	_, err := parseFile(provider.NewRegistry(itau.New()), nil, "../../internal/pdftext/testdata/plain.pdf")
	if !errors.Is(err, provider.ErrUnknownDocument) {
		t.Errorf("err = %v, want ErrUnknownDocument", err)
	}
}
