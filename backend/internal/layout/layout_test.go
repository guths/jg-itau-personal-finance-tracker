package layout

import (
	"reflect"
	"testing"

	"jgfinance/internal/pdftext"
)

func run(text string, x0, y0, x1, y1 float64) pdftext.Run {
	return pdftext.Run{Text: text, X0: x0, Y0: y0, X1: x1, Y1: y1}
}

func texts(ls []Line) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Text
	}
	return out
}

func TestLinesJoinsAccentRunsWithoutSpace(t *testing.T) {
	// PDFium returns accented glyphs as separate runs; the cedilla box also sits lower.
	p := pdftext.Page{Width: 595, Height: 842, Runs: []pdftext.Run{
		run("Lan", 133.6, 96.0, 147.6, 102.0),
		run("ç", 148.4, 97.5, 152.1, 104.5),
		run("amentos: compras", 152.8, 96.0, 230.0, 102.0),
	}}
	got := texts(Lines(p, nil))
	want := []string{"Lançamentos: compras"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLinesSeparatesRunsWithRealGapsAndCollapsesSpaces(t *testing.T) {
	p := pdftext.Page{Width: 595, Height: 842, Runs: []pdftext.Run{
		run("12,50", 300, 114, 322, 121),
		run("28/08", 40, 113, 62, 120),
		run("PADARIA   EXEMPLO", 70, 113, 150, 120),
	}}
	got := texts(Lines(p, nil))
	want := []string{"28/08 PADARIA EXEMPLO 12,50"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLinesReadingOrderWithColumns(t *testing.T) {
	p := pdftext.Page{Width: 595, Height: 842, Runs: []pdftext.Run{
		run("direita 2", 350, 108, 400, 114),
		run("esquerda 2", 40, 118, 90, 124),
		run("direita 1", 350, 88, 400, 94),
		run("esquerda 1", 40, 98, 90, 104),
	}}
	ls := Lines(p, []float64{337})
	if got, want := texts(ls), []string{"esquerda 1", "esquerda 2", "direita 1", "direita 2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	cols := []int{ls[0].Column, ls[1].Column, ls[2].Column, ls[3].Column}
	if want := []int{0, 0, 1, 1}; !reflect.DeepEqual(cols, want) {
		t.Errorf("columns = %v, want %v", cols, want)
	}
}

func TestLinesRunOnSplitGoesToRightColumn(t *testing.T) {
	p := pdftext.Page{Width: 595, Height: 842, Runs: []pdftext.Run{
		run("a", 336.9, 100, 340, 106),
		run("b", 337, 120, 345, 126),
	}}
	ls := Lines(p, []float64{337})
	if len(ls) != 2 || ls[0].Text != "a" || ls[0].Column != 0 || ls[1].Text != "b" || ls[1].Column != 1 {
		t.Errorf("got %+v", ls)
	}
}

func TestPageText(t *testing.T) {
	p := pdftext.Page{Width: 595, Height: 842, Runs: []pdftext.Run{
		run("segunda", 40, 120, 90, 126),
		run("primeira", 40, 100, 90, 106),
	}}
	if got := PageText(p); got != "primeira\nsegunda" {
		t.Errorf("PageText = %q", got)
	}
}
