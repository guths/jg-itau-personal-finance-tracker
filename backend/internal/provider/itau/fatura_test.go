package itau

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"jgfinance/internal/layout"
	"jgfinance/internal/provider"
)

func toLines(ss []string) []layout.Line {
	out := make([]layout.Line, len(ss))
	for i, s := range ss {
		out[i] = layout.Line{Text: s}
	}
	return out
}

// Invented invoice (no real data) covering every section and the page noise seen in real PDFs.
var faturaHeaderFixture = strings.Join([]string{
	"Platinum",
	"FULANO DE TAL Resumo da fatura em R$",
	"RUA EXEMPLO, 100 Total da fatura anterior 1.000,00",
	"Pagamento efetuado em 01/02/2026 -1.000,00",
	"S Saldo financiado 0,00",
	"L Lançamentos atuais 1.164,89",
	"Postagem: 24/02/2026",
	"Vencimento: 03/03/2026 = Total desta fatura 1.164,89",
	"Emissão: 24/02/2026",
	"Titular FULANO DE TAL",
	"Cartão 4111.XXXX.XXXX.1234",
	"Banco Itaú S.A. 341-7",
	"ITAU UNIBANCO HOLDING S.A.",
}, "\n")

var faturaBodyFixture = []string{
	"Caso você pague um valor entre o mínimo e o total de sua fatura, o saldo restante",
	"Pagamentos efetuados",
	"DATA VALOR EM R$",
	"01/02 Pagamento via conta -1.000,00",
	"P Total dos pagamentos -1.000,00",
	"Lançamentos: compras e saques",
	"FULANO DE TAL",
	"DATA ESTABELECIMENTO VALOR EM R$",
	"15/11 LOJA EXEMPLO 03/10 150,00",
	"vestuário SAO PAULO",
	"28/01 PADARIA BOM PÃO 25,50",
	"restaurante FLORIANOPOLIS",
	"02/02 99*CORRIDA 12,90",
	"transporte SAO PAULO",
	"4004 4828",
	"0800 970 4828",
	"Lançamentos: compras e saques",
	"DATA ESTABELECIMENTO VALOR EM R$",
	"10/02 MERCADO CENTRAL 300,00",
	"supermercado PORTO BELO",
	"12/02 aliexpressLOJA 100,00",
	"serviços SAO PAULO",
	"12/02 aliexpressLOJA -40,00",
	"serviços SAO PAULO",
	"20/02 FARMACIA SAUDE 45,00",
	"saúde",
	"continua...",
	"Lançamentos no cartão 593,40",
	"Lançamentos internacionais",
	"FULANO DE TAL",
	"DATA ESTABELECIMENTO US$ R$",
	"05/02 STREAMING INC 110,00",
	"20,00 USD 20,00",
	"Dólar de Conversão R$ 5,50",
	"10/02 SOFTWARE CO 432,50",
	"400,00 BRL 80,00",
	"Dólar de Conversão R$ 5,40",
	"Total transações inter. em R$ 542,50",
	"Repasse de IOF em R$ 18,99",
	"Total lançamentos inter. em R$ 561,49",
	"Lançamentos: produtos e serviços",
	"DATA PRODUTOS/SERVIÇOS VALOR EM R$",
	"25/01 Mensalidade - Plano do 20,00",
	"Anuidade Diferenciada",
	"03/02 Redução Mensalidade - P -10,00",
	"Anuidade Diferenciada",
	"Lançamentos produtos e serviços 10,00",
	"L Total dos lançamentos atuais 1.164,89",
	"Compras parceladas - próximas faturas",
	"DATA ESTABELECIMENTO VALOR EM R$",
	"15/11 LOJA EXEMPLO 04/10 150,00",
	"Próxima fatura 150,00",
	"Demais faturas 900,00",
	"Total para próximas faturas 1.050,00",
	"Limites de crédito Valor em R$",
	"Limite total de crédito 5.000,00",
	"Encargos cobrados nesta fatura",
	"Juros do rotativo 16,10 % 0,00",
	"Total a pagar R$ 5.189,74",
}

