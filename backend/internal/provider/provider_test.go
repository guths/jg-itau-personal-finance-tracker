package provider

import (
	"errors"
	"testing"

	"jgfinance/internal/pdftext"
)

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"PIX TRANSF Cassiê ":         "PIX TRANSF CASSIE",
		"Antônio":                    "ANTONIO",
		"DL    *UberRidesS":          "DL *UBERRIDESS",
		"  Redução Mensalidade - P ": "REDUCAO MENSALIDADE - P",
		"aliexpressSAO PAULOBR":      "ALIEXPRESSSAO PAULOBR",
		"Repasse de IOF em R$":       "REPASSE DE IOF EM R$",
		"PADARIA BOM PÃO":            "PADARIA BOM PAO",
	}
	for in, want := range cases {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReconciliationErrorMessage(t *testing.T) {
	err := &ReconciliationError{Rule: "compras = Lançamentos no cartão", Computed: 395466, Printed: 405286}
	want := "conferência falhou (compras = Lançamentos no cartão): calculado R$ 3.954,66, impresso R$ 4.052,86, diferença -R$ 98,20"
	if err.Error() != want {
		t.Errorf("got %q\nwant %q", err.Error(), want)
	}
	if got := (&LayoutError{Msg: "x"}).Error(); got != "layout inesperado: x" {
		t.Errorf("LayoutError = %q", got)
	}
}

type fakeProvider struct {
	id    string
	kind  DocKind
	match string
}

func (f fakeProvider) ID() string { return f.id }

func (f fakeProvider) Detect(doc *pdftext.Document) (DocKind, bool) {
	for _, p := range doc.Pages {
		for _, r := range p.Runs {
			if r.Text == f.match {
				return f.kind, true
			}
		}
	}
	return "", false
}

func (f fakeProvider) Parse(*pdftext.Document, DocKind) (*Statement, error) { return nil, nil }

func docWith(text string) *pdftext.Document {
	return &pdftext.Document{Pages: []pdftext.Page{{Runs: []pdftext.Run{{Text: text}}}}}
}

func TestRegistryDetect(t *testing.T) {
	reg := NewRegistry(fakeProvider{"a", CardStatement, "A"}, fakeProvider{"b", BankStatement, "B"})
	p, kind, err := reg.Detect(docWith("B"))
	if err != nil || p.ID() != "b" || kind != BankStatement {
		t.Errorf("Detect(B) = %v, %v, %v", p, kind, err)
	}
	if _, _, err := reg.Detect(docWith("C")); !errors.Is(err, ErrUnknownDocument) {
		t.Errorf("Detect(C) err = %v, want ErrUnknownDocument", err)
	}
}
