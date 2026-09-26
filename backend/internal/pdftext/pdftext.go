// Package pdftext opens PDFs (optionally password protected) with PDFium compiled to
// WebAssembly (no cgo) and returns the positioned text runs of each page.
package pdftext

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	pdferr "github.com/klippa-app/go-pdfium/errors"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

var (
	// ErrPasswordRequired means the PDF is encrypted and no password was provided.
	ErrPasswordRequired = errors.New("o PDF tem senha e nenhuma senha foi configurada")
	// ErrWrongPassword means none of the provided passwords opened the PDF.
	ErrWrongPassword = errors.New("nenhuma das senhas configuradas abriu o PDF")
	// ErrNotPDF means the bytes are not a readable PDF.
	ErrNotPDF = errors.New("o arquivo não é um PDF válido")
)

// Run is a piece of text and its bounding box in points. The origin is the top-left
// corner of the page and Y grows downwards (PDFium itself reports Y growing upwards).
type Run struct {
	Text           string
	X0, Y0, X1, Y1 float64
}

// Page holds the runs of one page, in PDFium's order (not reading order).
type Page struct {
	Index         int
	Width, Height float64
	Runs          []Run
}

// Document is the extracted text of a whole PDF.
type Document struct {
	Pages []Page
}

var (
	mu       sync.Mutex // a single PDFium instance, used by one caller at a time
	initOnce sync.Once
	instance pdfium.Pdfium
	initErr  error
)

// engine lazily starts the WebAssembly runtime (~1.4s, once per process).
func engine() (pdfium.Pdfium, error) {
	initOnce.Do(func() {
		pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
		if err != nil {
			initErr = fmt.Errorf("pdftext: iniciar pdfium: %w", err)
			return
		}
		instance, initErr = pool.GetInstance(30 * time.Second)
	})
	return instance, initErr
}

// Open extracts the text runs of every page. It first tries without a password and, if the
// file is encrypted, tries each password in order.
func Open(data []byte, passwords []string) (*Document, error) {
	mu.Lock()
	defer mu.Unlock()
	inst, err := engine()
	if err != nil {
		return nil, err
	}

	doc, err := inst.OpenDocument(&requests.OpenDocument{File: &data})
	if errors.Is(err, pdferr.ErrPassword) {
		if len(passwords) == 0 {
			return nil, ErrPasswordRequired
		}
		for _, pw := range passwords {
			doc, err = inst.OpenDocument(&requests.OpenDocument{File: &data, Password: &pw})
			if !errors.Is(err, pdferr.ErrPassword) {
				break
			}
		}
		if errors.Is(err, pdferr.ErrPassword) {
			return nil, ErrWrongPassword
		}
	}
	if errors.Is(err, pdferr.ErrFormat) || errors.Is(err, pdferr.ErrFile) {
		return nil, ErrNotPDF
	}
	if err != nil {
		return nil, fmt.Errorf("pdftext: abrir: %w", err)
	}
	defer func() {
		_, _ = inst.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	}()

	count, err := inst.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, fmt.Errorf("pdftext: contar páginas: %w", err)
	}
	out := &Document{Pages: make([]Page, 0, count.PageCount)}
	for i := 0; i < count.PageCount; i++ {
		ref := requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i}}
		size, err := inst.GetPageSize(&requests.GetPageSize{Page: ref})
		if err != nil {
			return nil, fmt.Errorf("pdftext: tamanho da página %d: %w", i+1, err)
		}
		text, err := inst.GetPageTextStructured(&requests.GetPageTextStructured{
			Page: ref,
			Mode: requests.GetPageTextStructuredModeRects,
		})
		if err != nil {
			return nil, fmt.Errorf("pdftext: texto da página %d: %w", i+1, err)
		}
		page := Page{Index: i, Width: size.Width, Height: size.Height}
		for _, r := range text.Rects {
			t := strings.TrimSpace(r.Text)
			if t == "" {
				continue
			}
			p := r.PointPosition
			page.Runs = append(page.Runs, Run{
				Text: t,
				X0:   p.Left,
				Y0:   size.Height - p.Top,
				X1:   p.Right,
				Y1:   size.Height - p.Bottom,
			})
		}
		out.Pages = append(out.Pages, page)
	}
	return out, nil
}
