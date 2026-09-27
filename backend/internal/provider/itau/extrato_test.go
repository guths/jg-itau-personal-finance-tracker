package itau

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"jgfinance/internal/provider"
)

// Invented statement (no real data), newest first like the real one. Arithmetic:
// 500,00 → 02/03: -200,00 -150,50 +1.000,00 = 1.149,50 → 03/03: -800,00 +0,33 +50,00 = 399,83
// → 04/03: -20,00 +20,00 -35,00 -35,00 = 329,83 → 05/03: no transactions = 329,83.
var extratoFixture = []string{
	"saldo em conta",
	"extrato conta / lançamentos",
	"período de visualização: 01/03/2026 até 05/03/2026",
	"05/03/2026 SALDO DO DIA 329,83",
	"04/03/2026 PIX QRS PADARIA BO04/03 -20,00",
	"04/03/2026 DEV PIX PADARIA BO04/03 20,00",
	"04/03/2026 PIX TRANSF MARIA S04/03 -35,00",
	"04/03/2026 PIX TRANSF MARIA S04/03 -35,00",
	"04/03/2026 SALDO DO DIA 329,83",
	"03/03/2026 FATURA PAGA CARTAO X -800,00",
	"03/03/2026 JUROS POUP AUT 0001 0,33",
	"03/03/2026 PIX TRANSF Joâo P03/03 50,00",
	"03/03/2026 SALDO DO DIA 399,83",
	"02/03/2026 PIX TRANSF Fulána02/03 -200,00",
	"02/03/2026 PIX QRS LUZ ENERGIA02/03 -150,50",
	"02/03/2026 RESGATE CDB DI 1.000,00",
	"02/03/2026 SALDO DO DIA 1.149,50",
	"28/02/2026 SALDO DO DIA 500,00",
	"FULANO DE TAL",
	"agência: 1234",
	"conta: 00000-1",
	"Aviso! Os saldos acima são baseados nas informações disponíveis. www.itau.com.br",
}

func TestParseExtratoLines(t *testing.T) {
	st, err := parseExtratoLines(toLines(extratoFixture))
	if err != nil {
		t.Fatalf("parseExtratoLines: %v", err)
	}
	d4, d3, d2 := day(2026, 3, 4), day(2026, 3, 3), day(2026, 3, 2)
	want := &provider.Statement{
		Kind:        provider.BankStatement,
		Account:     provider.AccountRef{Provider: "itau", Kind: provider.Checking, ExternalID: "1234/00000-1", DefaultName: "Itaú conta 00000-1"},
		PeriodStart: day(2026, 3, 1),
		PeriodEnd:   day(2026, 3, 5),
		Transactions: []provider.Transaction{
			{Seq: 1, Date: d4, Description: "PIX QRS PADARIA BO04/03", Key: "PIX QRS PADARIA BO", AmountCents: 2000, Kind: provider.Expense},
			{Seq: 2, Date: d4, Description: "DEV PIX PADARIA BO04/03", Key: "DEV PIX PADARIA BO", AmountCents: -2000, Kind: provider.Refund},
			{Seq: 3, Date: d4, Description: "PIX TRANSF MARIA S04/03", Key: "PIX TRANSF MARIA S", AmountCents: 3500, Kind: provider.Expense, PersonTransfer: true},
			{Seq: 4, Date: d4, Description: "PIX TRANSF MARIA S04/03", Key: "PIX TRANSF MARIA S", AmountCents: 3500, Kind: provider.Expense, PersonTransfer: true},
			{Seq: 5, Date: d3, Description: "FATURA PAGA CARTAO X", Key: "FATURA PAGA CARTAO X", AmountCents: 80000, Kind: provider.BillPayment},
			{Seq: 6, Date: d3, Description: "JUROS POUP AUT 0001", Key: "JUROS POUP AUT 0001", AmountCents: -33, Kind: provider.Income},
			{Seq: 7, Date: d3, Description: "PIX TRANSF Joâo P03/03", Key: "PIX TRANSF JOAO P", AmountCents: -5000, Kind: provider.Income},
			{Seq: 8, Date: d2, Description: "PIX TRANSF Fulána02/03", Key: "PIX TRANSF FULANA", AmountCents: 20000, Kind: provider.Expense, PersonTransfer: true},
			{Seq: 9, Date: d2, Description: "PIX QRS LUZ ENERGIA02/03", Key: "PIX QRS LUZ ENERGIA", AmountCents: 15050, Kind: provider.Expense},
			{Seq: 10, Date: d2, Description: "RESGATE CDB DI", Key: "RESGATE CDB DI", AmountCents: -100000, Kind: provider.Investment},
		},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("statement mismatch (-want +got):\n%s", diff)
	}
}

