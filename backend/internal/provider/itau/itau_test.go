package itau

import (
	"math"
	"testing"

	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
)

// page lays out each column's lines as one run per line (x = 40 or 352), 12pt apart.
func page(cols ...[]string) pdftext.Page {
	p := pdftext.Page{Width: 595.28, Height: 841.89}
	for c, lines := range cols {
		x := 40.0
		if c == 1 {
			x = 352
		}
		for i, text := range lines {
			y := 60 + 12*float64(i)
			p.Runs = append(p.Runs, pdftext.Run{Text: text, X0: x, Y0: y - 4, X1: x + 4.5*float64(len(text)), Y1: y + 4})
		}
	}
	return p
}

func TestDetect(t *testing.T) {
	fatura := &pdftext.Document{Pages: []pdftext.Page{page([]string{"Resumo da fatura em R$", "Total desta fatura 10,00", "Banco Itaú S.A."})}}
	extrato := &pdftext.Document{Pages: []pdftext.Page{page([]string{"extrato conta / lançamentos", "02/03/2026 SALDO DO DIA 1,00", "www.itau.com.br"})}}
	other := &pdftext.Document{Pages: []pdftext.Page{page([]string{"Nubank fatura"})}}
	p := New()
	if k, ok := p.Detect(fatura); !ok || k != provider.CardStatement {
		t.Errorf("fatura: %v %v", k, ok)
	}
	if k, ok := p.Detect(extrato); !ok || k != provider.BankStatement {
		t.Errorf("extrato: %v %v", k, ok)
	}
	if _, ok := p.Detect(other); ok {
		t.Error("other document must not be detected")
	}
}

func TestFaturaSplit(t *testing.T) {
	two := &pdftext.Document{Pages: []pdftext.Page{page([]string{"capa"}), page([]string{"DATA VALOR EM R$"}, []string{"DATA ESTABELECIMENTO VALOR EM R$"})}}
	if got := faturaSplit(two); got != 337 {
		t.Errorf("split = %v, want 337", got)
	}
	one := &pdftext.Document{Pages: []pdftext.Page{page([]string{"capa"}), page([]string{"DATA ESTABELECIMENTO VALOR EM R$"})}}
	if got := faturaSplit(one); math.Abs(got-595.28*0.565) > 0.01 {
		t.Errorf("fallback split = %v", got)
	}
}

func TestParseCardStatementFromTwoColumnDocument(t *testing.T) {
	header := page([]string{
		"Platinum",
		"Resumo da fatura em R$",
		"S Saldo financiado 0,00",
		"L Lançamentos atuais 30,00",
		"Postagem: 24/02/2026",
		"Vencimento: 03/03/2026 = Total desta fatura 30,00",
		"Cartão 4111.XXXX.XXXX.1234",
		"Banco Itaú S.A.",
	})
	body := page(
		[]string{
			"Pagamentos efetuados",
			"DATA VALOR EM R$",
			"01/02 Pagamento via conta -100,00",
			"P Total dos pagamentos -100,00",
			"Lançamentos: compras e saques",
			"FULANO DE TAL",
			"DATA ESTABELECIMENTO VALOR EM R$",
			"05/02 LOJA A 10,00",
			"vestuário SAO PAULO",
		},
		[]string{
			"Lançamentos: compras e saques",
			"DATA ESTABELECIMENTO VALOR EM R$",
			"06/02 LOJA B 20,00",
			"lazer SAO PAULO",
			"Lançamentos no cartão 30,00",
			"L Total dos lançamentos atuais 30,00",
		},
	)
	doc := &pdftext.Document{Pages: []pdftext.Page{header, body}}
	kind, ok := New().Detect(doc)
	if !ok || kind != provider.CardStatement {
		t.Fatalf("Detect = %v, %v", kind, ok)
	}
	st, err := New().Parse(doc, kind)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(st.Transactions) != 3 || st.Transactions[1].Description != "LOJA A" || st.Transactions[2].Description != "LOJA B" ||
		st.Transactions[2].Holder != "FULANO DE TAL" || st.Transactions[2].BankCategory != "lazer" {
		t.Errorf("transactions = %+v", st.Transactions)
	}
}
