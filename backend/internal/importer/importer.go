// Package importer turns uploaded PDFs into stored statements (spec §8): it deduplicates
// files and statements, parses them with the registered providers, stores everything in one
// MySQL transaction per file and finally runs categorization over the new transactions.
package importer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"jgfinance/internal/categorize"
	"jgfinance/internal/provider"
	"jgfinance/internal/store"
)

// File is one uploaded PDF.
type File struct {
	Name string
	Data []byte
}

// Status of one file in a batch.
const (
	StatusImported  = "imported"
	StatusDuplicate = "duplicate"
	StatusFailed    = "failed"
)

// FileResult reports what happened to one file.
type FileResult struct {
	Name           string
	Status         string
	AccountName    string
	Kind           provider.DocKind
	ReferenceMonth time.Time // first day of the month; zero when unknown
	NewCount       int
	ExistingCount  int
	TotalCents     int64 // card: printed invoice total; bank: sum of expenses and refunds
	Warnings       []string
	Error          string
}

// BatchResult reports a whole upload.
type BatchResult struct {
	Results        []FileResult
	Categorization categorize.Result
	Warnings       []string // batch-level problems that did not undo the imports
}

// Categorizer runs the categorization pipeline over newly imported transactions.
type Categorizer interface {
	Run(ctx context.Context, txIDs []int64) (categorize.Result, error)
}

// Importer imports batches of PDFs. Batches are processed one at a time.
type Importer struct {
	store       *store.Store
	registry    *provider.Registry
	passwords   []string
	dataDir     string
	categorizer Categorizer // may be nil
	mu          sync.Mutex
}

// New returns an Importer that stores PDFs under dataDir/uploads.
func New(st *store.Store, registry *provider.Registry, passwords []string, dataDir string, categorizer Categorizer) *Importer {
	return &Importer{store: st, registry: registry, passwords: passwords, dataDir: dataDir, categorizer: categorizer}
}

// Import processes files in order and then categorizes every new transaction once. Per-file
// problems are reported in the results; the error is only returned when ctx is cancelled.
func (im *Importer) Import(ctx context.Context, files []File) (BatchResult, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	var out BatchResult
	var newIDs []int64
	for _, f := range files {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		res, ids := im.importFile(ctx, f)
		out.Results = append(out.Results, res)
		newIDs = append(newIDs, ids...)
	}
	if im.categorizer != nil && len(newIDs) > 0 {
		r, err := im.categorizer.Run(ctx, newIDs)
		out.Categorization = r
		if err != nil {
			slog.Error("categorização falhou", "err", err)
			out.Warnings = append(out.Warnings,
				"a importação foi concluída, mas a categorização falhou; use \"sugerir de novo\" em Perguntas")
		}
	}
	return out, nil
}
