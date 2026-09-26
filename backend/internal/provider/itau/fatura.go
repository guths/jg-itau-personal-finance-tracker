package itau

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jgfinance/internal/layout"
	"jgfinance/internal/money"
	"jgfinance/internal/provider"
)

// section is the invoice block a body line belongs to.
type section int

const (
	secNone section = iota
	secPayments
	secPurchases
	secFees
	secIntl
	secUpcoming
)

// sectionStarts maps line prefixes to the section they open; secNone closes the current one.
var sectionStarts = []struct {
	prefix string
	sec    section
}{
	{"Pagamentos efetuados", secPayments},
	{"Lançamentos: compras e saques", secPurchases},
	{"Lançamentos: produtos e serviços", secFees},
	{"Lançamentos internacionais", secIntl},
	{"Compras parceladas - próximas faturas", secUpcoming},
	{"Lançamentos no cartão", secNone},
	{"Encargos cobrados", secNone},
	{"Limites de crédito", secNone},
	{"Novo teto", secNone},
	{"Simulação", secNone},
	{"Credito Rotativo", secNone},
	{"Fique atento", secNone},
}

// Labels of the totals printed in the invoice body.
const (
	lblPayments      = "P Total dos pagamentos"
	lblPurchases     = "Lançamentos no cartão"
	lblFees          = "Lançamentos produtos e serviços"
	lblIntl          = "Total transações inter. em R$"
	lblIOF           = "Repasse de IOF em R$"
	lblIntlTotal     = "Total lançamentos inter. em R$"
	lblCurrent       = "L Total dos lançamentos atuais"
	lblNextInvoice   = "Próxima fatura"
	lblLaterInvoices = "Demais faturas"
	lblUpcomingTotal = "Total para próximas faturas"
)

var (
	faturaTxRe    = regexp.MustCompile(`^(\d{2})/(\d{2})\s+(.+?)\s+(` + money.Pattern + `)$`)
	faturaTotalRe = regexp.MustCompile(`^(` + alternation(lblPayments, lblPurchases, lblFees, lblIntl, lblIOF,
		lblIntlTotal, lblCurrent, lblNextInvoice, lblLaterInvoices, lblUpcomingTotal) + `)\s+(` + money.Pattern + `)$`)
	installmentRe  = regexp.MustCompile(`\s(\d{2})/(\d{2})$`)
	intlOriginalRe = regexp.MustCompile(`^(` + money.Pattern + `)\s+([A-Z]{3})\s+` + money.Pattern + `$`)
	holderRe       = regexp.MustCompile(`^[A-ZÀ-Ý]+(?: [A-ZÀ-Ý]+)+$`)
	cardRe         = regexp.MustCompile(`Cartão\s+\d{4}\.XXXX\.XXXX\.(\d{4})`)
	tierRe         = regexp.MustCompile(`\b(Platinum|Infinite|Gold|Black|Signature|Grafite|Nanquim|Internacional)\b`)
	closingRe      = regexp.MustCompile(`Postagem:\s*(\d{2}/\d{2}/\d{4})`)
	dueRe          = regexp.MustCompile(`Vencimento:\s*(\d{2}/\d{2}/\d{4})`)
	invoiceTotalRe = regexp.MustCompile(`Total desta fatura\s+(` + money.Pattern + `)`)
	currentRe      = regexp.MustCompile(`Lançamentos atuais\s+(` + money.Pattern + `)`)
	financedRe     = regexp.MustCompile(`Saldo financiado\s+(` + money.Pattern + `)`)
)

func alternation(labels ...string) string {
	quoted := make([]string, len(labels))
	for i, l := range labels {
		quoted[i] = regexp.QuoteMeta(l)
	}
	return strings.Join(quoted, "|")
}

// faturaHeader holds the fields printed on page 1.
type faturaHeader struct {
	last4, tier             string
	closing, due            time.Time
	total                   int64
	current, financed       int64
	hasCurrent, hasFinanced bool
}

type faturaItem struct {
	sec section
	tx  provider.Transaction
}

// faturaBody is what the body pages yielded, before reconciliation.
type faturaBody struct {
	items    []faturaItem
	upcoming []provider.UpcomingInstallment
	totals   map[string]int64 // summed per label
	seen     map[string]bool
}

// parseFaturaLines turns the page-1 text and the body lines (pages 2+, reading order)
// into a Statement.
func parseFaturaLines(header string, body []layout.Line) (*provider.Statement, error) {
	h, err := parseFaturaHeader(header)
	if err != nil {
		return nil, err
	}
	f, err := parseFaturaBody(body, h.closing)
	if err != nil {
		return nil, err
	}
	return buildFaturaStatement(h, f, nil), nil
}

func parseFaturaHeader(text string) (faturaHeader, error) {
	var h faturaHeader
	m := cardRe.FindStringSubmatch(text)
	if m == nil {
		return h, &provider.LayoutError{Msg: "número do cartão não encontrado na fatura"}
	}
	h.last4 = m[1]
	if m := tierRe.FindStringSubmatch(text); m != nil {
		h.tier = m[1]
	}
	var err error
	if h.closing, err = findDate(closingRe, text, "Postagem"); err != nil {
		return h, err
	}
	if h.due, err = findDate(dueRe, text, "Vencimento"); err != nil {
		return h, err
	}
	var ok bool
	if h.total, ok, err = findAmount(invoiceTotalRe, text); err != nil {
		return h, err
	}
	if !ok {
		return h, &provider.LayoutError{Msg: `"Total desta fatura" não encontrado`}
	}
	if h.current, h.hasCurrent, err = findAmount(currentRe, text); err != nil {
		return h, err
	}
	if h.financed, h.hasFinanced, err = findAmount(financedRe, text); err != nil {
		return h, err
	}
	return h, nil
}

