package importer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"jgfinance/internal/provider"
)

// monthOf returns the first day of t's month (UTC).
func monthOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// referenceMonth is the month a transaction counts in (spec §8.2): the closing month for
// card statements, the transaction's own month for bank statements.
func referenceMonth(st *provider.Statement, tx provider.Transaction) time.Time {
	if st.Kind == provider.CardStatement {
		return monthOf(st.ClosingDate)
	}
	return monthOf(tx.Date)
}

// documentMonth is the month stored on the document: closing month or end of the period.
func documentMonth(st *provider.Statement) time.Time {
	if st.Kind == provider.CardStatement {
		return monthOf(st.ClosingDate)
	}
	return monthOf(st.PeriodEnd)
}

// dedupeKeys computes the dedupe_key of every transaction (spec §8.2). Card lines are
// identified by position within the invoice; bank lines by content plus the occurrence
// number among identical lines of the same document, so overlapping statements collapse.
func dedupeKeys(accountID int64, st *provider.Statement) []string {
	keys := make([]string, len(st.Transactions))
	seen := map[string]int{}
	for i, tx := range st.Transactions {
		var raw string
		if st.Kind == provider.CardStatement {
			raw = fmt.Sprintf("card|%d|%s|%d", accountID, day(st.ClosingDate), tx.Seq)
		} else {
			id := fmt.Sprintf("%s|%s|%d", day(tx.Date), tx.Description, tx.AmountCents)
			seen[id]++
			raw = fmt.Sprintf("bank|%d|%s|%d", accountID, id, seen[id])
		}
		sum := sha256.Sum256([]byte(raw))
		keys[i] = hex.EncodeToString(sum[:])
	}
	return keys
}

func day(t time.Time) string { return t.Format("2006-01-02") }

// projectedInstallment is one future installment row (spec §8.3).
type projectedInstallment struct {
	Key, Description string
	PurchaseDate     time.Time
	DueMonth         time.Time
	N, Total         int
	AmountCents      int64
}

// projectInstallments expands each upcoming n/N item of an invoice with reference month m into
// installments n…N, installment n+k falling in month m+1+k, all with the listed amount.
func projectInstallments(m time.Time, upcoming []provider.UpcomingInstallment) []projectedInstallment {
	var out []projectedInstallment
	for _, u := range upcoming {
		n, total := u.N, u.Total
		if n < 1 || total < n {
			n, total = 1, 1
		}
		for k := 0; n+k <= total; k++ {
			out = append(out, projectedInstallment{
				Key: u.Key, Description: u.Description, PurchaseDate: u.PurchaseDate,
				DueMonth: m.AddDate(0, 1+k, 0), N: n + k, Total: total, AmountCents: u.AmountCents,
			})
		}
	}
	return out
}
