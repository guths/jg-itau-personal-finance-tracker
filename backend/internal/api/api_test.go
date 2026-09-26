package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"jgfinance/internal/categorize"
	"jgfinance/internal/importer"
	"jgfinance/internal/insights"
	"jgfinance/internal/llm"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
	"jgfinance/internal/store"
	"jgfinance/internal/store/storetest"
)

// fakeProvider recognizes the synthetic fixture (text "Topo") and returns a fixed statement.
type fakeProvider struct{ st *provider.Statement }

func (f *fakeProvider) ID() string { return "fake" }

func (f *fakeProvider) Detect(doc *pdftext.Document) (provider.DocKind, bool) {
	for _, p := range doc.Pages {
		for _, r := range p.Runs {
			if strings.HasPrefix(r.Text, "Topo") {
				return f.st.Kind, true
			}
		}
	}
	return "", false
}

func (f *fakeProvider) Parse(*pdftext.Document, provider.DocKind) (*provider.Statement, error) {
	cp := *f.st
	return &cp, nil
}

// fakeLLM only answers free-text interpretations; delay slows every call down.
type fakeLLM struct {
	interp map[string]any
	delay  time.Duration
}

func (f *fakeLLM) JSON(_ context.Context, req llm.Request, out any) error {
	time.Sleep(f.delay)
	if req.SchemaName != "interpretacao" {
		return fmt.Errorf("fora do ar")
	}
	raw, _ := json.Marshal(f.interp)
	return json.Unmarshal(raw, out)
}

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func statement() *provider.Statement {
	return &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{Provider: "fake", Kind: provider.CreditCard, ExternalID: "8778", DefaultName: "Platinum •••• 8778"},
		ClosingDate: d(2026, 9, 24), DueDate: d(2026, 10, 3), TotalCents: 5250,
		Transactions: []provider.Transaction{
			{Seq: 1, Date: d(2026, 9, 1), Description: "PADARIA BOA", Key: "PADARIA BOA", AmountCents: 1250, Kind: provider.Expense},
			{Seq: 2, Date: d(2026, 9, 2), Description: "PADARIA BOA", Key: "PADARIA BOA", AmountCents: 1000, Kind: provider.Expense},
			{Seq: 3, Date: d(2026, 9, 3), Description: "LOJA X 02/03", Key: "LOJA X", AmountCents: 3000, Kind: provider.Expense, InstallmentN: 2, InstallmentTotal: 3},
		},
		Upcoming: []provider.UpcomingInstallment{{Description: "LOJA X 03/03", Key: "LOJA X", AmountCents: 3000, N: 3, Total: 3}},
	}
}

type testAPI struct {
	t   *testing.T
	s   *store.Store
	h   http.Handler
	llm *fakeLLM
}

func newAPI(t *testing.T) *testAPI {
	t.Helper()
	s := storetest.New(t)
	fl := &fakeLLM{}
	cat := categorize.New(s, fl)
	im := importer.New(s, provider.NewRegistry(&fakeProvider{st: statement()}), nil, t.TempDir(), cat)
	srv := &Server{store: s, importer: im, categorize: cat, insights: insights.New(s),
		now: func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) }}
	return &testAPI{t: t, s: s, h: srv.routes(), llm: fl}
}

// do sends a JSON request and decodes the JSON response into out (when not nil).
func (a *testAPI) do(method, path string, body any, out any) int {
	a.t.Helper()
	var rd io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Host = "localhost:8080"
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			a.t.Fatalf("%s %s: %v\n%s", method, path, err, rec.Body.String())
		}
	}
	if rec.Code >= 400 {
		var e map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e["error"] == "" {
			a.t.Errorf("%s %s: error body must be {\"error\": ...}: %s", method, path, rec.Body.String())
		}
	}
	return rec.Code
}

