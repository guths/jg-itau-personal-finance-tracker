package provider

import (
	"reflect"
	"testing"
	"time"
)

func day(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }

// rtx builds a transaction whose kind follows the sign of the amount.
func rtx(seq, d int, key string, cents int64) Transaction {
	kind := Expense
	if cents < 0 {
		kind = Refund
	}
	return Transaction{Seq: seq, Date: day(d), Key: key, AmountCents: cents, Kind: kind}
}

func kinds(txs []Transaction) []TxKind {
	out := make([]TxKind, len(txs))
	for i, t := range txs {
		out[i] = t.Kind
	}
	return out
}

func TestMarkReversals(t *testing.T) {
	const ali = "ALIEXPRESSSAO PAULOBR"
	cases := []struct {
		name   string
		txs    []Transaction
		want   []TxKind
		marked int
	}{
		{
			name: "compra e dois estornos no mesmo dia se anulam (fatura real)",
			txs:  []Transaction{rtx(1, 22, ali, 41986), rtx(2, 22, ali, -1742), rtx(3, 22, ali, -40244)},
			want: []TxKind{Reversed, Reversed, Reversed}, marked: 3,
		},
		{
			name: "só o par de mesmo valor é anulado quando o grupo não zera",
			txs:  []Transaction{rtx(1, 10, "LOJA", 5000), rtx(2, 10, "LOJA", 3000), rtx(3, 10, "LOJA", -3000)},
			want: []TxKind{Expense, Reversed, Reversed}, marked: 2,
		},
		{
			name: "pareamento guloso e determinístico pela ordem",
			txs:  []Transaction{rtx(1, 10, "LOJA", 100), rtx(2, 10, "LOJA", 100), rtx(3, 10, "LOJA", -100), rtx(4, 10, "LOJA", 700)},
			want: []TxKind{Reversed, Expense, Reversed, Expense}, marked: 2,
		},
		{
			name: "datas diferentes nunca se anulam",
			txs:  []Transaction{rtx(1, 21, ali, 41986), rtx(2, 22, ali, -1742), rtx(3, 22, ali, -40244)},
			want: []TxKind{Expense, Refund, Refund},
		},
		{
			name: "chaves diferentes nunca se anulam",
			txs:  []Transaction{rtx(1, 10, "LOJA A", 5000), rtx(2, 10, "LOJA B", -5000)},
			want: []TxKind{Expense, Refund},
		},
		{
			name: "estorno parcial sem valor igual continua gasto",
			txs:  []Transaction{rtx(1, 10, "LOJA", 5000), rtx(2, 10, "LOJA", -1000)},
			want: []TxKind{Expense, Refund},
		},
		{
			name: "só gastos, sem estorno, nada muda",
			txs:  []Transaction{rtx(1, 10, "LOJA", 5000), rtx(2, 10, "LOJA", 5000)},
			want: []TxKind{Expense, Expense},
		},
		{
			name: "devolução de PIX do extrato anula o PIX QR Code",
			txs:  []Transaction{rtx(1, 2, "PIX QRS DAL SOTO SO", 4200), rtx(2, 2, "DEV PIX DAL SOTO SO", -4200)},
			want: []TxKind{Reversed, Reversed}, marked: 2,
		},
		{
			name: "devolução de PIX do extrato anula a transferência PIX",
			txs:  []Transaction{rtx(1, 2, "PIX TRANSF LAURA N", 15000), rtx(2, 2, "DEV PIX LAURA N", -15000)},
			want: []TxKind{Reversed, Reversed}, marked: 2,
		},
		{
			name: "devolução de PIX de outra pessoa não anula",
			txs:  []Transaction{rtx(1, 2, "PIX QRS DAL SOTO SO", 4200), rtx(2, 2, "DEV PIX OUTRA PESSOA", -4200)},
			want: []TxKind{Expense, Refund},
		},
		{
			name: "só gasto e estorno entram na conta",
			txs: []Transaction{
				rtx(1, 10, "LOJA", 5000),
				{Seq: 2, Date: day(10), Key: "LOJA", AmountCents: -5000, Kind: Income},
				{Seq: 3, Date: day(10), Key: "LOJA", AmountCents: -5000, Kind: Ignored},
			},
			want: []TxKind{Expense, Income, Ignored},
		},
		{
			name: "estornos já anulados ficam de fora",
			txs:  []Transaction{rtx(1, 10, "LOJA", 5000), {Seq: 2, Date: day(10), Key: "LOJA", AmountCents: -5000, Kind: Reversed}},
			want: []TxKind{Expense, Reversed},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			txs := append([]Transaction(nil), c.txs...)
			n := MarkReversals(txs)
			if got := kinds(txs); !reflect.DeepEqual(got, c.want) {
				t.Errorf("kinds = %v, want %v", got, c.want)
			}
			if n != c.marked {
				t.Errorf("marked = %d, want %d", n, c.marked)
			}
			for i := range txs { // only Kind changes
				txs[i].Kind = c.txs[i].Kind
			}
			if !reflect.DeepEqual(txs, c.txs) {
				t.Errorf("other fields changed: %+v", txs)
			}
		})
	}
}

func TestMarkReversalsIsIdempotentAndOrderIndependent(t *testing.T) {
	txs := []Transaction{rtx(3, 10, "LOJA", -3000), rtx(1, 10, "LOJA", 5000), rtx(2, 10, "LOJA", 3000)}
	if n := MarkReversals(txs); n != 2 {
		t.Fatalf("marked = %d", n)
	}
	if want := []TxKind{Reversed, Expense, Reversed}; !reflect.DeepEqual(kinds(txs), want) {
		t.Fatalf("kinds = %v, want %v", kinds(txs), want)
	}
	if n := MarkReversals(txs); n != 0 {
		t.Errorf("second run marked %d", n)
	}
}

func TestReversalKey(t *testing.T) {
	cases := map[string]string{
		"DEV PIX DAL SOTO SO":    "PIX DAL SOTO SO",
		"PIX QRS DAL SOTO SO":    "PIX DAL SOTO SO",
		"PIX TRANSF DAL SOTO SO": "PIX DAL SOTO SO",
		"ALIEXPRESSSAO PAULOBR":  "ALIEXPRESSSAO PAULOBR",
		"DAL SOTO SO":            "DAL SOTO SO",
	}
	for in, want := range cases {
		if got := ReversalKey(in); got != want {
			t.Errorf("ReversalKey(%q) = %q, want %q", in, got, want)
		}
	}
}