func findDate(re *regexp.Regexp, text, label string) (time.Time, error) {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return time.Time{}, &provider.LayoutError{Msg: fmt.Sprintf("%q não encontrado", label)}
	}
	return parseDate(m[1])
}

func findAmount(re *regexp.Regexp, text string) (int64, bool, error) {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0, false, nil
	}
	v, err := money.ParseBRL(m[1])
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func sectionStart(text string) (section, bool) {
	for _, s := range sectionStarts {
		if strings.HasPrefix(text, s.prefix) {
			return s.sec, true
		}
	}
	return secNone, false
}

func parseFaturaBody(lines []layout.Line, closing time.Time) (*faturaBody, error) {
	f := &faturaBody{totals: map[string]int64{}, seen: map[string]bool{}}
	sec, holder, prev := secNone, "", ""
	last := -1 // index in f.items of the transaction whose detail line may follow
	for _, l := range lines {
		text := l.Text
		if m := faturaTotalRe.FindStringSubmatch(text); m != nil {
			v, err := money.ParseBRL(m[2])
			if err != nil {
				return nil, err
			}
			f.totals[m[1]] += v
			f.seen[m[1]] = true
			if m[1] == lblPurchases {
				sec = secNone
			}
			prev, last = text, -1
			continue
		}
		if s, ok := sectionStart(text); ok {
			sec = s
			prev, last = text, -1
			continue
		}
		if strings.HasPrefix(text, "DATA ") {
			if (sec == secPurchases || sec == secIntl) && holderRe.MatchString(prev) {
				holder = prev
			}
			prev, last = text, -1
			continue
		}
		if m := faturaTxRe.FindStringSubmatch(text); m != nil && sec != secNone {
			idx, err := f.add(sec, m, closing, holder)
			if err != nil {
				return nil, err
			}
			prev, last = text, idx
			continue
		}
		if last >= 0 {
			f.attachDetail(last, text)
			last = -1
		}
		prev = text
	}
	return f, nil
}

// add records a transaction line; it returns the item index, or -1 for upcoming installments.
func (f *faturaBody) add(sec section, m []string, closing time.Time, holder string) (int, error) {
	date, err := cardDate(m[1], m[2], closing)
	if err != nil {
		return -1, err
	}
	amount, err := money.ParseBRL(m[4])
	if err != nil {
		return -1, err
	}
	desc := m[3]
	n, total, base := splitInstallment(desc)
	key := provider.NormalizeKey(base)
	if sec == secUpcoming {
		if n == 0 {
			return -1, &provider.LayoutError{Msg: fmt.Sprintf("parcela futura sem n/N: %q", desc)}
		}
		f.upcoming = append(f.upcoming, provider.UpcomingInstallment{
			PurchaseDate: date, Description: desc, Key: key, AmountCents: amount, N: n, Total: total,
		})
		return -1, nil
	}
	tx := provider.Transaction{
		Date: date, Description: desc, Key: key, AmountCents: amount,
		Kind: kindBySign(amount), InstallmentN: n, InstallmentTotal: total,
	}
	switch sec {
	case secPayments:
		tx.Kind = provider.BillPayment
	case secPurchases, secIntl:
		tx.Holder = holder
	}
	f.items = append(f.items, faturaItem{sec: sec, tx: tx})
	return len(f.items) - 1, nil
}

// attachDetail reads the line printed right below a transaction.
func (f *faturaBody) attachDetail(i int, text string) {
	it := &f.items[i]
	switch it.sec {
	case secPurchases:
		it.tx.BankCategory, it.tx.City, _ = strings.Cut(text, " ")
	case secIntl:
		if m := intlOriginalRe.FindStringSubmatch(text); m != nil {
			if v, err := money.ParseBRL(m[1]); err == nil {
				it.tx.OriginalAmountCents, it.tx.OriginalCurrency = v, m[2]
			}
		}
	}
}

func kindBySign(cents int64) provider.TxKind {
	if cents < 0 {
		return provider.Refund
	}
	return provider.Expense
}

// splitInstallment detects a trailing " NN/NN" installment marker (1 <= n <= total).
func splitInstallment(desc string) (n, total int, base string) {
	loc := installmentRe.FindStringSubmatchIndex(desc)
	if loc == nil {
		return 0, 0, desc
	}
	n, _ = strconv.Atoi(desc[loc[2]:loc[3]])
	total, _ = strconv.Atoi(desc[loc[4]:loc[5]])
	if n < 1 || n > total {
		return 0, 0, desc
	}
	return n, total, strings.TrimSpace(desc[:loc[0]])
}

func buildFaturaStatement(h faturaHeader, f *faturaBody, warnings []string) *provider.Statement {
	name := "Itaú •••• " + h.last4
	if h.tier != "" {
		name = "Itaú " + h.tier + " •••• " + h.last4
	}
	st := &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{Provider: "itau", Kind: provider.CreditCard, ExternalID: h.last4, DefaultName: name},
		ClosingDate: h.closing,
		DueDate:     h.due,
		TotalCents:  h.total,
		Upcoming:    f.upcoming,
		Warnings:    warnings,
	}
	for _, it := range f.items {
		st.Transactions = append(st.Transactions, it.tx)
	}
	if iof := f.totals[lblIOF]; iof != 0 {
		st.Transactions = append(st.Transactions, provider.Transaction{
			Date: h.closing, Description: lblIOF, Key: provider.NormalizeKey(lblIOF),
			AmountCents: iof, Kind: kindBySign(iof),
		})
	}
	for i := range st.Transactions {
		st.Transactions[i].Seq = i + 1
	}
	return st
}
