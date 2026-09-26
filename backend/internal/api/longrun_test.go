package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func (a *testAPI) uploadRequest() *http.Request {
	a.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", "fatura.pdf")
	fw.Write(fixturePDF(a.t))
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/imports", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// A client that gives up mid-import must not leave the batch half categorized.
func TestImportAndSuggestSurviveClientDisconnect(t *testing.T) {
	a := newAPI(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, a.uploadRequest().WithContext(ctx))
	if rec.Code != 200 {
		t.Fatalf("import = %d %s", rec.Code, rec.Body.String())
	}
	var pending int
	a.s.DB.QueryRow(`SELECT COUNT(*) FROM transactions WHERE kind IN ('expense', 'refund') AND category_id IS NULL
		AND categorized_by IS NULL AND question_id IS NULL`).Scan(&pending)
	if pending != 0 {
		t.Errorf("orphan transactions after import = %d", pending)
	}

	req := httptest.NewRequest(http.MethodPost, "http://localhost/api/questions/suggest", nil).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code == http.StatusInternalServerError {
		t.Errorf("suggest = %d %s", rec.Code, rec.Body.String())
	}
}

// Import and suggest may call the LLM for longer than the server's WriteTimeout.
func TestSlowImportAndSuggestOutliveWriteTimeout(t *testing.T) {
	a := newAPI(t)
	a.llm.delay = 600 * time.Millisecond
	srv := httptest.NewUnstartedServer(a.h)
	srv.Config.WriteTimeout = 200 * time.Millisecond
	srv.Start()
	defer srv.Close()

	req := a.uploadRequest()
	up, err := http.NewRequest(http.MethodPost, srv.URL+"/api/imports", req.Body)
	if err != nil {
		t.Fatal(err)
	}
	up.Header.Set("Content-Type", req.Header.Get("Content-Type"))
	res, err := srv.Client().Do(up)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	var out map[string]any
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil || res.StatusCode != 200 {
		t.Fatalf("import = %d %v (%v)", res.StatusCode, out, err)
	}
	res.Body.Close()

	sg, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/questions/suggest", nil)
	sg.Header.Set("Content-Type", "application/json")
	res, err = srv.Client().Do(sg)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	defer res.Body.Close()
	var e map[string]any
	if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
		t.Fatalf("suggest = %d (%v)", res.StatusCode, err)
	}
}
