package itau

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jgfinance/internal/config"
	"jgfinance/internal/money"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
)

// TestSamples parses every real PDF under SAMPLES_DIR (gitignored). Parse includes the
// reconciliation, so every real document must match its printed totals to the cent.
func TestSamples(t *testing.T) {
	cfg := config.Load("../../../../.env")
	dir := cfg.SamplesDir
	if dir == "" {
		t.Skip("SAMPLES_DIR não definido")
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".pdf") {
			files = append(files, path)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) || len(files) == 0 {
		t.Skipf("nenhum PDF em %s", dir)
	}
	if err != nil {
		t.Fatal(err)
	}
	reg := provider.NewRegistry(New())
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := pdftext.Open(data, cfg.PDFPasswords)
			if errors.Is(err, pdftext.ErrPasswordRequired) {
				t.Skip("PDF com senha: defina ITAU_PDF_PASSWORD no ambiente ou no .env")
			}
			if err != nil {
				t.Fatal(err)
			}
			p, kind, err := reg.Detect(doc)
			if err != nil {
				t.Fatal(err)
			}
			st, err := p.Parse(doc, kind)
			if err != nil {
				t.Fatalf("parse/conferência: %v", err)
			}
			if len(st.Transactions) == 0 {
				t.Fatal("nenhuma transação extraída")
			}
			for _, w := range st.Warnings {
				t.Logf("aviso: %s", w)
			}
			t.Logf("%s (%s): %d transações, total %s", st.Account.DefaultName, kind, len(st.Transactions), money.FormatBRL(st.TotalCents))
		})
	}
}