func TestParseExtratoReconciliationFails(t *testing.T) {
	lines := replaceLine(t, extratoFixture, "02/03/2026 PIX QRS LUZ ENERGIA02/03 -150,50", "02/03/2026 PIX QRS LUZ ENERGIA02/03 -150,00")
	_, err := parseExtratoLines(toLines(lines))
	wantReconciliation(t, err, "saldo do dia 02/03/2026", 115000, 114950)
}

func TestParseExtratoLayoutErrors(t *testing.T) {
	cases := map[string][]string{
		"transaction date without daily balance":  replaceLine(t, extratoFixture, "04/03/2026 SALDO DO DIA 329,83", ""),
		"transaction on the opening balance date": append(append([]string{}, extratoFixture...), "28/02/2026 PIX TRANSF X28/02 -10,00"),
		"missing account":                         replaceLine(t, extratoFixture, "conta: 00000-1", ""),
	}
	for name, lines := range cases {
		_, err := parseExtratoLines(toLines(lines))
		var le *provider.LayoutError
		if !errors.As(err, &le) {
			t.Errorf("%s: err = %v, want *provider.LayoutError", name, err)
		}
	}
}

func TestClassifyExtrato(t *testing.T) {
	cases := []struct {
		desc    string
		printed int64
		want    provider.TxKind
	}{
		{"FATURA PAGA Itau Uniclas", -100, provider.BillPayment},
		{"APLICACAO CDB DI", -100, provider.Investment},
		{"RESGATE CDB DI", 100, provider.Investment},
		{"JUROS POUP AUT 0005000", 33, provider.Income},
		{"REMUNER BASICA POUP AUT 0001693", 11, provider.Income},
		{"REND PAGO APLIC AUT", 50, provider.Income},
		{"DEV PIX LOJA X02/09", 4200, provider.Refund},
		{"ESTORNO TARIFA", 500, provider.Refund},
		{"PAG BOLETO ENERGIA", -9000, provider.Expense},
		{"TED RECEBIDA", 9000, provider.Income},
	}
	for _, c := range cases {
		if got := classifyExtrato(c.desc, c.printed); got != c.want {
			t.Errorf("classifyExtrato(%q, %d) = %s, want %s", c.desc, c.printed, got, c.want)
		}
	}
}

// A weekend purchase is printed with the next business day's date, after the last "SALDO DO DIA",
// but that last balance already includes it (real case: PIX on Saturday 26/09 printed as 28/09).
func TestParseExtratoTransactionAfterLastBalance(t *testing.T) {
	lines := append([]string{"06/03/2026 PIX QRS MERCADO X05/03 -29,83"}, extratoFixture...)
	lines = replaceLine(t, lines, "05/03/2026 SALDO DO DIA 329,83", "05/03/2026 SALDO DO DIA 300,00")
	st, err := parseExtratoLines(toLines(lines))
	if err != nil {
		t.Fatalf("parseExtratoLines: %v", err)
	}
	first := st.Transactions[0]
	if !first.Date.Equal(day(2026, 3, 6)) || first.AmountCents != 2983 || first.Key != "PIX QRS MERCADO X" {
		t.Errorf("first transaction = %+v", first)
	}

	wrong := replaceLine(t, lines, "05/03/2026 SALDO DO DIA 300,00", "05/03/2026 SALDO DO DIA 329,83")
	_, err = parseExtratoLines(toLines(wrong))
	wantReconciliation(t, err, "saldo do dia 05/03/2026", 30000, 32983)
}
