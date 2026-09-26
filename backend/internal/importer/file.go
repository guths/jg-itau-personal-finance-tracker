package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"

	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
)

// User-facing messages of spec §14.
const (
	msgPasswordRequired = "O PDF tem senha: configure ITAU_PDF_PASSWORD no .env"
	msgWrongPassword    = "Nenhuma das senhas configuradas abriu este PDF"
	msgUnknownDocument  = "Nenhum banco reconheceu este arquivo"
	msgDuplicateFile    = "arquivo já importado"
	msgDuplicateInvoice = "fatura já importada"
	msgUnexpected       = "erro inesperado ao importar este arquivo; detalhes no log da API"
)

// errDuplicate aborts the file transaction when the document already exists.
type errDuplicate struct{ msg string }

func (e errDuplicate) Error() string { return e.msg }

// importFile runs the per-file pipeline of spec §8.1 and returns the ids of new transactions.
func (im *Importer) importFile(ctx context.Context, f File) (FileResult, []int64) {
	res := FileResult{Name: f.Name}
	sum := sha256.Sum256(f.Data)
	sha := hex.EncodeToString(sum[:])

	exists, err := im.documentExists(ctx, sha)
	if err != nil {
		return im.unexpected(res, f, err), nil
	}
	if exists {
		res.Status, res.Error = StatusDuplicate, msgDuplicateFile
		return res, nil
	}

	doc, err := pdftext.Open(f.Data, im.passwords)
	if err != nil {
		res.Status, res.Error = StatusFailed, openMessage(err)
		return res, nil
	}
	p, kind, err := im.registry.Detect(doc)
	if err != nil {
		res.Status, res.Error = StatusFailed, msgUnknownDocument
		return res, nil
	}
	res.Kind = kind
	st, err := p.Parse(doc, kind)
	if err != nil {
		res.Status, res.Error = StatusFailed, err.Error()
		if cerr := im.saveFile(filepath.Join("uploads", "failed", sha+".pdf"), f.Data); cerr != nil {
			slog.Error("guardar PDF que falhou", "file", f.Name, "err", cerr)
		}
		return res, nil
	}

	saved, ids, err := im.persist(ctx, f, sha, p.ID(), st)
	var dup errDuplicate
	switch {
	case errors.As(err, &dup):
		res.Status, res.Error = StatusDuplicate, dup.msg
		return res, nil
	case err != nil:
		return im.unexpected(res, f, err), nil
	}
	res.Status = StatusImported
	res.AccountName = saved.accountName
	res.ReferenceMonth = saved.referenceMonth
	res.NewCount = len(ids)
	res.ExistingCount = len(st.Transactions) - len(ids)
	res.TotalCents = statementTotal(st)
	res.Warnings = st.Warnings
	return res, ids
}

func (im *Importer) unexpected(res FileResult, f File, err error) FileResult {
	slog.Error("importar arquivo", "file", f.Name, "err", err)
	res.Status, res.Error = StatusFailed, msgUnexpected
	return res
}

// openMessage translates pdftext errors into the messages of spec §14.
func openMessage(err error) string {
	switch {
	case errors.Is(err, pdftext.ErrPasswordRequired):
		return msgPasswordRequired
	case errors.Is(err, pdftext.ErrWrongPassword):
		return msgWrongPassword
	case errors.Is(err, pdftext.ErrNotPDF):
		return pdftext.ErrNotPDF.Error()
	}
	slog.Error("abrir PDF", "err", err)
	return msgUnexpected
}

// saveFile writes data to dataDir/rel, creating directories as needed.
func (im *Importer) saveFile(rel string, data []byte) error {
	path := filepath.Join(im.dataDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// statementTotal is the amount shown for an imported file.
func statementTotal(st *provider.Statement) int64 {
	if st.Kind == provider.CardStatement {
		return st.TotalCents
	}
	var total int64
	for _, tx := range st.Transactions {
		if tx.Kind == provider.Expense || tx.Kind == provider.Refund {
			total += tx.AmountCents
		}
	}
	return total
}