func (a *testAPI) upload(files map[string][]byte) (int, map[string]any) {
	a.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, name := range []string{"fatura.pdf", "notas.txt"} {
		if data, ok := files[name]; ok {
			fw, _ := mw.CreateFormFile("files", name)
			fw.Write(data)
		}
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/imports", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func fixturePDF(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "pdftext", "testdata", "plain.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (a *testAPI) importFixture() {
	a.t.Helper()
	if code, out := a.upload(map[string][]byte{"fatura.pdf": fixturePDF(a.t)}); code != 200 {
		a.t.Fatalf("upload = %d %v", code, out)
	}
}

func (a *testAPI) subID(cat, sub string) int64 {
	a.t.Helper()
	var id int64
	if err := a.s.DB.QueryRow(`SELECT s.id FROM categories s JOIN categories c ON c.id = s.parent_id WHERE c.name = ? AND s.name = ?`, cat, sub).Scan(&id); err != nil {
		a.t.Fatal(err)
	}
	return id
}

func TestUpload(t *testing.T) {
	a := newAPI(t)
	code, out := a.upload(map[string][]byte{"fatura.pdf": fixturePDF(t), "notas.txt": []byte("não sou pdf")})
	if code != 200 {
		t.Fatalf("status = %d %v", code, out)
	}
	results := out["results"].([]any)
	first, second := results[0].(map[string]any), results[1].(map[string]any)
	if first["status"] != "imported" || first["reference_month"] != "2026-09" || first["new_count"] != 3.0 ||
		first["account_name"] != "Platinum •••• 8778" || first["total_cents"] != 5250.0 || first["kind"] != "card_statement" {
		t.Errorf("first = %v", first)
	}
	if second["status"] != "failed" || second["error"] != "o arquivo não é um PDF" {
		t.Errorf("second = %v", second)
	}
	if cat := out["categorization"].(map[string]any); cat["questions"] != 2.0 || cat["auto"] != 0.0 {
		t.Errorf("categorization = %v (LLM down → one question per key)", cat)
	}
	code, out = a.upload(map[string][]byte{"fatura.pdf": fixturePDF(t)})
	if r := out["results"].([]any)[0].(map[string]any); code != 200 || r["status"] != "duplicate" {
		t.Errorf("second upload = %v", out)
	}
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/imports", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("non-multipart = %d", rec.Code)
	}

	var docs []map[string]any
	if a.do("GET", "/api/documents", nil, &docs) != 200 || len(docs) != 1 || docs[0]["closing_date"] != "2026-09-24" ||
		docs[0]["transaction_count"] != 3.0 {
		t.Errorf("documents = %v", docs)
	}
	var accounts []map[string]any
	a.do("GET", "/api/accounts", nil, &accounts)
	id := int64(accounts[0]["id"].(float64))
	if code := a.do("PATCH", fmt.Sprintf("/api/accounts/%d", id), map[string]string{"name": "Meu Platinum"}, nil); code != 200 {
		t.Errorf("rename = %d", code)
	}
	if code := a.do("PATCH", "/api/accounts/999999", map[string]string{"name": "x"}, nil); code != 404 {
		t.Errorf("rename missing = %d", code)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/accounts/%d", id), map[string]string{"name": " "}, nil); code != 422 {
		t.Errorf("empty name = %d", code)
	}
}

type questionResp struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	Status       string `json:"status"`
	TotalCents   int64  `json:"total_cents"`
	Count        int    `json:"count"`
	Transactions []struct {
		ID   int64  `json:"id"`
		Date string `json:"date"`
	} `json:"transactions"`
	Suggestions []any `json:"suggestions"`
}

