package provider

import (
	"sort"
	"strings"
)

// pixPrefixes are the bank-statement prefixes of a PIX payment and of its refund ("DEV PIX");
// ReversalKey drops them so "DEV PIX <NOME>" meets "PIX QRS <NOME>" and "PIX TRANSF <NOME>".
var pixPrefixes = []string{"DEV PIX ", "PIX QRS ", "PIX TRANSF "}

// ReversalKey is the key under which a purchase and its refund are matched: the transaction
// Key, with the PIX prefixes of a bank statement reduced to "PIX <NOME>".
func ReversalKey(key string) string {
	for _, p := range pixPrefixes {
		if strings.HasPrefix(key, p) {
			return "PIX " + strings.TrimSpace(key[len(p):])
		}
	}
	return key
}

// MarkReversals finds purchases cancelled by refunds in the transactions of ONE document and
// sets their Kind to Reversed; it returns how many were marked. Only Expense and Refund count,
// grouped by (Date, ReversalKey); a positive amount is a purchase and a negative one a refund.
//
//   - A group with purchases and refunds that sums to exactly zero is reversed as a whole
//     (e.g. +419,86, −17,42 and −402,44 on the same day).
//   - Otherwise each refund, in Seq order, cancels the first remaining purchase of exactly the
//     same absolute amount; the rest of the group is unchanged.
//
// Different dates never cancel each other. Running it again changes nothing.
func MarkReversals(txs []Transaction) int {
	type groupKey struct{ day, key string }
	groups := map[groupKey][]int{}
	var order []groupKey
	for i, t := range txs {
		if t.Kind != Expense && t.Kind != Refund {
			continue
		}
		k := groupKey{t.Date.Format("2006-01-02"), ReversalKey(t.Key)}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], i)
	}
	marked := 0
	for _, k := range order {
		idx := groups[k]
		sort.SliceStable(idx, func(a, b int) bool { return txs[idx[a]].Seq < txs[idx[b]].Seq })
		var sum int64
		var pos, neg bool
		for _, i := range idx {
			sum += txs[i].AmountCents
			pos = pos || txs[i].AmountCents > 0
			neg = neg || txs[i].AmountCents < 0
		}
		if !pos || !neg {
			continue
		}
		if sum == 0 {
			for _, i := range idx {
				txs[i].Kind = Reversed
			}
			marked += len(idx)
			continue
		}
		used := map[int]bool{}
		for _, r := range idx {
			if txs[r].AmountCents >= 0 {
				continue
			}
			for _, p := range idx {
				if !used[p] && txs[p].AmountCents == -txs[r].AmountCents {
					used[p] = true
					txs[p].Kind, txs[r].Kind = Reversed, Reversed
					marked += 2
					break
				}
			}
		}
	}
	return marked
}
