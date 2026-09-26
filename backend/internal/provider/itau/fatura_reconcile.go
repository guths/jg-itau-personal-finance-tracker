package itau

import (
	"fmt"

	"jgfinance/internal/money"
	"jgfinance/internal/provider"
)

// reconcileFatura checks every printed total against the extracted items (spec §7.2).
// Rules whose label is absent from the document are skipped.
func reconcileFatura(h faturaHeader, f *faturaBody) ([]string, error) {
	sums := map[section]int64{}
	counts := map[section]int{}
	for _, it := range f.items {
		sums[it.sec] += it.tx.AmountCents
		counts[it.sec]++
	}
	check := func(rule string, computed, printed int64) error {
		if computed != printed {
			return &provider.ReconciliationError{Rule: rule, Computed: computed, Printed: printed}
		}
		return nil
	}
	requireTotal := func(sec section, label, what string) error {
		if counts[sec] > 0 && !f.seen[label] {
			return &provider.LayoutError{Msg: fmt.Sprintf("%s sem o total %q", what, label)}
		}
		return nil
	}

	iof := f.totals[lblIOF]
	current := sums[secPurchases] + sums[secFees] + sums[secIntl] + iof
	var upcomingNext, projectedLater int64
	for _, u := range f.upcoming {
		upcomingNext += u.AmountCents
		projectedLater += int64(u.Total-u.N) * u.AmountCents
	}

	steps := []func() error{
		func() error { return requireTotal(secPurchases, lblPurchases, "compras") },
		func() error { return requireTotal(secFees, lblFees, "produtos e serviços") },
		func() error { return requireTotal(secIntl, lblIntl, "compras internacionais") },
	}
	if f.seen[lblPayments] {
		steps = append(steps, func() error { return check("pagamentos = "+lblPayments, sums[secPayments], f.totals[lblPayments]) })
	}
	if f.seen[lblPurchases] {
		steps = append(steps, func() error { return check("compras = "+lblPurchases, sums[secPurchases], f.totals[lblPurchases]) })
	}
	if f.seen[lblFees] {
		steps = append(steps, func() error { return check("produtos e serviços = "+lblFees, sums[secFees], f.totals[lblFees]) })
	}
	if f.seen[lblIntl] {
		steps = append(steps, func() error { return check("internacionais = "+lblIntl, sums[secIntl], f.totals[lblIntl]) })
	}
	if f.seen[lblIntlTotal] {
		steps = append(steps, func() error {
			return check("internacionais + IOF = "+lblIntlTotal, sums[secIntl]+iof, f.totals[lblIntlTotal])
		})
	}
	if f.seen[lblCurrent] {
		steps = append(steps, func() error {
			return check("compras + produtos + internacionais + IOF = "+lblCurrent, current, f.totals[lblCurrent])
		})
	}
	if h.hasCurrent {
		steps = append(steps, func() error {
			return check("compras + produtos + internacionais + IOF = Lançamentos atuais (resumo)", current, h.current)
		})
	}
	if h.hasCurrent && h.hasFinanced {
		steps = append(steps, func() error {
			return check("Saldo financiado + Lançamentos atuais = Total desta fatura", h.financed+h.current, h.total)
		})
	}
	if f.seen[lblNextInvoice] {
		steps = append(steps, func() error {
			return check("parcelas listadas = "+lblNextInvoice, upcomingNext, f.totals[lblNextInvoice])
		})
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}

	var warnings []string
	if f.seen[lblLaterInvoices] && projectedLater != f.totals[lblLaterInvoices] {
		warnings = append(warnings, fmt.Sprintf("projeção das parcelas restantes (%s) difere de %q (%s)",
			money.FormatBRL(projectedLater), lblLaterInvoices, money.FormatBRL(f.totals[lblLaterInvoices])))
	}
	return warnings, nil
}
