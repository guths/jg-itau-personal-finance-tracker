package itau

import (
	"fmt"
	"strconv"
	"time"

	"jgfinance/internal/provider"
)

// parseDate parses "dd/mm/aaaa" as a civil date in UTC.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		return time.Time{}, &provider.LayoutError{Msg: fmt.Sprintf("data inválida %q", s)}
	}
	return t, nil
}

// cardDate resolves an invoice "dd/mm" to the most recent valid date that is not after the
// closing date (invoices print purchases and installments without the year).
func cardDate(dd, mm string, closing time.Time) (time.Time, error) {
	d, errD := strconv.Atoi(dd)
	m, errM := strconv.Atoi(mm)
	if errD == nil && errM == nil {
		for _, y := range []int{closing.Year(), closing.Year() - 1} {
			t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
			if t.Day() == d && int(t.Month()) == m && !t.After(closing) {
				return t, nil
			}
		}
	}
	return time.Time{}, &provider.LayoutError{Msg: fmt.Sprintf("data inválida %s/%s", dd, mm)}
}
