package pdftext

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func findRun(t *testing.T, p Page, substr string) Run {
	t.Helper()
	for _, r := range p.Runs {
		if strings.Contains(r.Text, substr) {
			return r
		}
	}
	t.Fatalf("no run containing %q in %+v", substr, p.Runs)
	return Run{}
}

func TestOpenPlainPDF(t *testing.T) {
	doc, err := Open(readFixture(t, "plain.pdf"), nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(doc.Pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(doc.Pages))
	}
	p := doc.Pages[0]
	if math.Abs(p.Width-595.28) > 0.5 || math.Abs(p.Height-841.89) > 0.5 {
		t.Errorf("page size = %.2fx%.2f, want 595.28x841.89", p.Width, p.Height)
	}
	top, bottom := findRun(t, p, "Topo"), findRun(t, p, "Rodapé")
	if top.Y0 > 150 || bottom.Y0 < 700 {
		t.Errorf("Y must grow downwards from the top: top=%+v bottom=%+v", top, bottom)
	}
	if top.Y0 >= top.Y1 || top.X0 >= top.X1 {
		t.Errorf("bounding box not normalized: %+v", top)
	}
	if v := findRun(t, p, "12,50"); v.X0 < 290 {
		t.Errorf("amount run X0 = %.1f, want >= 290", v.X0)
	}
	for _, r := range p.Runs {
		if r.Text == "" || r.Text != strings.TrimSpace(r.Text) {
			t.Errorf("run text must be trimmed and non-empty: %q", r.Text)
		}
	}
}

func TestOpenEncryptedPDF(t *testing.T) {
	data := readFixture(t, "encrypted.pdf")
	if _, err := Open(data, nil); !errors.Is(err, ErrPasswordRequired) {
		t.Errorf("no password: err = %v, want ErrPasswordRequired", err)
	}
	if _, err := Open(data, []string{"errada"}); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("wrong password: err = %v, want ErrWrongPassword", err)
	}
	doc, err := Open(data, []string{"errada", "teste123"})
	if err != nil {
		t.Fatalf("second password should open it: %v", err)
	}
	findRun(t, doc.Pages[0], "Lançamentos")
}

func TestOpenRejectsNonPDF(t *testing.T) {
	if _, err := Open([]byte("isto não é um pdf"), nil); !errors.Is(err, ErrNotPDF) {
		t.Errorf("err = %v, want ErrNotPDF", err)
	}
}
