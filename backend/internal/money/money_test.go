package money

import "testing"

func TestParseBRL(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0,00", 0},
		{"12,96", 1296},
		{"1.234,56", 123456},
		{"-70,00", -7000},
		{"-19.000,00", -1900000},
		{"43.271,94", 4327194},
		{" 5,40 ", 540},
		{"-0,33", -33},
	}
	for _, c := range cases {
		got, err := ParseBRL(c.in)
		if err != nil {
			t.Errorf("ParseBRL(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseBRL(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseBRLRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "12", "12,5", "12.50", "1,234.56", "1.23,45", "R$ 12,00", "abc"} {
		if _, err := ParseBRL(in); err == nil {
			t.Errorf("ParseBRL(%q) should fail", in)
		}
	}
}

func TestFormatBRL(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "R$ 0,00"},
		{5, "R$ 0,05"},
		{1296, "R$ 12,96"},
		{123456, "R$ 1.234,56"},
		{-7000, "-R$ 70,00"},
		{100000000, "R$ 1.000.000,00"},
	}
	for _, c := range cases {
		if got := FormatBRL(c.in); got != c.want {
			t.Errorf("FormatBRL(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
