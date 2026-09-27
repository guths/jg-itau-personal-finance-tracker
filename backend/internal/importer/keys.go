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
