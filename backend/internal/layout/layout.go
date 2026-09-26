// Package layout rebuilds reading-order text lines from the positioned runs of a page.
package layout

import (
	"math"
	"sort"
	"strings"

	"jgfinance/internal/pdftext"
)

const (
	// lineTolerance is the max distance (pt) between a run's vertical center and the mean
	// center of the current line for the run to join it. Accented glyphs such as "ç" come
	// as separate runs whose boxes sit ~1.2pt lower.
	lineTolerance = 2.5
	// wordGap is the horizontal gap (pt) above which two runs are separated by a space.
	// PDFium splits accented letters into runs separated by less than 1pt.
	wordGap = 1.0
)

// Line is one reconstructed text line.
type Line struct {
	Text   string
	Column int
	Y      float64 // mean vertical center, top-origin
	X0     float64
}

// Lines groups the runs of p into lines. splits are the X boundaries between columns in
// ascending order (nil means a single column); a run belongs to column i when exactly i
// boundaries are <= its X0. Output order: column 0 top to bottom, then column 1, and so on.
func Lines(p pdftext.Page, splits []float64) []Line {
	columns := make([][]pdftext.Run, len(splits)+1)
	for _, r := range p.Runs {
		c := sort.Search(len(splits), func(i int) bool { return splits[i] > r.X0 })
		columns[c] = append(columns[c], r)
	}
	var out []Line
	for c, runs := range columns {
		sort.SliceStable(runs, func(i, j int) bool { return center(runs[i]) < center(runs[j]) })
		var cur []pdftext.Run
		var sum float64
		for _, r := range runs {
			if len(cur) > 0 && math.Abs(center(r)-sum/float64(len(cur))) > lineTolerance {
				out = append(out, build(cur, c))
				cur, sum = nil, 0
			}
			cur = append(cur, r)
			sum += center(r)
		}
		if len(cur) > 0 {
			out = append(out, build(cur, c))
		}
	}
	return out
}

// PageText returns the page as single-column lines joined by "\n".
func PageText(p pdftext.Page) string {
	lines := Lines(p, nil)
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.Text
	}
	return strings.Join(texts, "\n")
}

func build(runs []pdftext.Run, column int) Line {
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].X0 < runs[j].X0 })
	var b strings.Builder
	var sum float64
	for i, r := range runs {
		if i > 0 && r.X0-runs[i-1].X1 > wordGap {
			b.WriteByte(' ')
		}
		b.WriteString(r.Text)
		sum += center(r)
	}
	return Line{
		Text:   strings.Join(strings.Fields(b.String()), " "),
		Column: column,
		Y:      sum / float64(len(runs)),
		X0:     runs[0].X0,
	}
}

func center(r pdftext.Run) float64 { return (r.Y0 + r.Y1) / 2 }