func TestQuestionsFlow(t *testing.T) {
	a := newAPI(t)
	a.importFixture()
	var qs []questionResp
	if code := a.do("GET", "/api/questions?status=pending", nil, &qs); code != 200 || len(qs) != 2 {
		t.Fatalf("questions = %d %+v", code, qs)
	}
	bakery := qs[1]
	if bakery.Key != "PADARIA BOA" || bakery.Count != 2 || bakery.TotalCents != 2250 || bakery.Transactions[0].Date != "2026-09-01" ||
		bakery.Suggestions == nil {
		t.Errorf("question = %+v", bakery)
	}
	cafe := a.subID("Alimentação", "Padaria e café")
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/answer", bakery.ID), map[string]any{"category_id": cafe, "remember": true, "alias": "Padaria Boa"}, nil); code != 200 {
		t.Errorf("answer = %d", code)
	}
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/answer", bakery.ID), map[string]any{"category_id": cafe, "remember": true}, nil); code != 409 {
		t.Errorf("answer twice = %d", code)
	}
	loja := qs[0]
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/answer", loja.ID), map[string]any{"category_id": 1, "remember": true}, nil); code != 422 {
		t.Errorf("top-level category = %d", code)
	}
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/answer", loja.ID), map[string]any{"bogus": 1}, nil); code != 400 {
		t.Errorf("unknown field = %d", code)
	}
	if code := a.do("POST", "/api/questions/999999/answer", map[string]any{"category_id": cafe}, nil); code != 404 {
		t.Errorf("missing = %d", code)
	}
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/skip", loja.ID), nil, nil); code != 200 {
		t.Errorf("skip = %d", code)
	}
	if code := a.do("POST", "/api/questions/999999/skip", nil, nil); code != 404 {
		t.Errorf("skip missing = %d", code)
	}
	if code := a.do("POST", "/api/questions/abc/skip", nil, nil); code != 400 {
		t.Errorf("bad id = %d", code)
	}
	a.do("GET", "/api/questions", nil, &qs)
	if len(qs) != 1 || qs[0].Status != "skipped" {
		t.Errorf("open questions = %+v", qs)
	}
	if code := a.do("GET", "/api/questions?status=answered", nil, nil); code != 400 {
		t.Errorf("bad status = %d", code)
	}

	online := a.subID("Compras", "Online")
	a.llm.interp = map[string]any{"kind": "expense", "subcategory_id": online, "alias": "Loja X"}
	var interp map[string]any
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/interpret", loja.ID), map[string]string{"text": "comprei online"}, &interp); code != 200 ||
		interp["category_path"] != "Compras › Online" || interp["alias"] != "Loja X" {
		t.Errorf("interpret = %d %v", code, interp)
	}
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/interpret", loja.ID), map[string]string{"text": " "}, nil); code != 422 {
		t.Errorf("empty text = %d", code)
	}
	if code := a.do("POST", "/api/questions/suggest", nil, nil); code != 502 {
		t.Errorf("suggest with the LLM down = %d", code)
	}
	if code := a.do("POST", fmt.Sprintf("/api/questions/%d/answer", loja.ID), map[string]any{"kind": "ignored", "remember": false}, nil); code != 200 {
		t.Errorf("answer skipped question = %d", code)
	}

	var rules []map[string]any
	a.do("GET", "/api/rules?source=user", nil, &rules)
	if len(rules) != 5 || rules[0]["key"] == nil {
		t.Errorf("rules = %v", rules)
	}
}

type txResp struct {
	ID            int64   `json:"id"`
	Key           string  `json:"key"`
	Kind          string  `json:"kind"`
	CategoryID    *int64  `json:"category_id"`
	CategoryPath  *string `json:"category_path"`
	CategorizedBy *string `json:"categorized_by"`
	Date          string  `json:"date"`
}

