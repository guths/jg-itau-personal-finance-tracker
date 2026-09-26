// Package money converts Brazilian-formatted amounts ("1.234,56") to integer cents and back.
// Amounts are always int64 cents; floats are never used for money.
package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Pattern matches a printed BRL amount such as "12,96", "1.234,56" or "-70,00".
const Pattern = `-?\d{1,3}(?:\.\d{3})*,\d{2}`

var exact = regexp.MustCompile(`^` + Pattern + `$`)

// ParseBRL converts a printed amount to cents. Surrounding spaces are ignored.
func ParseBRL(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !exact.MatchString(s) {
		return 0, fmt.Errorf("valor em reais inválido: %q", s)
	}
	return strconv.ParseInt(strings.NewReplacer(".", "", ",", "").Replace(s), 10, 64)
}

// FormatBRL renders cents as "R$ 1.234,56" (negative values as "-R$ 70,00").
func FormatBRL(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, b.String(), cents%100)
}
