// Command inspect parses bank PDFs and prints what would be imported, without a database.
//
//	cd backend && go run ./cmd/inspect [-v] arquivo.pdf...
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jgfinance/internal/config"
	"jgfinance/internal/money"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
	"jgfinance/internal/provider/itau"
)

func main() {
	verbose := flag.Bool("v", false, "lista todas as transações")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "uso: go run ./cmd/inspect [-v] arquivo.pdf...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	cfg := config.Load("../.env", ".env")
	reg := provider.NewRegistry(itau.New())
	failed := false
	for _, path := range flag.Args() {
		st, err := parseFile(reg, cfg.PDFPasswords, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", filepath.Base(path), err)
			failed = true
			continue
		}
		fmt.Print(summary(filepath.Base(path), st, *verbose))
	}
	if failed {
		os.Exit(1)
	}
}

// parseFile opens, detects and parses (with reconciliation) one PDF.
func parseFile(reg *provider.Registry, passwords []string, path string) (*provider.Statement, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := pdftext.Open(data, passwords)
	if err != nil {
		return nil, err
	}
	p, kind, err := reg.Detect(doc)
	if err != nil {
		return nil, err
	}
	st, err := p.Parse(doc, kind)
	if err != nil {
		return nil, err
	}
	provider.MarkReversals(st.Transactions) // as the importer does
	return st, nil
}

var kindLabels = []struct {
	kind  provider.TxKind
	label string
}{
	{provider.Expense, "gastos"},
	{provider.Refund, "estornos"},
	{provider.Reversed, "estornos anul."},
	{provider.BillPayment, "pagam. fatura"},
	{provider.Investment, "investimentos"},
	{provider.Income, "entradas"},
}

// summary renders a human-readable report of a parsed statement.
func summary(name string, st *provider.Statement, verbose bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✓ %s — %s\n", name, st.Account.DefaultName)
	switch st.Kind {
	case provider.CardStatement:
		fmt.Fprintf(&b, "  fatura: fechamento %s · vencimento %s · total %s (conferido)\n",
			date(st.ClosingDate), date(st.DueDate), money.FormatBRL(st.TotalCents))
	case provider.BankStatement:
		fmt.Fprintf(&b, "  extrato: %s a %s (saldos diários conferidos)\n", date(st.PeriodStart), date(st.PeriodEnd))
	}
	counts := map[provider.TxKind]int{}
	sums := map[provider.TxKind]int64{}
	for _, tx := range st.Transactions {
		counts[tx.Kind]++
		sums[tx.Kind] += tx.AmountCents
	}
	for _, k := range kindLabels {
		if counts[k.kind] > 0 {
			fmt.Fprintf(&b, "  %-14s %3d  %s\n", k.label, counts[k.kind], money.FormatBRL(sums[k.kind]))
		}
	}
	if len(st.Upcoming) > 0 {
		var next int64
		for _, u := range st.Upcoming {
			next += u.AmountCents
		}
		fmt.Fprintf(&b, "  parcelas na próxima fatura: %d · %s\n", len(st.Upcoming), money.FormatBRL(next))
	}
	for _, w := range st.Warnings {
		fmt.Fprintf(&b, "  ⚠ %s\n", w)
	}
	if verbose {
		for _, tx := range st.Transactions {
			fmt.Fprintf(&b, "    %s  %-12s %14s  %-34s %s\n",
				date(tx.Date), tx.Kind, money.FormatBRL(tx.AmountCents), tx.Description, tx.Key)
		}
	}
	return b.String()
}

func date(t time.Time) string { return t.Format("02/01/2006") }