func TestTransactionsPatchScopes(t *testing.T) {
	a := newAPI(t)
	a.importFixture()
	var txs []txResp
	if code := a.do("GET", "/api/transactions?month=2026-09&uncategorized=true", nil, &txs); code != 200 || len(txs) != 3 {
		t.Fatalf("transactions = %d %+v", code, txs)
	}
	var bakery []txResp
	a.do("GET", "/api/transactions?q=padaria", nil, &bakery)
	if len(bakery) != 2 {
		t.Fatalf("search = %+v", bakery)
	}
	cafe, rest := a.subID("Alimentação", "Padaria e café"), a.subID("Alimentação", "Restaurante")
	if code := a.do("PATCH", fmt.Sprintf("/api/transactions/%d", bakery[0].ID), map[string]any{"category_id": cafe, "scope": "one"}, nil); code != 200 {
		t.Errorf("patch one = %d", code)
	}
	var one []txResp
	a.do("GET", "/api/transactions?categorized_by=user", nil, &one)
	if len(one) != 1 || one[0].ID != bakery[0].ID || *one[0].CategoryPath != "Alimentação › Padaria e café" {
		t.Errorf("after scope one = %+v", one)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/transactions/%d", bakery[1].ID), map[string]any{"category_id": rest, "scope": "key"}, nil); code != 200 {
		t.Errorf("patch key = %d", code)
	}
	var byCat []txResp
	a.do("GET", fmt.Sprintf("/api/transactions?category_id=%d", rest), nil, &byCat)
	if len(byCat) != 1 || byCat[0].ID != bakery[1].ID {
		t.Errorf("category filter = %+v (the user's scope-one choice must survive)", byCat)
	}
	var byParent []txResp
	a.do("GET", "/api/transactions?category_id=2", nil, &byParent) // Alimentação includes its subcategories
	if len(byParent) != 2 {
		t.Errorf("parent filter = %+v", byParent)
	}
	cases := []struct {
		path string
		body map[string]any
		want int
	}{
		{fmt.Sprintf("/api/transactions/%d", bakery[0].ID), map[string]any{"category_id": cafe, "scope": "all"}, 422},
		{fmt.Sprintf("/api/transactions/%d", bakery[0].ID), map[string]any{"scope": "one"}, 400},
		{"/api/transactions/999999", map[string]any{"category_id": cafe, "scope": "one"}, 404},
		{fmt.Sprintf("/api/transactions/%d", bakery[0].ID), map[string]any{"category_id": 999999, "scope": "one"}, 422},
	}
	for _, c := range cases {
		if code := a.do("PATCH", c.path, c.body, nil); code != c.want {
			t.Errorf("PATCH %s %v = %d, want %d", c.path, c.body, code, c.want)
		}
	}
	for _, q := range []string{"month=2026-13", "account_id=x", "kind=banana", "uncategorized=talvez", "categorized_by=robo"} {
		if code := a.do("GET", "/api/transactions?"+q, nil, nil); code != 400 {
			t.Errorf("GET ?%s = %d", q, code)
		}
	}

	var rules []map[string]any
	a.do("GET", "/api/rules", nil, &rules)
	ruleID := int64(rules[0]["id"].(float64))
	if rules[0]["key"] != "PADARIA BOA" {
		t.Fatalf("rules = %v", rules)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/rules/%d", ruleID), map[string]any{"category_id": cafe, "recurring": "yes"}, nil); code != 200 {
		t.Errorf("patch rule = %d", code)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/rules/%d", ruleID), map[string]any{"mode": "sempre"}, nil); code != 422 {
		t.Errorf("bad rule = %d", code)
	}
	req := httptest.NewRequest("DELETE", fmt.Sprintf("http://localhost:8080/api/rules/%d", ruleID), nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Errorf("delete = %d", rec.Code)
	}
	if code := a.do("DELETE", fmt.Sprintf("/api/rules/%d", ruleID), nil, nil); code != 404 {
		t.Errorf("delete twice = %d", code)
	}
}

