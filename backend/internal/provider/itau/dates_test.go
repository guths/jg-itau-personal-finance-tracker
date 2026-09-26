package itau

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseDate(t *testing.T) {
	got, err := parseDate("24/09/2026")
	if err != nil || !got.Equal(day(2026, 9, 24)) {
		t.Errorf("parseDate = %v, %v", got, err)
	}
	if _, err := parseDate("31/02/2026"); err == nil {
		t.Error("31/02/2026 should fail")
	}
}

func TestCardDate(t *testing.T) {
	cases := []struct {
		dd, mm  string
		closing time.Time
		want    time.Time
	}{
		{"28", "12", day(2026, 1, 10), day(2025, 12, 28)}, // December purchase on a January invoice
		{"05", "01", day(2026, 1, 10), day(2026, 1, 5)},
		{"10", "01", day(2026, 1, 10), day(2026, 1, 10)},  // same day as closing
		{"15", "11", day(2026, 2, 24), day(2025, 11, 15)}, // installment bought last year
		{"29", "02", day(2029, 1, 10), day(2028, 2, 29)},  // leap day only exists in the previous year
	}
	for _, c := range cases {
		got, err := cardDate(c.dd, c.mm, c.closing)
		if err != nil || !got.Equal(c.want) {
			t.Errorf("cardDate(%s/%s, %s) = %v, %v; want %v", c.dd, c.mm, c.closing.Format("02/01/2006"), got, err, c.want)
		}
	}
	if _, err := cardDate("31", "02", day(2026, 3, 10)); err == nil {
		t.Error("31/02 should fail")
	}
}
