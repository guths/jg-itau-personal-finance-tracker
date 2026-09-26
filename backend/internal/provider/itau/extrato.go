package itau

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"jgfinance/internal/layout"
	"jgfinance/internal/money"
	"jgfinance/internal/provider"
)

const balanceLabel = "SALDO DO DIA"

var (
	extratoRowRe = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4})\s+(.+?)\s+(` + money.Pattern + `)$`)
	periodRe     = regexp.MustCompile(`período de visualização:\s*(\d{2}/\d{2}/\d{4})\s*até\s*(\d{2}/\d{2}/\d{4})`)
	agencyRe     = regexp.MustCompile(`agência:\s*(\d+)`)
	accountNoRe  = regexp.MustCompile(`conta:\s*(\d[\d-]*)`)
	gluedDateRe  = regexp.MustCompile(`\d{2}/\d{2}$`)
)

// parseExtratoLines turns the lines of an Itaú checking account statement into a Statement,
// after reconciling every day against the printed "SALDO DO DIA".
func parseExtratoLines(lines []layout.Line) (*provider.Statement, error) {
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l.Text)
		sb.WriteByte('\n')
	}
	text := sb.String()

	agency, account := agencyRe.FindStringSubmatch(text), accountNoRe.FindStringSubmatch(text)
	if agency == nil || account == nil {
		return nil, &provider.LayoutError{Msg: "agência/conta não encontradas no extrato"}
	}

	balances := map[time.Time]int64{}
	sums := map[time.Time]int64{}
	var txs []provider.Transaction
	for _, l := range lines {
		m := extratoRowRe.FindStringSubmatch(l.Text)
		if m == nil {
			continue
		}
		date, err := parseDate(m[1])
		if err != nil {
			return nil, err
		}
		printed, err := money.ParseBRL(m[3])
		if err != nil {
			return nil, err
		}
		desc := m[2]
		if provider.NormalizeKey(desc) == balanceLabel {
			balances[date] = printed
			continue
		}
		sums[date] += printed
		txs = append(txs, provider.Transaction{
			Date:           date,
			Description:    desc,
			Key:            provider.NormalizeKey(gluedDateRe.ReplaceAllString(desc, "")),
			AmountCents:    -printed,
			Kind:           classifyExtrato(desc, printed),
			PersonTransfer: printed < 0 && strings.HasPrefix(provider.NormalizeKey(desc), "PIX TRANSF"),
		})
	}
	if err := reconcileExtrato(balances, sums); err != nil {
		return nil, err
	}
	for i := range txs {
		txs[i].Seq = i + 1
	}

	st := &provider.Statement{
		Kind: provider.BankStatement,
		Account: provider.AccountRef{
			Provider: "itau", Kind: provider.Checking,
			ExternalID: agency[1] + "/" + account[1], DefaultName: "Itaú conta " + account[1],
		},
		Transactions: txs,
	}
	if m := periodRe.FindStringSubmatch(text); m != nil {
		start, err := parseDate(m[1])
		if err != nil {
			return nil, err
		}
		end, err := parseDate(m[2])
		if err != nil {
			return nil, err
		}
		st.PeriodStart, st.PeriodEnd = start, end
	} else {
		dates := sortedDates(balances)
		st.PeriodStart, st.PeriodEnd = dates[0], dates[len(dates)-1]
	}
	return st, nil
}

// classifyExtrato decides the kind of a statement line from its description and printed sign.
func classifyExtrato(desc string, printed int64) provider.TxKind {
	k := provider.NormalizeKey(desc)
	hasPrefix := func(prefixes ...string) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				return true
			}
		}
		return false
	}
	switch {
	case hasPrefix("FATURA PAGA"):
		return provider.BillPayment
	case hasPrefix("APLICACAO", "RESGATE"):
		return provider.Investment
	case hasPrefix("JUROS POUP", "REMUNER BASICA POUP", "REND PAGO", "RENDIMENTO"):
		return provider.Income
	case hasPrefix("DEV PIX") || strings.Contains(k, "ESTORNO"):
		return provider.Refund
	case printed < 0:
		return provider.Expense
	default:
		return provider.Income
	}
}

// reconcileExtrato checks balance[d] = balance[previous] + sum of printed amounts on d.
func reconcileExtrato(balances, sums map[time.Time]int64) error {
	if len(balances) == 0 {
		return &provider.LayoutError{Msg: "nenhuma linha SALDO DO DIA no extrato"}
	}
	for d := range sums {
		if _, ok := balances[d]; !ok {
			return &provider.LayoutError{Msg: fmt.Sprintf("lançamentos em %s sem linha SALDO DO DIA", d.Format("02/01/2006"))}
		}
	}
	dates := sortedDates(balances)
	if _, ok := sums[dates[0]]; ok {
		return &provider.LayoutError{Msg: fmt.Sprintf("lançamentos em %s não podem ser conferidos (sem saldo anterior)", dates[0].Format("02/01/2006"))}
	}
	for i := 1; i < len(dates); i++ {
		prev, d := dates[i-1], dates[i]
		if got := balances[prev] + sums[d]; got != balances[d] {
			return &provider.ReconciliationError{Rule: "saldo do dia " + d.Format("02/01/2006"), Computed: got, Printed: balances[d]}
		}
	}
	return nil
}

func sortedDates(m map[time.Time]int64) []time.Time {
	dates := make([]time.Time, 0, len(m))
	for d := range m {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	return dates
}
