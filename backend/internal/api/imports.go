package api

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"jgfinance/internal/importer"
)

const (
	maxFileSize    = 20 << 20  // per PDF (spec §12)
	maxRequestSize = 500 << 20 // whole batch
)

type fileResultJSON struct {
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	AccountName    string   `json:"account_name,omitempty"`
	Kind           string   `json:"kind,omitempty"`
	ReferenceMonth Month    `json:"reference_month"`
	NewCount       int      `json:"new_count"`
	ExistingCount  int      `json:"existing_count"`
	TotalCents     int64    `json:"total_cents"`
	Warnings       []string `json:"warnings"`
	Error          string   `json:"error,omitempty"`
}

type importResponse struct {
	Results        []fileResultJSON `json:"results"`
	Categorization struct {
		Auto      int `json:"auto"`
		Questions int `json:"questions"`
	} `json:"categorization"`
	Warnings []string `json:"warnings"`
}

// postImports receives multipart "files" (or "files[]") PDFs. Files that are not PDFs or are
// larger than 20 MB are reported as failed without stopping the batch.
func (s *Server) postImports(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "envie os PDFs como multipart/form-data no campo \"files\"")
		return
	}
	var files []importer.File
	var rejected []fileResultJSON
	order := []int{} // index into files (>= 0) or rejected (encoded as -1-i)
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "upload inválido: "+err.Error())
			return
		}
		if name := part.FormName(); name != "files" && name != "files[]" {
			part.Close()
			continue
		}
		fileName := filepath.Base(part.FileName())
		data, err := io.ReadAll(io.LimitReader(part, maxFileSize+1))
		part.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, "upload inválido: "+err.Error())
			return
		}
		switch {
		case len(data) > maxFileSize:
			rejected = append(rejected, rejectedFile(fileName, "arquivo maior que 20 MB"))
			order = append(order, -len(rejected))
		case !isPDF(data):
			rejected = append(rejected, rejectedFile(fileName, "o arquivo não é um PDF"))
			order = append(order, -len(rejected))
		default:
			files = append(files, importer.File{Name: fileName, Data: data})
			order = append(order, len(files)-1)
		}
	}
	if len(order) == 0 {
		writeError(w, http.StatusBadRequest, "nenhum arquivo enviado no campo \"files\"")
		return
	}
	batch, err := s.importer.Import(r.Context(), files)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	resp := importResponse{Results: []fileResultJSON{}, Warnings: batch.Warnings}
	if resp.Warnings == nil {
		resp.Warnings = []string{}
	}
	resp.Categorization.Auto = batch.Categorization.Auto
	resp.Categorization.Questions = batch.Categorization.Questions
	for _, i := range order {
		if i < 0 {
			resp.Results = append(resp.Results, rejected[-i-1])
			continue
		}
		fr := batch.Results[i]
		j := fileResultJSON{
			Name: fr.Name, Status: fr.Status, AccountName: fr.AccountName, Kind: string(fr.Kind),
			ReferenceMonth: Month(fr.ReferenceMonth), NewCount: fr.NewCount, ExistingCount: fr.ExistingCount,
			TotalCents: fr.TotalCents, Warnings: fr.Warnings, Error: fr.Error,
		}
		if j.Warnings == nil {
			j.Warnings = []string{}
		}
		resp.Results = append(resp.Results, j)
	}
	writeJSON(w, http.StatusOK, resp)
}

func rejectedFile(name, msg string) fileResultJSON {
	return fileResultJSON{Name: name, Status: importer.StatusFailed, Warnings: []string{}, Error: msg}
}

// isPDF checks the content itself (application/pdf), not the name or declared type.
func isPDF(data []byte) bool {
	return http.DetectContentType(data) == "application/pdf" || bytes.HasPrefix(bytes.TrimLeft(data, "\r\n\t "), []byte("%PDF-"))
}
