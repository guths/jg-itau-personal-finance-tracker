package itau

import (
	"errors"
	"strings"
	"testing"

	"jgfinance/internal/provider"
)

// replaceLine returns a copy of lines with the first line equal to old replaced (or removed when repl is "").
func replaceLine(t *testing.T, lines []string, old, repl string) []string {
	t.Helper()
	out := make([]string, 0, len(lines))
	found := false
	for _, l := range lines {
		if !found && l == old {
			found = true
			if repl != "" {
				out = append(out, repl)
			}
			continue
		}
		out = append(out, l)
	}
	if !found {
		t.Fatalf("fixture line %q not found", old)
	}
	return out
}

func wantReconciliation(t *testing.T, err error, ruleContains string, computed, printed int64) {
	t.Helper()
	var re *provider.ReconciliationError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *provider.ReconciliationError", err)
	}
	if !strings.Contains(re.Rule, ruleContains) || re.Computed != computed || re.Printed != printed {
		t.Errorf("got %+v, want rule containing %q, computed %d, printed %d", re, ruleContains, computed, printed)
	}
}

func TestReconcileFaturaFixturePasses(t *testing.T) {
	st, err := parseFaturaLines(faturaHeaderFixture, toLines(faturaBodyFixture))
	if err != nil {
		t.Fatalf("fixture must reconcile: %v", err)
	}
	if st.Warnings != nil {
		t.Errorf("warnings = %v, want none", st.Warnings)
	}
}

func TestReconcileFaturaPurchasesMismatch(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "28/01 PADARIA BOM PÃO 25,50", "28/01 PADARIA BOM PÃO 26,50")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	wantReconciliation(t, err, lblPurchases, 59440, 59340)
}

func TestReconcileFaturaPurchasesWithoutTotalIsLayoutError(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Lançamentos no cartão 593,40", "")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	var le *provider.LayoutError
	if !errors.As(err, &le) {
		t.Errorf("err = %v, want *provider.LayoutError", err)
	}
}

func TestReconcileFaturaIOFMismatch(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Total lançamentos inter. em R$ 561,49", "Total lançamentos inter. em R$ 561,50")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	wantReconciliation(t, err, lblIntlTotal, 56149, 56150)
}

func TestReconcileFaturaHeaderTotalMismatch(t *testing.T) {
	header := strings.Replace(faturaHeaderFixture, "Total desta fatura 1.164,89", "Total desta fatura 1.164,90", 1)
	_, err := parseFaturaLines(header, toLines(faturaBodyFixture))
	wantReconciliation(t, err, "Total desta fatura", 116489, 116490)
}

func TestReconcileFaturaNextInvoiceMismatch(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Próxima fatura 150,00", "Próxima fatura 151,00")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	wantReconciliation(t, err, lblNextInvoice, 15000, 15100)
}

func TestReconcileFaturaLaterInvoicesOnlyWarns(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Demais faturas 900,00", "Demais faturas 899,00")
	st, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	if err != nil {
		t.Fatalf("projection mismatch must not fail the import: %v", err)
	}
	if len(st.Warnings) != 1 || !strings.Contains(st.Warnings[0], "Demais faturas") {
		t.Errorf("warnings = %v", st.Warnings)
	}
}

// Review focus: a second cardholder with its own subtotal, and no international/fees sections.
func TestParseFaturaTwoHolders(t *testing.T) {
	header := strings.Join([]string{
		"Resumo da fatura em R$",
		"S Saldo financiado 0,00",
		"L Lançamentos atuais 380,00",
		"Postagem: 24/02/2026",
		"Vencimento: 03/03/2026 = Total desta fatura 380,00",
		"Cartão 4111.XXXX.XXXX.1234",
	}, "\n")
	body := []string{
		"Lançamentos: compras e saques",
		"FULANO DE TAL",
		"DATA ESTABELECIMENTO VALOR EM R$",
		"10/02 MERCADO CENTRAL 300,00",
		"supermercado PORTO BELO",
		"Lançamentos no cartão 300,00",
		"Lançamentos: compras e saques",
		"CICLANA DE TAL",
		"DATA ESTABELECIMENTO VALOR EM R$",
		"11/02 LIVRARIA X 80,00",
		"educação SAO PAULO",
		"Lançamentos no cartão 80,00",
		"L Total dos lançamentos atuais 380,00",
	}
	st, err := parseFaturaLines(header, toLines(body))
	if err != nil {
		t.Fatalf("parseFaturaLines: %v", err)
	}
	if len(st.Transactions) != 2 || st.Transactions[0].Holder != "FULANO DE TAL" || st.Transactions[1].Holder != "CICLANA DE TAL" {
		t.Errorf("transactions = %+v", st.Transactions)
	}
	if st.Account.DefaultName != "Itaú •••• 1234" {
		t.Errorf("DefaultName = %q", st.Account.DefaultName)
	}
}