func TestParseFaturaLines(t *testing.T) {
	st, err := parseFaturaLines(faturaHeaderFixture, toLines(faturaBodyFixture))
	if err != nil {
		t.Fatalf("parseFaturaLines: %v", err)
	}
	const holder = "FULANO DE TAL"
	want := &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{Provider: "itau", Kind: provider.CreditCard, ExternalID: "1234", DefaultName: "Itaú Platinum •••• 1234"},
		ClosingDate: day(2026, 2, 24),
		DueDate:     day(2026, 3, 3),
		TotalCents:  116489,
		Transactions: []provider.Transaction{
			{Seq: 1, Date: day(2026, 2, 1), Description: "Pagamento via conta", Key: "PAGAMENTO VIA CONTA", AmountCents: -100000, Kind: provider.BillPayment},
			{Seq: 2, Date: day(2025, 11, 15), Description: "LOJA EXEMPLO 03/10", Key: "LOJA EXEMPLO", AmountCents: 15000, Kind: provider.Expense, InstallmentN: 3, InstallmentTotal: 10, BankCategory: "vestuário", City: "SAO PAULO", Holder: holder},
			{Seq: 3, Date: day(2026, 1, 28), Description: "PADARIA BOM PÃO", Key: "PADARIA BOM PAO", AmountCents: 2550, Kind: provider.Expense, BankCategory: "restaurante", City: "FLORIANOPOLIS", Holder: holder},
			{Seq: 4, Date: day(2026, 2, 2), Description: "99*CORRIDA", Key: "99*CORRIDA", AmountCents: 1290, Kind: provider.Expense, BankCategory: "transporte", City: "SAO PAULO", Holder: holder},
			{Seq: 5, Date: day(2026, 2, 10), Description: "MERCADO CENTRAL", Key: "MERCADO CENTRAL", AmountCents: 30000, Kind: provider.Expense, BankCategory: "supermercado", City: "PORTO BELO", Holder: holder},
			{Seq: 6, Date: day(2026, 2, 12), Description: "aliexpressLOJA", Key: "ALIEXPRESSLOJA", AmountCents: 10000, Kind: provider.Expense, BankCategory: "serviços", City: "SAO PAULO", Holder: holder},
			{Seq: 7, Date: day(2026, 2, 12), Description: "aliexpressLOJA", Key: "ALIEXPRESSLOJA", AmountCents: -4000, Kind: provider.Refund, BankCategory: "serviços", City: "SAO PAULO", Holder: holder},
			{Seq: 8, Date: day(2026, 2, 20), Description: "FARMACIA SAUDE", Key: "FARMACIA SAUDE", AmountCents: 4500, Kind: provider.Expense, BankCategory: "saúde", Holder: holder},
			{Seq: 9, Date: day(2026, 2, 5), Description: "STREAMING INC", Key: "STREAMING INC", AmountCents: 11000, Kind: provider.Expense, Holder: holder, OriginalAmountCents: 2000, OriginalCurrency: "USD"},
			{Seq: 10, Date: day(2026, 2, 10), Description: "SOFTWARE CO", Key: "SOFTWARE CO", AmountCents: 43250, Kind: provider.Expense, Holder: holder, OriginalAmountCents: 40000, OriginalCurrency: "BRL"},
			{Seq: 11, Date: day(2026, 1, 25), Description: "Mensalidade - Plano do", Key: "MENSALIDADE - PLANO DO", AmountCents: 2000, Kind: provider.Expense},
			{Seq: 12, Date: day(2026, 2, 3), Description: "Redução Mensalidade - P", Key: "REDUCAO MENSALIDADE - P", AmountCents: -1000, Kind: provider.Refund},
			{Seq: 13, Date: day(2026, 2, 24), Description: "Repasse de IOF em R$", Key: "REPASSE DE IOF EM R$", AmountCents: 1899, Kind: provider.Expense},
		},
		Upcoming: []provider.UpcomingInstallment{
			{PurchaseDate: day(2025, 11, 15), Description: "LOJA EXEMPLO 04/10", Key: "LOJA EXEMPLO", AmountCents: 15000, N: 4, Total: 10},
		},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("statement mismatch (-want +got):\n%s", diff)
	}
}

func TestParseFaturaHeaderRequiresCardAndDates(t *testing.T) {
	for _, drop := range []string{"Cartão 4111.XXXX.XXXX.1234", "Postagem: 24/02/2026"} {
		header := strings.Replace(faturaHeaderFixture, drop, "", 1)
		_, err := parseFaturaLines(header, toLines(faturaBodyFixture))
		var le *provider.LayoutError
		if !errors.As(err, &le) {
			t.Errorf("without %q: err = %v, want *provider.LayoutError", drop, err)
		}
	}
}

func TestSplitInstallment(t *testing.T) {
	cases := []struct {
		in       string
		n, total int
		base     string
	}{
		{"LOJA EXEMPLO 03/10", 3, 10, "LOJA EXEMPLO"},
		{"KIWIFY*Worksho 02/04", 2, 4, "KIWIFY*Worksho"},
		{"MP*INGRESSOS949FLORIANO", 0, 0, "MP*INGRESSOS949FLORIANO"},
		{"EVENTO 31/12", 0, 0, "EVENTO 31/12"}, // 31 > 12: a date, not n/N
		{"LOJA 00/10", 0, 0, "LOJA 00/10"},
	}
	for _, c := range cases {
		n, total, base := splitInstallment(c.in)
		if n != c.n || total != c.total || base != c.base {
			t.Errorf("splitInstallment(%q) = %d, %d, %q", c.in, n, total, base)
		}
	}
}