func TestDashboardAndMonths(t *testing.T) {
	a := newAPI(t)
	a.importFixture()
	var months []string
	if code := a.do("GET", "/api/months", nil, &months); code != 200 || len(months) != 1 || months[0] != "2026-09" {
		t.Errorf("months = %d %v", code, months)
	}
	var dash map[string]any
	if code := a.do("GET", "/api/dashboard?month=2026-09", nil, &dash); code != 200 {
		t.Fatalf("dashboard = %d", code)
	}
	for _, k := range []string{"summary", "by_category", "recurring", "upcoming", "evolution", "top_merchants"} {
		if _, ok := dash[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
	sum := dash["summary"].(map[string]any)
	if sum["total"] != 5250.0 || sum["prev3_avg"] != nil || sum["open_bills"] != 5250.0 || sum["future_installments"] != 3000.0 ||
		sum["uncategorized"].(map[string]any)["count"] != 3.0 || sum["month"] != "2026-09" {
		t.Errorf("summary = %v", sum)
	}
	up := dash["upcoming"].(map[string]any)
	if bills := up["bills"].([]any); len(bills) != 1 || bills[0].(map[string]any)["due_date"] != "2026-10-03" {
		t.Errorf("bills = %v", up["bills"])
	}
	if ev := dash["evolution"].([]any); len(ev) != 12 {
		t.Errorf("evolution = %v", ev)
	}
	var def map[string]any
	if a.do("GET", "/api/dashboard", nil, &def); def["summary"].(map[string]any)["month"] != "2026-09" {
		t.Errorf("default month = %v", def["summary"])
	}
	if code := a.do("GET", "/api/dashboard?month=09-2026", nil, nil); code != 400 {
		t.Errorf("bad month = %d", code)
	}
	if code := a.do("GET", "/api/nada", nil, nil); code != 404 {
		t.Errorf("unknown route = %d", code)
	}
}

func TestCategories(t *testing.T) {
	a := newAPI(t)
	var tree []struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Color         string `json:"color"`
		Subcategories []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"subcategories"`
	}
	if code := a.do("GET", "/api/categories", nil, &tree); code != 200 || len(tree) != 13 || tree[0].Name != "Moradia" ||
		len(tree[0].Subcategories) != 7 || tree[0].Color == "" {
		t.Fatalf("tree = %d %+v", code, tree)
	}
	var created map[string]any
	if code := a.do("POST", "/api/categories", map[string]any{"name": "Pets", "color": "#AA5500"}, &created); code != 201 {
		t.Fatalf("create = %d", code)
	}
	pets := int64(created["id"].(float64))
	if code := a.do("POST", "/api/categories", map[string]any{"name": "Veterinário", "parent_id": pets}, nil); code != 201 {
		t.Errorf("create sub = %d", code)
	}
	if code := a.do("POST", "/api/categories", map[string]any{"name": "X", "parent_id": tree[0].Subcategories[0].ID}, nil); code != 422 {
		t.Errorf("sub of sub = %d", code)
	}
	if code := a.do("POST", "/api/categories", map[string]any{"name": "Sem cor"}, nil); code != 422 {
		t.Errorf("no color = %d", code)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/categories/%d", pets), map[string]any{"name": "Pet", "archived": true, "position": 20}, nil); code != 200 {
		t.Errorf("patch = %d", code)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/categories/%d", tree[0].Subcategories[0].ID), map[string]any{"color": "#000000"}, nil); code != 422 {
		t.Errorf("sub color = %d", code)
	}
	if code := a.do("PATCH", "/api/categories/999999", map[string]any{"name": "x"}, nil); code != 404 {
		t.Errorf("missing = %d", code)
	}
	a.do("GET", "/api/categories", nil, &tree)
	last := tree[len(tree)-1]
	if len(tree) != 14 || last.Name != "Pet" || len(last.Subcategories) != 1 {
		t.Errorf("tree after = %+v", last)
	}
}

func TestTransactionsReversedKind(t *testing.T) {
	a := newAPI(t)
	a.importFixture()
	var all []txResp
	a.do("GET", "/api/transactions?month=2026-09&uncategorized=true", nil, &all)
	if len(all) == 0 {
		t.Fatal("no transactions")
	}
	id := all[0].ID
	if _, err := a.s.DB.Exec(`UPDATE transactions SET kind = 'reversed', question_id = NULL WHERE id = ?`, id); err != nil {
		t.Fatal(err)
	}
	var rev []txResp
	if code := a.do("GET", "/api/transactions?kind=reversed", nil, &rev); code != 200 || len(rev) != 1 || rev[0].ID != id || rev[0].Kind != "reversed" {
		t.Errorf("kind=reversed = %d %+v", code, rev)
	}
	var unc []txResp
	a.do("GET", "/api/transactions?month=2026-09&uncategorized=true", nil, &unc)
	if len(unc) != len(all)-1 {
		t.Errorf("uncategorized = %d, want %d (a reversal is not spending)", len(unc), len(all)-1)
	}
	if code := a.do("PATCH", fmt.Sprintf("/api/transactions/%d", all[1].ID), map[string]any{"kind": "reversed", "scope": "one"}, nil); code != 422 {
		t.Errorf("setting reversed by hand = %d, want 422", code)
	}
}
