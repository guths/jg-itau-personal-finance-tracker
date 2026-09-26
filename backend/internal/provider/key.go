package provider

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeKey uppercases s, removes accents and collapses whitespace. Providers call it
// after stripping their own noise (installment markers, glued dates) to build Transaction.Key.
func NormalizeKey(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	if out, _, err := transform.String(t, s); err == nil {
		s = out
	}
	return strings.Join(strings.Fields(strings.ToUpper(s)), " ")
}
