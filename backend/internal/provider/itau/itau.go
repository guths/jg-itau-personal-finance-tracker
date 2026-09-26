// Package itau reads Itaú credit card invoices (fatura) and checking account statements (extrato).
package itau

import (
	"fmt"
	"sort"
	"strings"

	"jgfinance/internal/layout"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
)

// Itau implements provider.Provider.
type Itau struct{}

var _ provider.Provider = (*Itau)(nil)

// New returns the Itaú provider.
func New() *Itau { return &Itau{} }

// ID implements provider.Provider.
func (*Itau) ID() string { return "itau" }

// Detect implements provider.Provider (spec §7.1).
func (*Itau) Detect(doc *pdftext.Document) (provider.DocKind, bool) {
	text := documentText(doc)
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(text, "Resumo da fatura em R$") && strings.Contains(text, "Total desta fatura") &&
		(strings.Contains(lower, "itau unibanco") || strings.Contains(lower, "banco itaú")):
		return provider.CardStatement, true
	case strings.Contains(lower, "extrato conta") && strings.Contains(text, "SALDO DO DIA") &&
		strings.Contains(lower, "itau.com.br"):
		return provider.BankStatement, true
	}
	return "", false
}

// Parse implements provider.Provider. The returned Statement is already reconciled.
func (*Itau) Parse(doc *pdftext.Document, kind provider.DocKind) (*provider.Statement, error) {
	switch kind {
	case provider.CardStatement:
		if len(doc.Pages) < 2 {
			return nil, &provider.LayoutError{Msg: "fatura com menos de 2 páginas"}
		}
		split := faturaSplit(doc)
		var body []layout.Line
		for _, p := range doc.Pages[1:] {
			body = append(body, layout.Lines(p, []float64{split})...)
		}
		return parseFaturaLines(layout.PageText(doc.Pages[0]), body)
	case provider.BankStatement:
		var lines []layout.Line
		for _, p := range doc.Pages {
			lines = append(lines, layout.Lines(p, nil)...)
		}
		return parseExtratoLines(lines)
	}
	return nil, fmt.Errorf("itau: tipo de documento não suportado: %s", kind)
}

func documentText(doc *pdftext.Document) string {
	var b strings.Builder
	for _, p := range doc.Pages {
		b.WriteString(layout.PageText(p))
		b.WriteByte('\n')
	}
	return b.String()
}

// faturaSplit finds the X that separates the two body columns: the right-most "DATA" header
// on the first body page with two header clusters (> 100pt apart), minus a 15pt margin.
// Fallback: 56.5% of the page width (the position measured on real invoices).
func faturaSplit(doc *pdftext.Document) float64 {
	for _, p := range doc.Pages[1:] {
		var xs []float64
		for _, r := range p.Runs {
			if r.Text == "DATA" || strings.HasPrefix(r.Text, "DATA ") {
				xs = append(xs, r.X0)
			}
		}
		if len(xs) < 2 {
			continue
		}
		sort.Float64s(xs)
		if xs[len(xs)-1]-xs[0] > 100 {
			return xs[len(xs)-1] - 15
		}
	}
	return doc.Pages[0].Width * 0.565
}
