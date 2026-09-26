# Plano 1 — Extração de PDFs (fundação do backend + Itaú)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ler faturas e extratos do Itaú em PDF (com senha), extrair transações normalizadas e **conferir centavo a centavo** contra os totais impressos, com uma ferramenta de linha de comando (`cmd/inspect`) para validar PDFs reais antes de existir banco de dados ou interface.

**Architecture:** `pdftext` abre o PDF via go-pdfium (PDFium em WebAssembly, sem cgo) e devolve trechos de texto com coordenadas; `layout` reconstrói linhas em ordem de leitura (com colunas); `provider` define o modelo comum (Statement/Transaction) e o registro de bancos; `provider/itau` implementa detecção, parse e conferência da fatura e do extrato.

**Tech Stack:** Go 1.26, github.com/klippa-app/go-pdfium v1.21.0 (webassembly), golang.org/x/text, github.com/joho/godotenv, github.com/google/go-cmp (testes).

**Spec:** `docs/superpowers/specs/2026-09-26-finance-tracker-design.md` (seções 4, 5, 6, 7 e 16). Planos 2 (banco, importação, categorização, API) e 3 (web) vêm depois deste.

## Global Constraints

- Módulo Go `jgfinance` em `backend/`, diretiva `go 1.26.0` (o toolchain é baixado automaticamente se o Go local for mais antigo).
- Dinheiro sempre em centavos `int64`; nunca `float` para valores monetários.
- Datas civis como `time.Time` à meia-noite UTC.
- Identificadores e comentários em inglês; mensagens para o usuário (erros, saída do CLI) em pt-BR.
- **Nenhum dado financeiro real em arquivos versionados.** Fixtures são inventadas. PDFs reais só em `samples/` (ignorado pelo git).
- **Nunca escrever a senha de PDF em arquivo algum.** Ela vem de `ITAU_PDF_PASSWORD` no ambiente ou no `.env` (que o usuário preenche; ignorado pelo git).
- Dependências permitidas neste plano: as listadas em Tech Stack.
- Antes de cada commit: `gofmt -w` nos arquivos alterados e `go vet ./...` limpo (rodar dentro de `backend/`).
- Mensagens de commit: `tipo(escopo): descrição em pt-BR`, terminando com a linha em branco + `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **Fatura com cartão adicional** (segundo titular, com seu próprio subtotal "Lançamentos no cartão") → precisa reconciliar somando os subtotais e manter o titular de cada compra. Teste: `TestParseFaturaTwoHolders` (Task 6).
2. **Virada de ano:** compra de dezembro numa fatura que fecha em janeiro, e parcela comprada no ano anterior → data com o ano certo. Testes: `TestCardDate` (Task 5) e o item `15/11 … 03/10` do fixture (Task 5).
3. **Dois PIX idênticos no mesmo dia** no extrato → os dois ficam, com `Seq` distintos. Teste: fixture do extrato com `PIX TRANSF MARIA S04/03 -35,00` duas vezes (Task 7).
4. **Fatura sem seção internacional e sem produtos/serviços** → regras de conferência daqueles rótulos são puladas, sem erro falso. Teste: `TestParseFaturaTwoHolders` (Task 6).
5. **Ruído de página** (rodapé "continua...", telefones, texto da oferta de parcelamento com valores em R$) nunca vira transação nem linha de detalhe. Teste: linhas de ruído dentro de `faturaBodyFixture` (Task 5).

---

### Task 1: Módulo Go e pacote `money`

**Files:**
- Create: `backend/go.mod` (via `go mod init`)
- Create: `backend/internal/money/money.go`
- Test: `backend/internal/money/money_test.go`
- Create: `Makefile`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `const money.Pattern = `-?\d{1,3}(?:\.\d{3})*,\d{2}`` (fragmento de regexp de valor impresso)
  - `func money.ParseBRL(s string) (int64, error)`
  - `func money.FormatBRL(cents int64) string` → `"R$ 1.234,56"`, negativos `"-R$ 70,00"`

- [ ] **Step 1: Criar o módulo**

```bash
mkdir -p backend && cd backend && go mod init jgfinance && go mod edit -go=1.26.0
```

- [ ] **Step 2: Escrever o teste que falha** — `backend/internal/money/money_test.go`

```go
package money

import "testing"

func TestParseBRL(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0,00", 0},
		{"12,96", 1296},
		{"1.234,56", 123456},
		{"-70,00", -7000},
		{"-19.000,00", -1900000},
		{"43.271,94", 4327194},
		{" 5,40 ", 540},
		{"-0,33", -33},
	}
	for _, c := range cases {
		got, err := ParseBRL(c.in)
		if err != nil {
			t.Errorf("ParseBRL(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseBRL(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseBRLRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "12", "12,5", "12.50", "1,234.56", "1.23,45", "R$ 12,00", "abc"} {
		if _, err := ParseBRL(in); err == nil {
			t.Errorf("ParseBRL(%q) should fail", in)
		}
	}
}

func TestFormatBRL(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "R$ 0,00"},
		{5, "R$ 0,05"},
		{1296, "R$ 12,96"},
		{123456, "R$ 1.234,56"},
		{-7000, "-R$ 70,00"},
		{100000000, "R$ 1.000.000,00"},
	}
	for _, c := range cases {
		if got := FormatBRL(c.in); got != c.want {
			t.Errorf("FormatBRL(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/money/...`
Expected: FAIL com `undefined: ParseBRL` / `undefined: FormatBRL`.

- [ ] **Step 4: Implementar** — `backend/internal/money/money.go`

```go
// Package money converts Brazilian-formatted amounts ("1.234,56") to integer cents and back.
// Amounts are always int64 cents; floats are never used for money.
package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Pattern matches a printed BRL amount such as "12,96", "1.234,56" or "-70,00".
const Pattern = `-?\d{1,3}(?:\.\d{3})*,\d{2}`

var exact = regexp.MustCompile(`^` + Pattern + `$`)

// ParseBRL converts a printed amount to cents. Surrounding spaces are ignored.
func ParseBRL(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !exact.MatchString(s) {
		return 0, fmt.Errorf("valor em reais inválido: %q", s)
	}
	return strconv.ParseInt(strings.NewReplacer(".", "", ",", "").Replace(s), 10, 64)
}

// FormatBRL renders cents as "R$ 1.234,56" (negative values as "-R$ 70,00").
func FormatBRL(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%sR$ %s,%02d", sign, b.String(), cents%100)
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/money/...`
Expected: `ok  	jgfinance/internal/money`

- [ ] **Step 6: Criar o `Makefile` na raiz**

```make
.PHONY: test

test:
	cd backend && go test ./...
```

Run: `make test` → Expected: `ok  	jgfinance/internal/money`

- [ ] **Step 7: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add Makefile backend/go.mod backend/internal/money
git commit -m "feat(money): valores em centavos a partir do formato brasileiro

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: `pdftext` — abrir PDF (com senha) e extrair trechos com coordenadas

**Files:**
- Create: `backend/internal/pdftext/testdata/gen_fixtures.py`
- Create (gerados pelo script): `backend/internal/pdftext/testdata/plain.pdf`, `backend/internal/pdftext/testdata/encrypted.pdf`
- Create: `backend/internal/pdftext/pdftext.go`
- Test: `backend/internal/pdftext/pdftext_test.go`

**Interfaces:**
- Consumes: nada.
- Produces:
  - `type pdftext.Run struct { Text string; X0, Y0, X1, Y1 float64 }` — pontos, origem no **topo** à esquerda, Y cresce para baixo; `Text` sem espaços nas pontas e nunca vazio.
  - `type pdftext.Page struct { Index int; Width, Height float64; Runs []Run }`
  - `type pdftext.Document struct { Pages []Page }`
  - `func pdftext.Open(data []byte, passwords []string) (*Document, error)`
  - `var pdftext.ErrPasswordRequired, pdftext.ErrWrongPassword, pdftext.ErrNotPDF error`

Fatos verificados no spike (go-pdfium v1.21.0): `OpenDocument` devolve `errors.ErrPassword` ("4: invalid password") tanto sem senha quanto com senha errada, e `errors.ErrFormat` ("3: incorrect format") para bytes que não são PDF. `PointPosition.Top`/`Bottom` têm Y crescendo **para cima** (Top > Bottom). O modo `rects` devolve trechos de texto; letras acentuadas podem vir em trechos separados (tratado na Task 3).

- [ ] **Step 1: Adicionar a dependência**

```bash
cd backend && go get github.com/klippa-app/go-pdfium@v1.21.0
```

- [ ] **Step 2: Criar o gerador de fixtures** — `backend/internal/pdftext/testdata/gen_fixtures.py`

```python
#!/usr/bin/env python3
"""Gera os PDFs sintéticos dos testes de pdftext (sem nenhum dado real).

Requer PyMuPDF (pip install pymupdf). Rodar dentro de testdata/:
    python3 gen_fixtures.py
"""
import fitz


def build():
    doc = fitz.open()
    page = doc.new_page(width=595.28, height=841.89)
    page.insert_text((72, 100), "Topo Lançamentos: compras", fontsize=9, fontname="helv")
    page.insert_text((72, 120), "28/08 PADARIA EXEMPLO", fontsize=9, fontname="helv")
    page.insert_text((300, 120), "12,50", fontsize=9, fontname="helv")
    page.insert_text((72, 800), "Rodapé", fontsize=9, fontname="helv")
    return doc


build().save("plain.pdf")
build().save(
    "encrypted.pdf",
    encryption=fitz.PDF_ENCRYPT_RC4_128,
    owner_pw="dono",
    user_pw="teste123",
    permissions=fitz.PDF_PERM_PRINT | fitz.PDF_PERM_COPY,
)
```

Run: `cd backend/internal/pdftext/testdata && python3 gen_fixtures.py && ls -la *.pdf`
Expected: `plain.pdf` e `encrypted.pdf` criados (alguns KB cada). A senha de teste `teste123` é fictícia e pode ficar versionada.

- [ ] **Step 3: Escrever o teste que falha** — `backend/internal/pdftext/pdftext_test.go`

```go
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
```

- [ ] **Step 4: Rodar e ver falhar**

Run: `cd backend && go test ./internal/pdftext/...`
Expected: FAIL com `undefined: Open` (e demais símbolos).

- [ ] **Step 5: Implementar** — `backend/internal/pdftext/pdftext.go`

```go
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
```

- [ ] **Step 6: Rodar e ver passar**

Run: `cd backend && go mod tidy && go test ./internal/pdftext/...`
Expected: `ok  	jgfinance/internal/pdftext` (≈2–3 s por causa da inicialização do WebAssembly).

- [ ] **Step 7: Commit** (o `.gitignore` já libera `backend/internal/pdftext/testdata/*.pdf`)

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add backend/go.mod backend/go.sum backend/internal/pdftext
git commit -m "feat(pdftext): abre PDF com senha via PDFium WASM e devolve trechos posicionados

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: `layout` — trechos → linhas em ordem de leitura

**Files:**
- Create: `backend/internal/layout/layout.go`
- Test: `backend/internal/layout/layout_test.go`

**Interfaces:**
- Consumes: `pdftext.Run`, `pdftext.Page` (Task 2).
- Produces:
  - `type layout.Line struct { Text string; Column int; Y float64; X0 float64 }`
  - `func layout.Lines(p pdftext.Page, splits []float64) []Line` — `splits` em ordem crescente; `nil` = uma coluna; trecho vai para a coluna `i` quando exatamente `i` limites são `<=` seu `X0`; saída: coluna 0 de cima para baixo, depois coluna 1…; espaços internos colapsados.
  - `func layout.PageText(p pdftext.Page) string` — linhas de coluna única unidas por `"\n"`.

Regras (validadas com os PDFs reais no spike): linhas agrupadas quando o centro vertical do trecho difere ≤ 2,5 pt da **média** dos centros da linha corrente; dentro da linha, trechos com vão horizontal ≤ 1,0 pt são colados sem espaço (acentos vêm em trechos separados: `Lan` + `ç` + `amentos`); vão maior vira um espaço.

- [ ] **Step 1: Escrever o teste que falha** — `backend/internal/layout/layout_test.go`

```go
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
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/layout/...`
Expected: FAIL com `undefined: Lines` / `undefined: Line` / `undefined: PageText`.

- [ ] **Step 3: Implementar** — `backend/internal/layout/layout.go`

```go
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
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/layout/...`
Expected: `ok  	jgfinance/internal/layout`

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add backend/internal/layout
git commit -m "feat(layout): reconstrói linhas em ordem de leitura com colunas e acentos

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `provider` — modelo comum, chave normalizada, erros e registro

**Files:**
- Create: `backend/internal/provider/provider.go`
- Create: `backend/internal/provider/errors.go`
- Create: `backend/internal/provider/key.go`
- Create: `backend/internal/provider/registry.go`
- Test: `backend/internal/provider/provider_test.go`

**Interfaces:**
- Consumes: `pdftext.Document` (Task 2), `money.FormatBRL` (Task 1).
- Produces (usados por todas as tasks seguintes e pelos planos 2 e 3):
  - `type DocKind string` com `CardStatement = "card_statement"`, `BankStatement = "bank_statement"`
  - `type TxKind string` com `Expense, Refund, BillPayment, OwnTransfer, Investment, Income, Ignored` (valores `"expense"`, `"refund"`, `"bill_payment"`, `"own_transfer"`, `"investment"`, `"income"`, `"ignored"`)
  - `type AccountKind string` com `CreditCard = "credit_card"`, `Checking = "checking"`
  - `type AccountRef struct { Provider string; Kind AccountKind; ExternalID string; DefaultName string }`
  - `type Transaction struct { Seq int; Date time.Time; Description, Key string; AmountCents int64; Kind TxKind; PersonTransfer bool; InstallmentN, InstallmentTotal int; BankCategory, City, Holder string; OriginalAmountCents int64; OriginalCurrency string }`
  - `type UpcomingInstallment struct { PurchaseDate time.Time; Description, Key string; AmountCents int64; N, Total int }`
  - `type Statement struct { Kind DocKind; Account AccountRef; PeriodStart, PeriodEnd, ClosingDate, DueDate time.Time; TotalCents int64; Transactions []Transaction; Upcoming []UpcomingInstallment; Warnings []string }`
  - `type Provider interface { ID() string; Detect(*pdftext.Document) (DocKind, bool); Parse(*pdftext.Document, DocKind) (*Statement, error) }`
  - `var ErrUnknownDocument error`; `type ReconciliationError struct { Rule string; Computed, Printed int64 }`; `type LayoutError struct { Msg string }`
  - `func NormalizeKey(s string) string`
  - `type Registry`; `func NewRegistry(providers ...Provider) *Registry`; `func (*Registry) Detect(doc *pdftext.Document) (Provider, DocKind, error)`

- [ ] **Step 1: Adicionar dependência**

```bash
cd backend && go get golang.org/x/text@latest
```

- [ ] **Step 2: Escrever o teste que falha** — `backend/internal/provider/provider_test.go`

```go
package provider

import (
	"errors"
	"testing"

	"jgfinance/internal/pdftext"
)

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"PIX TRANSF Cassiê ":         "PIX TRANSF CASSIE",
		"Antônio":                    "ANTONIO",
		"DL    *UberRidesS":          "DL *UBERRIDESS",
		"  Redução Mensalidade - P ": "REDUCAO MENSALIDADE - P",
		"aliexpressSAO PAULOBR":      "ALIEXPRESSSAO PAULOBR",
		"Repasse de IOF em R$":       "REPASSE DE IOF EM R$",
		"PADARIA BOM PÃO":            "PADARIA BOM PAO",
	}
	for in, want := range cases {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReconciliationErrorMessage(t *testing.T) {
	err := &ReconciliationError{Rule: "compras = Lançamentos no cartão", Computed: 395466, Printed: 405286}
	want := "conferência falhou (compras = Lançamentos no cartão): calculado R$ 3.954,66, impresso R$ 4.052,86, diferença -R$ 98,20"
	if err.Error() != want {
		t.Errorf("got %q\nwant %q", err.Error(), want)
	}
	if got := (&LayoutError{Msg: "x"}).Error(); got != "layout inesperado: x" {
		t.Errorf("LayoutError = %q", got)
	}
}

type fakeProvider struct {
	id    string
	kind  DocKind
	match string
}

func (f fakeProvider) ID() string { return f.id }

func (f fakeProvider) Detect(doc *pdftext.Document) (DocKind, bool) {
	for _, p := range doc.Pages {
		for _, r := range p.Runs {
			if r.Text == f.match {
				return f.kind, true
			}
		}
	}
	return "", false
}

func (f fakeProvider) Parse(*pdftext.Document, DocKind) (*Statement, error) { return nil, nil }

func docWith(text string) *pdftext.Document {
	return &pdftext.Document{Pages: []pdftext.Page{{Runs: []pdftext.Run{{Text: text}}}}}
}

func TestRegistryDetect(t *testing.T) {
	reg := NewRegistry(fakeProvider{"a", CardStatement, "A"}, fakeProvider{"b", BankStatement, "B"})
	p, kind, err := reg.Detect(docWith("B"))
	if err != nil || p.ID() != "b" || kind != BankStatement {
		t.Errorf("Detect(B) = %v, %v, %v", p, kind, err)
	}
	if _, _, err := reg.Detect(docWith("C")); !errors.Is(err, ErrUnknownDocument) {
		t.Errorf("Detect(C) err = %v, want ErrUnknownDocument", err)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/provider/...`
Expected: FAIL com `undefined: NormalizeKey`, `undefined: ReconciliationError`, etc.

- [ ] **Step 4: Implementar** — `backend/internal/provider/provider.go`

```go
// Package provider defines the bank-agnostic statement model and the contract every bank
// integration implements.
package provider

import (
	"time"

	"jgfinance/internal/pdftext"
)

// DocKind is the type of document a provider recognized.
type DocKind string

const (
	CardStatement DocKind = "card_statement" // fatura de cartão
	BankStatement DocKind = "bank_statement" // extrato de conta
)

// TxKind classifies a transaction; only Expense and Refund count as spending.
type TxKind string

const (
	Expense     TxKind = "expense"
	Refund      TxKind = "refund"
	BillPayment TxKind = "bill_payment"
	OwnTransfer TxKind = "own_transfer"
	Investment  TxKind = "investment"
	Income      TxKind = "income"
	Ignored     TxKind = "ignored"
)

// AccountKind is the kind of account a statement belongs to.
type AccountKind string

const (
	CreditCard AccountKind = "credit_card"
	Checking   AccountKind = "checking"
)

// AccountRef identifies the card or account of a statement.
type AccountRef struct {
	Provider    string
	Kind        AccountKind
	ExternalID  string // card last 4 digits, or "agência/conta"
	DefaultName string
}

// Transaction is one normalized statement line.
type Transaction struct {
	Seq                 int // 1-based order within the document
	Date                time.Time
	Description         string // as printed
	Key                 string // normalized merchant/person key
	AmountCents         int64  // > 0 money leaving; < 0 money coming in / refund
	Kind                TxKind
	PersonTransfer      bool // PIX to a person: always asked the first time
	InstallmentN        int  // 0 when not an installment
	InstallmentTotal    int
	BankCategory        string
	City                string
	Holder              string
	OriginalAmountCents int64 // international purchases, in OriginalCurrency
	OriginalCurrency    string
}

// UpcomingInstallment is an installment listed under "próximas faturas", as printed.
type UpcomingInstallment struct {
	PurchaseDate time.Time
	Description  string
	Key          string
	AmountCents  int64
	N, Total     int
}

// Statement is everything a provider extracted from one document. Parse only returns a
// Statement after its amounts reconciled with the totals printed on the document.
type Statement struct {
	Kind                   DocKind
	Account                AccountRef
	PeriodStart, PeriodEnd time.Time // bank statements
	ClosingDate, DueDate   time.Time // card statements
	TotalCents             int64     // card statements: "Total desta fatura"
	Transactions           []Transaction
	Upcoming               []UpcomingInstallment
	Warnings               []string
}

// Provider is implemented by each bank integration.
type Provider interface {
	ID() string
	Detect(doc *pdftext.Document) (DocKind, bool)
	Parse(doc *pdftext.Document, kind DocKind) (*Statement, error)
}
```

`backend/internal/provider/errors.go`

```go
package provider

import (
	"errors"
	"fmt"

	"jgfinance/internal/money"
)

// ErrUnknownDocument is returned when no provider recognizes a document.
var ErrUnknownDocument = errors.New("nenhum banco reconheceu este arquivo")

// ReconciliationError means extracted amounts disagree with a total printed on the document.
type ReconciliationError struct {
	Rule     string
	Computed int64
	Printed  int64
}

func (e *ReconciliationError) Error() string {
	return fmt.Sprintf("conferência falhou (%s): calculado %s, impresso %s, diferença %s",
		e.Rule, money.FormatBRL(e.Computed), money.FormatBRL(e.Printed), money.FormatBRL(e.Computed-e.Printed))
}

// LayoutError means the document was recognized but its structure is not what the parser expects.
type LayoutError struct {
	Msg string
}

func (e *LayoutError) Error() string { return "layout inesperado: " + e.Msg }
```

`backend/internal/provider/key.go`

```go
package provider

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeKey uppercases s, removes accents and collapses whitespace. Providers call it
// after stripping their own noise (installment markers, glued dates) to build Transaction.Key.
func NormalizeKey(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	if out, _, err := transform.String(t, s); err == nil {
		s = out
	}
	return strings.Join(strings.Fields(strings.ToUpper(s)), " ")
}
```

`backend/internal/provider/registry.go`

```go
package provider

import "jgfinance/internal/pdftext"

// Registry holds the known providers, asked in registration order.
type Registry struct {
	providers []Provider
}

// NewRegistry returns a registry asking providers in the given order.
func NewRegistry(providers ...Provider) *Registry {
	return &Registry{providers: providers}
}

// Detect returns the first provider that recognizes doc.
func (r *Registry) Detect(doc *pdftext.Document) (Provider, DocKind, error) {
	for _, p := range r.providers {
		if kind, ok := p.Detect(doc); ok {
			return p, kind, nil
		}
	}
	return nil, "", ErrUnknownDocument
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go mod tidy && go test ./internal/provider/...`
Expected: `ok  	jgfinance/internal/provider`

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add backend/go.mod backend/go.sum backend/internal/provider
git commit -m "feat(provider): modelo comum de extrato, chave normalizada e registro de bancos

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Itaú — parse da fatura (linhas → Statement)

**Files:**
- Create: `backend/internal/provider/itau/dates.go`
- Create: `backend/internal/provider/itau/fatura.go`
- Test: `backend/internal/provider/itau/dates_test.go`
- Test: `backend/internal/provider/itau/fatura_test.go`

**Interfaces:**
- Consumes: `layout.Line` (Task 3); `money.Pattern`, `money.ParseBRL`, `money.FormatBRL` (Task 1); tipos, `NormalizeKey`, `LayoutError` de `provider` (Task 4).
- Produces (internos ao pacote `itau`, usados pelas Tasks 6–8):
  - `func parseDate(s string) (time.Time, error)` — `"dd/mm/aaaa"`, UTC.
  - `func cardDate(dd, mm string, closing time.Time) (time.Time, error)` — data mais recente, não posterior ao fechamento (tenta o ano do fechamento e o anterior).
  - `func splitInstallment(desc string) (n, total int, base string)`
  - `type faturaHeader struct { last4, tier string; closing, due time.Time; total, current, financed int64; hasCurrent, hasFinanced bool }`
  - `type section int` com `secNone, secPayments, secPurchases, secFees, secIntl, secUpcoming`
  - `type faturaItem struct { sec section; tx provider.Transaction }`
  - `type faturaBody struct { items []faturaItem; upcoming []provider.UpcomingInstallment; totals map[string]int64; seen map[string]bool }`
  - Constantes de rótulo `lblPayments, lblPurchases, lblFees, lblIntl, lblIOF, lblIntlTotal, lblCurrent, lblNextInvoice, lblLaterInvoices, lblUpcomingTotal`
  - `func parseFaturaHeader(text string) (faturaHeader, error)`
  - `func parseFaturaBody(lines []layout.Line, closing time.Time) (*faturaBody, error)`
  - `func buildFaturaStatement(h faturaHeader, f *faturaBody, warnings []string) *provider.Statement`
  - `func parseFaturaLines(header string, body []layout.Line) (*provider.Statement, error)` — nesta task **sem** conferência (a Task 6 a adiciona).

Regras (spec §7.2): seções por prefixo de linha; linha de transação `^(\d{2})/(\d{2})\s+(.+?)\s+(valor)$`; linha de total por rótulo; primeira linha após uma compra = `categoria cidade`; após uma compra internacional = `valor MOEDA valorUSD`; titular = linha em maiúsculas logo antes do cabeçalho `DATA …` em compras/internacionais; parcela = sufixo ` NN/NN` (com `1 <= n <= total`); sinal: pagamentos → `bill_payment`, positivo → `expense`, negativo → `refund`; `Repasse de IOF em R$` vira transação na data do fechamento.

- [ ] **Step 0: Adicionar a dependência de testes**

```bash
cd backend && go get github.com/google/go-cmp@latest
```

- [ ] **Step 1: Escrever os testes que falham** — `backend/internal/provider/itau/dates_test.go`

```go
package itau

import (
	"testing"
	"time"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestParseDate(t *testing.T) {
	got, err := parseDate("24/09/2026")
	if err != nil || !got.Equal(day(2026, 9, 24)) {
		t.Errorf("parseDate = %v, %v", got, err)
	}
	if _, err := parseDate("31/02/2026"); err == nil {
		t.Error("31/02/2026 should fail")
	}
}

func TestCardDate(t *testing.T) {
	cases := []struct {
		dd, mm  string
		closing time.Time
		want    time.Time
	}{
		{"28", "12", day(2026, 1, 10), day(2025, 12, 28)}, // December purchase on a January invoice
		{"05", "01", day(2026, 1, 10), day(2026, 1, 5)},
		{"10", "01", day(2026, 1, 10), day(2026, 1, 10)}, // same day as closing
		{"15", "11", day(2026, 2, 24), day(2025, 11, 15)}, // installment bought last year
		{"29", "02", day(2029, 1, 10), day(2028, 2, 29)},  // leap day only exists in the previous year
	}
	for _, c := range cases {
		got, err := cardDate(c.dd, c.mm, c.closing)
		if err != nil || !got.Equal(c.want) {
			t.Errorf("cardDate(%s/%s, %s) = %v, %v; want %v", c.dd, c.mm, c.closing.Format("02/01/2006"), got, err, c.want)
		}
	}
	if _, err := cardDate("31", "02", day(2026, 3, 10)); err == nil {
		t.Error("31/02 should fail")
	}
}
```

`backend/internal/provider/itau/fatura_test.go`

```go
package itau

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"jgfinance/internal/layout"
	"jgfinance/internal/provider"
)

func toLines(ss []string) []layout.Line {
	out := make([]layout.Line, len(ss))
	for i, s := range ss {
		out[i] = layout.Line{Text: s}
	}
	return out
}

// Invented invoice (no real data) covering every section and the page noise seen in real PDFs.
var faturaHeaderFixture = strings.Join([]string{
	"Platinum",
	"FULANO DE TAL Resumo da fatura em R$",
	"RUA EXEMPLO, 100 Total da fatura anterior 1.000,00",
	"Pagamento efetuado em 01/02/2026 -1.000,00",
	"S Saldo financiado 0,00",
	"L Lançamentos atuais 1.164,89",
	"Postagem: 24/02/2026",
	"Vencimento: 03/03/2026 = Total desta fatura 1.164,89",
	"Emissão: 24/02/2026",
	"Titular FULANO DE TAL",
	"Cartão 4111.XXXX.XXXX.1234",
	"Banco Itaú S.A. 341-7",
	"ITAU UNIBANCO HOLDING S.A.",
}, "\n")

var faturaBodyFixture = []string{
	"Caso você pague um valor entre o mínimo e o total de sua fatura, o saldo restante",
	"Pagamentos efetuados",
	"DATA VALOR EM R$",
	"01/02 Pagamento via conta -1.000,00",
	"P Total dos pagamentos -1.000,00",
	"Lançamentos: compras e saques",
	"FULANO DE TAL",
	"DATA ESTABELECIMENTO VALOR EM R$",
	"15/11 LOJA EXEMPLO 03/10 150,00",
	"vestuário SAO PAULO",
	"28/01 PADARIA BOM PÃO 25,50",
	"restaurante FLORIANOPOLIS",
	"02/02 99*CORRIDA 12,90",
	"transporte SAO PAULO",
	"4004 4828",
	"0800 970 4828",
	"Lançamentos: compras e saques",
	"DATA ESTABELECIMENTO VALOR EM R$",
	"10/02 MERCADO CENTRAL 300,00",
	"supermercado PORTO BELO",
	"12/02 aliexpressLOJA 100,00",
	"serviços SAO PAULO",
	"12/02 aliexpressLOJA -40,00",
	"serviços SAO PAULO",
	"20/02 FARMACIA SAUDE 45,00",
	"saúde",
	"continua...",
	"Lançamentos no cartão 593,40",
	"Lançamentos internacionais",
	"FULANO DE TAL",
	"DATA ESTABELECIMENTO US$ R$",
	"05/02 STREAMING INC 110,00",
	"20,00 USD 20,00",
	"Dólar de Conversão R$ 5,50",
	"10/02 SOFTWARE CO 432,50",
	"400,00 BRL 80,00",
	"Dólar de Conversão R$ 5,40",
	"Total transações inter. em R$ 542,50",
	"Repasse de IOF em R$ 18,99",
	"Total lançamentos inter. em R$ 561,49",
	"Lançamentos: produtos e serviços",
	"DATA PRODUTOS/SERVIÇOS VALOR EM R$",
	"25/01 Mensalidade - Plano do 20,00",
	"Anuidade Diferenciada",
	"03/02 Redução Mensalidade - P -10,00",
	"Anuidade Diferenciada",
	"Lançamentos produtos e serviços 10,00",
	"L Total dos lançamentos atuais 1.164,89",
	"Compras parceladas - próximas faturas",
	"DATA ESTABELECIMENTO VALOR EM R$",
	"15/11 LOJA EXEMPLO 04/10 150,00",
	"Próxima fatura 150,00",
	"Demais faturas 900,00",
	"Total para próximas faturas 1.050,00",
	"Limites de crédito Valor em R$",
	"Limite total de crédito 5.000,00",
	"Encargos cobrados nesta fatura",
	"Juros do rotativo 16,10 % 0,00",
	"Total a pagar R$ 5.189,74",
}

func TestParseFaturaLines(t *testing.T) {
	st, err := parseFaturaLines(faturaHeaderFixture, toLines(faturaBodyFixture))
	if err != nil {
		t.Fatalf("parseFaturaLines: %v", err)
	}
	const holder = "FULANO DE TAL"
	want := &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{Provider: "itau", Kind: provider.CreditCard, ExternalID: "1234", DefaultName: "Itaú Platinum •••• 1234"},
		ClosingDate: day(2026, 2, 24),
		DueDate:     day(2026, 3, 3),
		TotalCents:  116489,
		Transactions: []provider.Transaction{
			{Seq: 1, Date: day(2026, 2, 1), Description: "Pagamento via conta", Key: "PAGAMENTO VIA CONTA", AmountCents: -100000, Kind: provider.BillPayment},
			{Seq: 2, Date: day(2025, 11, 15), Description: "LOJA EXEMPLO 03/10", Key: "LOJA EXEMPLO", AmountCents: 15000, Kind: provider.Expense, InstallmentN: 3, InstallmentTotal: 10, BankCategory: "vestuário", City: "SAO PAULO", Holder: holder},
			{Seq: 3, Date: day(2026, 1, 28), Description: "PADARIA BOM PÃO", Key: "PADARIA BOM PAO", AmountCents: 2550, Kind: provider.Expense, BankCategory: "restaurante", City: "FLORIANOPOLIS", Holder: holder},
			{Seq: 4, Date: day(2026, 2, 2), Description: "99*CORRIDA", Key: "99*CORRIDA", AmountCents: 1290, Kind: provider.Expense, BankCategory: "transporte", City: "SAO PAULO", Holder: holder},
			{Seq: 5, Date: day(2026, 2, 10), Description: "MERCADO CENTRAL", Key: "MERCADO CENTRAL", AmountCents: 30000, Kind: provider.Expense, BankCategory: "supermercado", City: "PORTO BELO", Holder: holder},
			{Seq: 6, Date: day(2026, 2, 12), Description: "aliexpressLOJA", Key: "ALIEXPRESSLOJA", AmountCents: 10000, Kind: provider.Expense, BankCategory: "serviços", City: "SAO PAULO", Holder: holder},
			{Seq: 7, Date: day(2026, 2, 12), Description: "aliexpressLOJA", Key: "ALIEXPRESSLOJA", AmountCents: -4000, Kind: provider.Refund, BankCategory: "serviços", City: "SAO PAULO", Holder: holder},
			{Seq: 8, Date: day(2026, 2, 20), Description: "FARMACIA SAUDE", Key: "FARMACIA SAUDE", AmountCents: 4500, Kind: provider.Expense, BankCategory: "saúde", Holder: holder},
			{Seq: 9, Date: day(2026, 2, 5), Description: "STREAMING INC", Key: "STREAMING INC", AmountCents: 11000, Kind: provider.Expense, Holder: holder, OriginalAmountCents: 2000, OriginalCurrency: "USD"},
			{Seq: 10, Date: day(2026, 2, 10), Description: "SOFTWARE CO", Key: "SOFTWARE CO", AmountCents: 43250, Kind: provider.Expense, Holder: holder, OriginalAmountCents: 40000, OriginalCurrency: "BRL"},
			{Seq: 11, Date: day(2026, 1, 25), Description: "Mensalidade - Plano do", Key: "MENSALIDADE - PLANO DO", AmountCents: 2000, Kind: provider.Expense},
			{Seq: 12, Date: day(2026, 2, 3), Description: "Redução Mensalidade - P", Key: "REDUCAO MENSALIDADE - P", AmountCents: -1000, Kind: provider.Refund},
			{Seq: 13, Date: day(2026, 2, 24), Description: "Repasse de IOF em R$", Key: "REPASSE DE IOF EM R$", AmountCents: 1899, Kind: provider.Expense},
		},
		Upcoming: []provider.UpcomingInstallment{
			{PurchaseDate: day(2025, 11, 15), Description: "LOJA EXEMPLO 04/10", Key: "LOJA EXEMPLO", AmountCents: 15000, N: 4, Total: 10},
		},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("statement mismatch (-want +got):\n%s", diff)
	}
}

func TestParseFaturaHeaderRequiresCardAndDates(t *testing.T) {
	for _, drop := range []string{"Cartão 4111.XXXX.XXXX.1234", "Postagem: 24/02/2026"} {
		header := strings.Replace(faturaHeaderFixture, drop, "", 1)
		_, err := parseFaturaLines(header, toLines(faturaBodyFixture))
		var le *provider.LayoutError
		if !errors.As(err, &le) {
			t.Errorf("without %q: err = %v, want *provider.LayoutError", drop, err)
		}
	}
}

func TestSplitInstallment(t *testing.T) {
	cases := []struct {
		in       string
		n, total int
		base     string
	}{
		{"LOJA EXEMPLO 03/10", 3, 10, "LOJA EXEMPLO"},
		{"KIWIFY*Worksho 02/04", 2, 4, "KIWIFY*Worksho"},
		{"MP*INGRESSOS949FLORIANO", 0, 0, "MP*INGRESSOS949FLORIANO"},
		{"EVENTO 31/12", 0, 0, "EVENTO 31/12"}, // 31 > 12: a date, not n/N
		{"LOJA 00/10", 0, 0, "LOJA 00/10"},
	}
	for _, c := range cases {
		n, total, base := splitInstallment(c.in)
		if n != c.n || total != c.total || base != c.base {
			t.Errorf("splitInstallment(%q) = %d, %d, %q", c.in, n, total, base)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/provider/itau/...`
Expected: FAIL com `undefined: parseDate`, `undefined: cardDate`, `undefined: parseFaturaLines`, `undefined: splitInstallment`.

- [ ] **Step 3: Implementar datas** — `backend/internal/provider/itau/dates.go`

```go
package itau

import (
	"fmt"
	"strconv"
	"time"

	"jgfinance/internal/provider"
)

// parseDate parses "dd/mm/aaaa" as a civil date in UTC.
func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		return time.Time{}, &provider.LayoutError{Msg: fmt.Sprintf("data inválida %q", s)}
	}
	return t, nil
}

// cardDate resolves an invoice "dd/mm" to the most recent valid date that is not after the
// closing date (invoices print purchases and installments without the year).
func cardDate(dd, mm string, closing time.Time) (time.Time, error) {
	d, errD := strconv.Atoi(dd)
	m, errM := strconv.Atoi(mm)
	if errD == nil && errM == nil {
		for _, y := range []int{closing.Year(), closing.Year() - 1} {
			t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
			if t.Day() == d && int(t.Month()) == m && !t.After(closing) {
				return t, nil
			}
		}
	}
	return time.Time{}, &provider.LayoutError{Msg: fmt.Sprintf("data inválida %s/%s", dd, mm)}
}
```

- [ ] **Step 4: Implementar o parse** — `backend/internal/provider/itau/fatura.go`

```go
package itau

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"jgfinance/internal/layout"
	"jgfinance/internal/money"
	"jgfinance/internal/provider"
)

// section is the invoice block a body line belongs to.
type section int

const (
	secNone section = iota
	secPayments
	secPurchases
	secFees
	secIntl
	secUpcoming
)

// sectionStarts maps line prefixes to the section they open; secNone closes the current one.
var sectionStarts = []struct {
	prefix string
	sec    section
}{
	{"Pagamentos efetuados", secPayments},
	{"Lançamentos: compras e saques", secPurchases},
	{"Lançamentos: produtos e serviços", secFees},
	{"Lançamentos internacionais", secIntl},
	{"Compras parceladas - próximas faturas", secUpcoming},
	{"Lançamentos no cartão", secNone},
	{"Encargos cobrados", secNone},
	{"Limites de crédito", secNone},
	{"Novo teto", secNone},
	{"Simulação", secNone},
	{"Credito Rotativo", secNone},
	{"Fique atento", secNone},
}

// Labels of the totals printed in the invoice body.
const (
	lblPayments      = "P Total dos pagamentos"
	lblPurchases     = "Lançamentos no cartão"
	lblFees          = "Lançamentos produtos e serviços"
	lblIntl          = "Total transações inter. em R$"
	lblIOF           = "Repasse de IOF em R$"
	lblIntlTotal     = "Total lançamentos inter. em R$"
	lblCurrent       = "L Total dos lançamentos atuais"
	lblNextInvoice   = "Próxima fatura"
	lblLaterInvoices = "Demais faturas"
	lblUpcomingTotal = "Total para próximas faturas"
)

var (
	faturaTxRe    = regexp.MustCompile(`^(\d{2})/(\d{2})\s+(.+?)\s+(` + money.Pattern + `)$`)
	faturaTotalRe = regexp.MustCompile(`^(` + alternation(lblPayments, lblPurchases, lblFees, lblIntl, lblIOF,
		lblIntlTotal, lblCurrent, lblNextInvoice, lblLaterInvoices, lblUpcomingTotal) + `)\s+(` + money.Pattern + `)$`)
	installmentRe  = regexp.MustCompile(`\s(\d{2})/(\d{2})$`)
	intlOriginalRe = regexp.MustCompile(`^(` + money.Pattern + `)\s+([A-Z]{3})\s+` + money.Pattern + `$`)
	holderRe       = regexp.MustCompile(`^[A-ZÀ-Ý]+(?: [A-ZÀ-Ý]+)+$`)
	cardRe         = regexp.MustCompile(`Cartão\s+\d{4}\.XXXX\.XXXX\.(\d{4})`)
	tierRe         = regexp.MustCompile(`\b(Platinum|Infinite|Gold|Black|Signature|Grafite|Nanquim|Internacional)\b`)
	closingRe      = regexp.MustCompile(`Postagem:\s*(\d{2}/\d{2}/\d{4})`)
	dueRe          = regexp.MustCompile(`Vencimento:\s*(\d{2}/\d{2}/\d{4})`)
	invoiceTotalRe = regexp.MustCompile(`Total desta fatura\s+(` + money.Pattern + `)`)
	currentRe      = regexp.MustCompile(`Lançamentos atuais\s+(` + money.Pattern + `)`)
	financedRe     = regexp.MustCompile(`Saldo financiado\s+(` + money.Pattern + `)`)
)

func alternation(labels ...string) string {
	quoted := make([]string, len(labels))
	for i, l := range labels {
		quoted[i] = regexp.QuoteMeta(l)
	}
	return strings.Join(quoted, "|")
}

// faturaHeader holds the fields printed on page 1.
type faturaHeader struct {
	last4, tier             string
	closing, due            time.Time
	total                   int64
	current, financed       int64
	hasCurrent, hasFinanced bool
}

type faturaItem struct {
	sec section
	tx  provider.Transaction
}

// faturaBody is what the body pages yielded, before reconciliation.
type faturaBody struct {
	items    []faturaItem
	upcoming []provider.UpcomingInstallment
	totals   map[string]int64 // summed per label
	seen     map[string]bool
}

// parseFaturaLines turns the page-1 text and the body lines (pages 2+, reading order)
// into a Statement.
func parseFaturaLines(header string, body []layout.Line) (*provider.Statement, error) {
	h, err := parseFaturaHeader(header)
	if err != nil {
		return nil, err
	}
	f, err := parseFaturaBody(body, h.closing)
	if err != nil {
		return nil, err
	}
	return buildFaturaStatement(h, f, nil), nil
}

func parseFaturaHeader(text string) (faturaHeader, error) {
	var h faturaHeader
	m := cardRe.FindStringSubmatch(text)
	if m == nil {
		return h, &provider.LayoutError{Msg: "número do cartão não encontrado na fatura"}
	}
	h.last4 = m[1]
	if m := tierRe.FindStringSubmatch(text); m != nil {
		h.tier = m[1]
	}
	var err error
	if h.closing, err = findDate(closingRe, text, "Postagem"); err != nil {
		return h, err
	}
	if h.due, err = findDate(dueRe, text, "Vencimento"); err != nil {
		return h, err
	}
	var ok bool
	if h.total, ok, err = findAmount(invoiceTotalRe, text); err != nil {
		return h, err
	}
	if !ok {
		return h, &provider.LayoutError{Msg: `"Total desta fatura" não encontrado`}
	}
	if h.current, h.hasCurrent, err = findAmount(currentRe, text); err != nil {
		return h, err
	}
	if h.financed, h.hasFinanced, err = findAmount(financedRe, text); err != nil {
		return h, err
	}
	return h, nil
}

func findDate(re *regexp.Regexp, text, label string) (time.Time, error) {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return time.Time{}, &provider.LayoutError{Msg: fmt.Sprintf("%q não encontrado", label)}
	}
	return parseDate(m[1])
}

func findAmount(re *regexp.Regexp, text string) (int64, bool, error) {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0, false, nil
	}
	v, err := money.ParseBRL(m[1])
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func sectionStart(text string) (section, bool) {
	for _, s := range sectionStarts {
		if strings.HasPrefix(text, s.prefix) {
			return s.sec, true
		}
	}
	return secNone, false
}

func parseFaturaBody(lines []layout.Line, closing time.Time) (*faturaBody, error) {
	f := &faturaBody{totals: map[string]int64{}, seen: map[string]bool{}}
	sec, holder, prev := secNone, "", ""
	last := -1 // index in f.items of the transaction whose detail line may follow
	for _, l := range lines {
		text := l.Text
		if m := faturaTotalRe.FindStringSubmatch(text); m != nil {
			v, err := money.ParseBRL(m[2])
			if err != nil {
				return nil, err
			}
			f.totals[m[1]] += v
			f.seen[m[1]] = true
			if m[1] == lblPurchases {
				sec = secNone
			}
			prev, last = text, -1
			continue
		}
		if s, ok := sectionStart(text); ok {
			sec = s
			prev, last = text, -1
			continue
		}
		if strings.HasPrefix(text, "DATA ") {
			if (sec == secPurchases || sec == secIntl) && holderRe.MatchString(prev) {
				holder = prev
			}
			prev, last = text, -1
			continue
		}
		if m := faturaTxRe.FindStringSubmatch(text); m != nil && sec != secNone {
			idx, err := f.add(sec, m, closing, holder)
			if err != nil {
				return nil, err
			}
			prev, last = text, idx
			continue
		}
		if last >= 0 {
			f.attachDetail(last, text)
			last = -1
		}
		prev = text
	}
	return f, nil
}

// add records a transaction line; it returns the item index, or -1 for upcoming installments.
func (f *faturaBody) add(sec section, m []string, closing time.Time, holder string) (int, error) {
	date, err := cardDate(m[1], m[2], closing)
	if err != nil {
		return -1, err
	}
	amount, err := money.ParseBRL(m[4])
	if err != nil {
		return -1, err
	}
	desc := m[3]
	n, total, base := splitInstallment(desc)
	key := provider.NormalizeKey(base)
	if sec == secUpcoming {
		if n == 0 {
			return -1, &provider.LayoutError{Msg: fmt.Sprintf("parcela futura sem n/N: %q", desc)}
		}
		f.upcoming = append(f.upcoming, provider.UpcomingInstallment{
			PurchaseDate: date, Description: desc, Key: key, AmountCents: amount, N: n, Total: total,
		})
		return -1, nil
	}
	tx := provider.Transaction{
		Date: date, Description: desc, Key: key, AmountCents: amount,
		Kind: kindBySign(amount), InstallmentN: n, InstallmentTotal: total,
	}
	switch sec {
	case secPayments:
		tx.Kind = provider.BillPayment
	case secPurchases, secIntl:
		tx.Holder = holder
	}
	f.items = append(f.items, faturaItem{sec: sec, tx: tx})
	return len(f.items) - 1, nil
}

// attachDetail reads the line printed right below a transaction.
func (f *faturaBody) attachDetail(i int, text string) {
	it := &f.items[i]
	switch it.sec {
	case secPurchases:
		it.tx.BankCategory, it.tx.City, _ = strings.Cut(text, " ")
	case secIntl:
		if m := intlOriginalRe.FindStringSubmatch(text); m != nil {
			if v, err := money.ParseBRL(m[1]); err == nil {
				it.tx.OriginalAmountCents, it.tx.OriginalCurrency = v, m[2]
			}
		}
	}
}

func kindBySign(cents int64) provider.TxKind {
	if cents < 0 {
		return provider.Refund
	}
	return provider.Expense
}

// splitInstallment detects a trailing " NN/NN" installment marker (1 <= n <= total).
func splitInstallment(desc string) (n, total int, base string) {
	loc := installmentRe.FindStringSubmatchIndex(desc)
	if loc == nil {
		return 0, 0, desc
	}
	n, _ = strconv.Atoi(desc[loc[2]:loc[3]])
	total, _ = strconv.Atoi(desc[loc[4]:loc[5]])
	if n < 1 || n > total {
		return 0, 0, desc
	}
	return n, total, strings.TrimSpace(desc[:loc[0]])
}

func buildFaturaStatement(h faturaHeader, f *faturaBody, warnings []string) *provider.Statement {
	name := "Itaú •••• " + h.last4
	if h.tier != "" {
		name = "Itaú " + h.tier + " •••• " + h.last4
	}
	st := &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{Provider: "itau", Kind: provider.CreditCard, ExternalID: h.last4, DefaultName: name},
		ClosingDate: h.closing,
		DueDate:     h.due,
		TotalCents:  h.total,
		Upcoming:    f.upcoming,
		Warnings:    warnings,
	}
	for _, it := range f.items {
		st.Transactions = append(st.Transactions, it.tx)
	}
	if iof := f.totals[lblIOF]; iof != 0 {
		st.Transactions = append(st.Transactions, provider.Transaction{
			Date: h.closing, Description: lblIOF, Key: provider.NormalizeKey(lblIOF),
			AmountCents: iof, Kind: kindBySign(iof),
		})
	}
	for i := range st.Transactions {
		st.Transactions[i].Seq = i + 1
	}
	return st
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/provider/itau/...`
Expected: `ok  	jgfinance/internal/provider/itau`

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add backend/internal/provider/itau
git commit -m "feat(itau): parse da fatura de cartão (seções, parcelas, internacionais, IOF)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Itaú — conferência da fatura contra os totais impressos

**Files:**
- Create: `backend/internal/provider/itau/fatura_reconcile.go`
- Modify: `backend/internal/provider/itau/fatura.go` (função `parseFaturaLines`)
- Test: `backend/internal/provider/itau/fatura_reconcile_test.go`

**Interfaces:**
- Consumes: `faturaHeader`, `faturaBody`, `faturaItem`, `section` e rótulos `lbl*` (Task 5); `provider.ReconciliationError`, `provider.LayoutError` (Task 4); `money.FormatBRL` (Task 1).
- Produces: `func reconcileFatura(h faturaHeader, f *faturaBody) ([]string, error)` — devolve avisos (nil quando não há) ou o primeiro erro; `parseFaturaLines` passa a chamá-la.

Regras (spec §7.2; cada regra é pulada quando o rótulo não aparece no documento):
0. Σ pagamentos = `P Total dos pagamentos`.
1. Σ compras = Σ `Lançamentos no cartão` (há compras mas não há o total → `LayoutError`).
2. Σ produtos = `Lançamentos produtos e serviços` (há produtos sem total → `LayoutError`).
3. Σ internacionais = `Total transações inter. em R$` (há internacionais sem total → `LayoutError`); Σ internacionais + IOF = `Total lançamentos inter. em R$`.
4. Σ compras + Σ produtos + Σ internacionais + IOF = `L Total dos lançamentos atuais` e = `Lançamentos atuais` do resumo (página 1).
5. `Saldo financiado` + `Lançamentos atuais` = `Total desta fatura`.
6. Σ parcelas listadas em "próximas faturas" = `Próxima fatura`.
7. Aviso (não erro): Σ (N − n) × valor de cada parcela listada ≠ `Demais faturas`.

- [ ] **Step 1: Escrever o teste que falha** — `backend/internal/provider/itau/fatura_reconcile_test.go`

```go
package itau

import (
	"errors"
	"strings"
	"testing"

	"jgfinance/internal/provider"
)

// replaceLine returns a copy of lines with the first line equal to old replaced (or removed when repl is "").
func replaceLine(t *testing.T, lines []string, old, repl string) []string {
	t.Helper()
	out := make([]string, 0, len(lines))
	found := false
	for _, l := range lines {
		if !found && l == old {
			found = true
			if repl != "" {
				out = append(out, repl)
			}
			continue
		}
		out = append(out, l)
	}
	if !found {
		t.Fatalf("fixture line %q not found", old)
	}
	return out
}

func wantReconciliation(t *testing.T, err error, ruleContains string, computed, printed int64) {
	t.Helper()
	var re *provider.ReconciliationError
	if !errors.As(err, &re) {
		t.Fatalf("err = %v, want *provider.ReconciliationError", err)
	}
	if !strings.Contains(re.Rule, ruleContains) || re.Computed != computed || re.Printed != printed {
		t.Errorf("got %+v, want rule containing %q, computed %d, printed %d", re, ruleContains, computed, printed)
	}
}

func TestReconcileFaturaFixturePasses(t *testing.T) {
	st, err := parseFaturaLines(faturaHeaderFixture, toLines(faturaBodyFixture))
	if err != nil {
		t.Fatalf("fixture must reconcile: %v", err)
	}
	if st.Warnings != nil {
		t.Errorf("warnings = %v, want none", st.Warnings)
	}
}

func TestReconcileFaturaPurchasesMismatch(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "28/01 PADARIA BOM PÃO 25,50", "28/01 PADARIA BOM PÃO 26,50")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	wantReconciliation(t, err, lblPurchases, 59440, 59340)
}

func TestReconcileFaturaPurchasesWithoutTotalIsLayoutError(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Lançamentos no cartão 593,40", "")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	var le *provider.LayoutError
	if !errors.As(err, &le) {
		t.Errorf("err = %v, want *provider.LayoutError", err)
	}
}

func TestReconcileFaturaIOFMismatch(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Total lançamentos inter. em R$ 561,49", "Total lançamentos inter. em R$ 561,50")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	wantReconciliation(t, err, lblIntlTotal, 56149, 56150)
}

func TestReconcileFaturaHeaderTotalMismatch(t *testing.T) {
	header := strings.Replace(faturaHeaderFixture, "Total desta fatura 1.164,89", "Total desta fatura 1.164,90", 1)
	_, err := parseFaturaLines(header, toLines(faturaBodyFixture))
	wantReconciliation(t, err, "Total desta fatura", 116489, 116490)
}

func TestReconcileFaturaNextInvoiceMismatch(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Próxima fatura 150,00", "Próxima fatura 151,00")
	_, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	wantReconciliation(t, err, lblNextInvoice, 15000, 15100)
}

func TestReconcileFaturaLaterInvoicesOnlyWarns(t *testing.T) {
	body := replaceLine(t, faturaBodyFixture, "Demais faturas 900,00", "Demais faturas 899,00")
	st, err := parseFaturaLines(faturaHeaderFixture, toLines(body))
	if err != nil {
		t.Fatalf("projection mismatch must not fail the import: %v", err)
	}
	if len(st.Warnings) != 1 || !strings.Contains(st.Warnings[0], "Demais faturas") {
		t.Errorf("warnings = %v", st.Warnings)
	}
}

// Review focus: a second cardholder with its own subtotal, and no international/fees sections.
func TestParseFaturaTwoHolders(t *testing.T) {
	header := strings.Join([]string{
		"Resumo da fatura em R$",
		"S Saldo financiado 0,00",
		"L Lançamentos atuais 380,00",
		"Postagem: 24/02/2026",
		"Vencimento: 03/03/2026 = Total desta fatura 380,00",
		"Cartão 4111.XXXX.XXXX.1234",
	}, "\n")
	body := []string{
		"Lançamentos: compras e saques",
		"FULANO DE TAL",
		"DATA ESTABELECIMENTO VALOR EM R$",
		"10/02 MERCADO CENTRAL 300,00",
		"supermercado PORTO BELO",
		"Lançamentos no cartão 300,00",
		"Lançamentos: compras e saques",
		"CICLANA DE TAL",
		"DATA ESTABELECIMENTO VALOR EM R$",
		"11/02 LIVRARIA X 80,00",
		"educação SAO PAULO",
		"Lançamentos no cartão 80,00",
		"L Total dos lançamentos atuais 380,00",
	}
	st, err := parseFaturaLines(header, toLines(body))
	if err != nil {
		t.Fatalf("parseFaturaLines: %v", err)
	}
	if len(st.Transactions) != 2 || st.Transactions[0].Holder != "FULANO DE TAL" || st.Transactions[1].Holder != "CICLANA DE TAL" {
		t.Errorf("transactions = %+v", st.Transactions)
	}
	if st.Account.DefaultName != "Itaú •••• 1234" {
		t.Errorf("DefaultName = %q", st.Account.DefaultName)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/provider/itau/... -run 'Reconcile|TwoHolders'`
Expected: FAIL — os testes de divergência passam a receber `err == nil` (ainda não há conferência), por exemplo `err = <nil>, want *provider.ReconciliationError`.

- [ ] **Step 3: Implementar** — `backend/internal/provider/itau/fatura_reconcile.go`

```go
package itau

import (
	"fmt"

	"jgfinance/internal/money"
	"jgfinance/internal/provider"
)

// reconcileFatura checks every printed total against the extracted items (spec §7.2).
// Rules whose label is absent from the document are skipped.
func reconcileFatura(h faturaHeader, f *faturaBody) ([]string, error) {
	sums := map[section]int64{}
	counts := map[section]int{}
	for _, it := range f.items {
		sums[it.sec] += it.tx.AmountCents
		counts[it.sec]++
	}
	check := func(rule string, computed, printed int64) error {
		if computed != printed {
			return &provider.ReconciliationError{Rule: rule, Computed: computed, Printed: printed}
		}
		return nil
	}
	requireTotal := func(sec section, label, what string) error {
		if counts[sec] > 0 && !f.seen[label] {
			return &provider.LayoutError{Msg: fmt.Sprintf("%s sem o total %q", what, label)}
		}
		return nil
	}

	iof := f.totals[lblIOF]
	current := sums[secPurchases] + sums[secFees] + sums[secIntl] + iof
	var upcomingNext, projectedLater int64
	for _, u := range f.upcoming {
		upcomingNext += u.AmountCents
		projectedLater += int64(u.Total-u.N) * u.AmountCents
	}

	steps := []func() error{
		func() error { return requireTotal(secPurchases, lblPurchases, "compras") },
		func() error { return requireTotal(secFees, lblFees, "produtos e serviços") },
		func() error { return requireTotal(secIntl, lblIntl, "compras internacionais") },
	}
	if f.seen[lblPayments] {
		steps = append(steps, func() error { return check("pagamentos = "+lblPayments, sums[secPayments], f.totals[lblPayments]) })
	}
	if f.seen[lblPurchases] {
		steps = append(steps, func() error { return check("compras = "+lblPurchases, sums[secPurchases], f.totals[lblPurchases]) })
	}
	if f.seen[lblFees] {
		steps = append(steps, func() error { return check("produtos e serviços = "+lblFees, sums[secFees], f.totals[lblFees]) })
	}
	if f.seen[lblIntl] {
		steps = append(steps, func() error { return check("internacionais = "+lblIntl, sums[secIntl], f.totals[lblIntl]) })
	}
	if f.seen[lblIntlTotal] {
		steps = append(steps, func() error {
			return check("internacionais + IOF = "+lblIntlTotal, sums[secIntl]+iof, f.totals[lblIntlTotal])
		})
	}
	if f.seen[lblCurrent] {
		steps = append(steps, func() error {
			return check("compras + produtos + internacionais + IOF = "+lblCurrent, current, f.totals[lblCurrent])
		})
	}
	if h.hasCurrent {
		steps = append(steps, func() error {
			return check("compras + produtos + internacionais + IOF = Lançamentos atuais (resumo)", current, h.current)
		})
	}
	if h.hasCurrent && h.hasFinanced {
		steps = append(steps, func() error {
			return check("Saldo financiado + Lançamentos atuais = Total desta fatura", h.financed+h.current, h.total)
		})
	}
	if f.seen[lblNextInvoice] {
		steps = append(steps, func() error { return check("parcelas listadas = "+lblNextInvoice, upcomingNext, f.totals[lblNextInvoice]) })
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}

	var warnings []string
	if f.seen[lblLaterInvoices] && projectedLater != f.totals[lblLaterInvoices] {
		warnings = append(warnings, fmt.Sprintf("projeção das parcelas restantes (%s) difere de %q (%s)",
			money.FormatBRL(projectedLater), lblLaterInvoices, money.FormatBRL(f.totals[lblLaterInvoices])))
	}
	return warnings, nil
}
```

- [ ] **Step 4: Ligar a conferência no parse** — em `backend/internal/provider/itau/fatura.go`, substituir o corpo de `parseFaturaLines` por:

```go
func parseFaturaLines(header string, body []layout.Line) (*provider.Statement, error) {
	h, err := parseFaturaHeader(header)
	if err != nil {
		return nil, err
	}
	f, err := parseFaturaBody(body, h.closing)
	if err != nil {
		return nil, err
	}
	warnings, err := reconcileFatura(h, f)
	if err != nil {
		return nil, err
	}
	return buildFaturaStatement(h, f, warnings), nil
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `cd backend && go test ./internal/provider/itau/...`
Expected: `ok  	jgfinance/internal/provider/itau` (incluindo `TestParseFaturaLines` da Task 5, que continua passando porque o fixture reconcilia).

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add backend/internal/provider/itau
git commit -m "feat(itau): confere a fatura centavo a centavo contra os totais impressos

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Itaú — extrato de conta (parse + conferência por saldo diário)

**Files:**
- Create: `backend/internal/provider/itau/extrato.go`
- Test: `backend/internal/provider/itau/extrato_test.go`

**Interfaces:**
- Consumes: `layout.Line` (Task 3); `money` (Task 1); `provider` (Task 4); `parseDate` (Task 5).
- Produces:
  - `func parseExtratoLines(lines []layout.Line) (*provider.Statement, error)`
  - `func classifyExtrato(desc string, printed int64) provider.TxKind`
  - `func reconcileExtrato(balances, sums map[time.Time]int64) error`

Regras (spec §7.3): linha `^(dd/mm/aaaa)\s+(.+?)\s+(valor)$`; `SALDO DO DIA` = saldo da data; `AmountCents = −valorImpresso`; chave = `NormalizeKey` sem a data colada no fim (`\d{2}/\d{2}$`); `PersonTransfer` = saída que começa com `PIX TRANSF`; conferência: `saldo[d] = saldo[anterior] + Σ valores impressos em d` para datas consecutivas com saldo; toda data com lançamento precisa de saldo; lançamento na primeira data com saldo não pode ser conferido → `LayoutError`.

- [ ] **Step 1: Escrever o teste que falha** — `backend/internal/provider/itau/extrato_test.go`

```go
package itau

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"jgfinance/internal/provider"
)

// Invented statement (no real data), newest first like the real one. Arithmetic:
// 500,00 → 02/03: -200,00 -150,50 +1.000,00 = 1.149,50 → 03/03: -800,00 +0,33 +50,00 = 399,83
// → 04/03: -20,00 +20,00 -35,00 -35,00 = 329,83 → 05/03: no transactions = 329,83.
var extratoFixture = []string{
	"saldo em conta",
	"extrato conta / lançamentos",
	"período de visualização: 01/03/2026 até 05/03/2026",
	"05/03/2026 SALDO DO DIA 329,83",
	"04/03/2026 PIX QRS PADARIA BO04/03 -20,00",
	"04/03/2026 DEV PIX PADARIA BO04/03 20,00",
	"04/03/2026 PIX TRANSF MARIA S04/03 -35,00",
	"04/03/2026 PIX TRANSF MARIA S04/03 -35,00",
	"04/03/2026 SALDO DO DIA 329,83",
	"03/03/2026 FATURA PAGA CARTAO X -800,00",
	"03/03/2026 JUROS POUP AUT 0001 0,33",
	"03/03/2026 PIX TRANSF Joâo P03/03 50,00",
	"03/03/2026 SALDO DO DIA 399,83",
	"02/03/2026 PIX TRANSF Fulána02/03 -200,00",
	"02/03/2026 PIX QRS LUZ ENERGIA02/03 -150,50",
	"02/03/2026 RESGATE CDB DI 1.000,00",
	"02/03/2026 SALDO DO DIA 1.149,50",
	"28/02/2026 SALDO DO DIA 500,00",
	"FULANO DE TAL",
	"agência: 1234",
	"conta: 00000-1",
	"Aviso! Os saldos acima são baseados nas informações disponíveis. www.itau.com.br",
}

func TestParseExtratoLines(t *testing.T) {
	st, err := parseExtratoLines(toLines(extratoFixture))
	if err != nil {
		t.Fatalf("parseExtratoLines: %v", err)
	}
	d4, d3, d2 := day(2026, 3, 4), day(2026, 3, 3), day(2026, 3, 2)
	want := &provider.Statement{
		Kind:        provider.BankStatement,
		Account:     provider.AccountRef{Provider: "itau", Kind: provider.Checking, ExternalID: "1234/00000-1", DefaultName: "Itaú conta 00000-1"},
		PeriodStart: day(2026, 3, 1),
		PeriodEnd:   day(2026, 3, 5),
		Transactions: []provider.Transaction{
			{Seq: 1, Date: d4, Description: "PIX QRS PADARIA BO04/03", Key: "PIX QRS PADARIA BO", AmountCents: 2000, Kind: provider.Expense},
			{Seq: 2, Date: d4, Description: "DEV PIX PADARIA BO04/03", Key: "DEV PIX PADARIA BO", AmountCents: -2000, Kind: provider.Refund},
			{Seq: 3, Date: d4, Description: "PIX TRANSF MARIA S04/03", Key: "PIX TRANSF MARIA S", AmountCents: 3500, Kind: provider.Expense, PersonTransfer: true},
			{Seq: 4, Date: d4, Description: "PIX TRANSF MARIA S04/03", Key: "PIX TRANSF MARIA S", AmountCents: 3500, Kind: provider.Expense, PersonTransfer: true},
			{Seq: 5, Date: d3, Description: "FATURA PAGA CARTAO X", Key: "FATURA PAGA CARTAO X", AmountCents: 80000, Kind: provider.BillPayment},
			{Seq: 6, Date: d3, Description: "JUROS POUP AUT 0001", Key: "JUROS POUP AUT 0001", AmountCents: -33, Kind: provider.Income},
			{Seq: 7, Date: d3, Description: "PIX TRANSF Joâo P03/03", Key: "PIX TRANSF JOAO P", AmountCents: -5000, Kind: provider.Income},
			{Seq: 8, Date: d2, Description: "PIX TRANSF Fulána02/03", Key: "PIX TRANSF FULANA", AmountCents: 20000, Kind: provider.Expense, PersonTransfer: true},
			{Seq: 9, Date: d2, Description: "PIX QRS LUZ ENERGIA02/03", Key: "PIX QRS LUZ ENERGIA", AmountCents: 15050, Kind: provider.Expense},
			{Seq: 10, Date: d2, Description: "RESGATE CDB DI", Key: "RESGATE CDB DI", AmountCents: -100000, Kind: provider.Investment},
		},
	}
	if diff := cmp.Diff(want, st); diff != "" {
		t.Errorf("statement mismatch (-want +got):\n%s", diff)
	}
}

func TestParseExtratoReconciliationFails(t *testing.T) {
	lines := replaceLine(t, extratoFixture, "02/03/2026 PIX QRS LUZ ENERGIA02/03 -150,50", "02/03/2026 PIX QRS LUZ ENERGIA02/03 -150,00")
	_, err := parseExtratoLines(toLines(lines))
	wantReconciliation(t, err, "saldo do dia 02/03/2026", 115000, 114950)
}

func TestParseExtratoLayoutErrors(t *testing.T) {
	cases := map[string][]string{
		"transaction date without daily balance":  replaceLine(t, extratoFixture, "04/03/2026 SALDO DO DIA 329,83", ""),
		"transaction on the opening balance date": append(append([]string{}, extratoFixture...), "28/02/2026 PIX TRANSF X28/02 -10,00"),
		"missing account":                         replaceLine(t, extratoFixture, "conta: 00000-1", ""),
	}
	for name, lines := range cases {
		_, err := parseExtratoLines(toLines(lines))
		var le *provider.LayoutError
		if !errors.As(err, &le) {
			t.Errorf("%s: err = %v, want *provider.LayoutError", name, err)
		}
	}
}

func TestClassifyExtrato(t *testing.T) {
	cases := []struct {
		desc    string
		printed int64
		want    provider.TxKind
	}{
		{"FATURA PAGA Itau Uniclas", -100, provider.BillPayment},
		{"APLICACAO CDB DI", -100, provider.Investment},
		{"RESGATE CDB DI", 100, provider.Investment},
		{"JUROS POUP AUT 0005000", 33, provider.Income},
		{"REMUNER BASICA POUP AUT 0001693", 11, provider.Income},
		{"REND PAGO APLIC AUT", 50, provider.Income},
		{"DEV PIX LOJA X02/09", 4200, provider.Refund},
		{"ESTORNO TARIFA", 500, provider.Refund},
		{"PAG BOLETO ENERGIA", -9000, provider.Expense},
		{"TED RECEBIDA", 9000, provider.Income},
	}
	for _, c := range cases {
		if got := classifyExtrato(c.desc, c.printed); got != c.want {
			t.Errorf("classifyExtrato(%q, %d) = %s, want %s", c.desc, c.printed, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./internal/provider/itau/... -run 'Extrato'`
Expected: FAIL com `undefined: parseExtratoLines` / `undefined: classifyExtrato`.

- [ ] **Step 3: Implementar** — `backend/internal/provider/itau/extrato.go`

```go
package itau

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"jgfinance/internal/layout"
	"jgfinance/internal/money"
	"jgfinance/internal/provider"
)

const balanceLabel = "SALDO DO DIA"

var (
	extratoRowRe = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4})\s+(.+?)\s+(` + money.Pattern + `)$`)
	periodRe     = regexp.MustCompile(`período de visualização:\s*(\d{2}/\d{2}/\d{4})\s*até\s*(\d{2}/\d{2}/\d{4})`)
	agencyRe     = regexp.MustCompile(`agência:\s*(\d+)`)
	accountNoRe  = regexp.MustCompile(`conta:\s*(\d[\d-]*)`)
	gluedDateRe  = regexp.MustCompile(`\d{2}/\d{2}$`)
)

// parseExtratoLines turns the lines of an Itaú checking account statement into a Statement,
// after reconciling every day against the printed "SALDO DO DIA".
func parseExtratoLines(lines []layout.Line) (*provider.Statement, error) {
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l.Text)
		sb.WriteByte('\n')
	}
	text := sb.String()

	agency, account := agencyRe.FindStringSubmatch(text), accountNoRe.FindStringSubmatch(text)
	if agency == nil || account == nil {
		return nil, &provider.LayoutError{Msg: "agência/conta não encontradas no extrato"}
	}

	balances := map[time.Time]int64{}
	sums := map[time.Time]int64{}
	var txs []provider.Transaction
	for _, l := range lines {
		m := extratoRowRe.FindStringSubmatch(l.Text)
		if m == nil {
			continue
		}
		date, err := parseDate(m[1])
		if err != nil {
			return nil, err
		}
		printed, err := money.ParseBRL(m[3])
		if err != nil {
			return nil, err
		}
		desc := m[2]
		if provider.NormalizeKey(desc) == balanceLabel {
			balances[date] = printed
			continue
		}
		sums[date] += printed
		txs = append(txs, provider.Transaction{
			Date:           date,
			Description:    desc,
			Key:            provider.NormalizeKey(gluedDateRe.ReplaceAllString(desc, "")),
			AmountCents:    -printed,
			Kind:           classifyExtrato(desc, printed),
			PersonTransfer: printed < 0 && strings.HasPrefix(provider.NormalizeKey(desc), "PIX TRANSF"),
		})
	}
	if err := reconcileExtrato(balances, sums); err != nil {
		return nil, err
	}
	for i := range txs {
		txs[i].Seq = i + 1
	}

	st := &provider.Statement{
		Kind: provider.BankStatement,
		Account: provider.AccountRef{
			Provider: "itau", Kind: provider.Checking,
			ExternalID: agency[1] + "/" + account[1], DefaultName: "Itaú conta " + account[1],
		},
		Transactions: txs,
	}
	if m := periodRe.FindStringSubmatch(text); m != nil {
		start, err := parseDate(m[1])
		if err != nil {
			return nil, err
		}
		end, err := parseDate(m[2])
		if err != nil {
			return nil, err
		}
		st.PeriodStart, st.PeriodEnd = start, end
	} else {
		dates := sortedDates(balances)
		st.PeriodStart, st.PeriodEnd = dates[0], dates[len(dates)-1]
	}
	return st, nil
}

// classifyExtrato decides the kind of a statement line from its description and printed sign.
func classifyExtrato(desc string, printed int64) provider.TxKind {
	k := provider.NormalizeKey(desc)
	hasPrefix := func(prefixes ...string) bool {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				return true
			}
		}
		return false
	}
	switch {
	case hasPrefix("FATURA PAGA"):
		return provider.BillPayment
	case hasPrefix("APLICACAO", "RESGATE"):
		return provider.Investment
	case hasPrefix("JUROS POUP", "REMUNER BASICA POUP", "REND PAGO", "RENDIMENTO"):
		return provider.Income
	case hasPrefix("DEV PIX") || strings.Contains(k, "ESTORNO"):
		return provider.Refund
	case printed < 0:
		return provider.Expense
	default:
		return provider.Income
	}
}

// reconcileExtrato checks balance[d] = balance[previous] + sum of printed amounts on d.
func reconcileExtrato(balances, sums map[time.Time]int64) error {
	if len(balances) == 0 {
		return &provider.LayoutError{Msg: "nenhuma linha SALDO DO DIA no extrato"}
	}
	for d := range sums {
		if _, ok := balances[d]; !ok {
			return &provider.LayoutError{Msg: fmt.Sprintf("lançamentos em %s sem linha SALDO DO DIA", d.Format("02/01/2006"))}
		}
	}
	dates := sortedDates(balances)
	if _, ok := sums[dates[0]]; ok {
		return &provider.LayoutError{Msg: fmt.Sprintf("lançamentos em %s não podem ser conferidos (sem saldo anterior)", dates[0].Format("02/01/2006"))}
	}
	for i := 1; i < len(dates); i++ {
		prev, d := dates[i-1], dates[i]
		if got := balances[prev] + sums[d]; got != balances[d] {
			return &provider.ReconciliationError{Rule: "saldo do dia " + d.Format("02/01/2006"), Computed: got, Printed: balances[d]}
		}
	}
	return nil
}

func sortedDates(m map[time.Time]int64) []time.Time {
	dates := make([]time.Time, 0, len(m))
	for d := range m {
		dates = append(dates, d)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	return dates
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./internal/provider/itau/...`
Expected: `ok  	jgfinance/internal/provider/itau`

- [ ] **Step 5: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && cd ..
git add backend/internal/provider/itau
git commit -m "feat(itau): extrato de conta com conferência pelo saldo de cada dia

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Provider Itaú completo, `config` e regressão com PDFs reais

**Files:**
- Create: `backend/internal/provider/itau/itau.go`
- Test: `backend/internal/provider/itau/itau_test.go`
- Test: `backend/internal/provider/itau/samples_test.go`
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`
- Modify: `Makefile` (alvo `test-samples`)
- Local (fora do git): `samples/itau/` com os 3 PDFs reais

**Interfaces:**
- Consumes: `pdftext` (Task 2), `layout` (Task 3), `provider` (Task 4), `parseFaturaLines` (Tasks 5–6), `parseExtratoLines` (Task 7), `money.FormatBRL`.
- Produces:
  - `type itau.Itau struct{}`; `func itau.New() *Itau`; métodos `ID() string` (= `"itau"`), `Detect(*pdftext.Document) (provider.DocKind, bool)`, `Parse(*pdftext.Document, provider.DocKind) (*provider.Statement, error)`
  - `func faturaSplit(doc *pdftext.Document) float64` (interno)
  - `type config.Config struct { MySQLDSN, MySQLTestDSN, OpenAIKey, OpenAIModel string; PDFPasswords []string; HTTPAddr, DataDir, SamplesDir, BaseDir string }`
  - `func config.Load(envFiles ...string) Config` — lê os `.env` existentes (o primeiro define `BaseDir`), e variáveis de ambiente **não vazias** têm precedência; não altera o ambiente do processo.
  - `func config.FromEnviron(environ []string, baseDir string) Config` — defaults da spec §5; `PDFPasswords` = valores não vazios de toda variável `*_PDF_PASSWORD`, ordenados pelo nome da variável; `DataDir`/`SamplesDir` relativos são resolvidos a partir de `baseDir`.

- [ ] **Step 1: Adicionar dependência**

```bash
cd backend && go get github.com/joho/godotenv@latest
```

- [ ] **Step 2: Escrever os testes de `config` que falham** — `backend/internal/config/config_test.go`

```go
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFromEnvironDefaults(t *testing.T) {
	got := FromEnviron(nil, "/repo")
	want := Config{
		MySQLDSN:     "finance:finance@tcp(127.0.0.1:3307)/finance?parseTime=true&loc=UTC",
		MySQLTestDSN: "finance:finance@tcp(127.0.0.1:3307)/finance_test?parseTime=true&loc=UTC",
		OpenAIModel:  "gpt-5-mini",
		HTTPAddr:     "127.0.0.1:8080",
		DataDir:      "/repo/data",
		BaseDir:      "/repo",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestFromEnvironPDFPasswordsSortedByName(t *testing.T) {
	got := FromEnviron([]string{"NUBANK_PDF_PASSWORD=n1", "ITAU_PDF_PASSWORD=i1", "EMPTY_PDF_PASSWORD=", "OTHER=x"}, "/repo")
	if want := []string{"i1", "n1"}; !reflect.DeepEqual(got.PDFPasswords, want) {
		t.Errorf("PDFPasswords = %v, want %v", got.PDFPasswords, want)
	}
}

func TestFromEnvironResolvesRelativeDirs(t *testing.T) {
	got := FromEnviron([]string{"DATA_DIR=./dados", "SAMPLES_DIR=samples", "OPENAI_MODEL=gpt-x"}, "/repo")
	if got.DataDir != "/repo/dados" || got.SamplesDir != "/repo/samples" || got.OpenAIModel != "gpt-x" {
		t.Errorf("got %+v", got)
	}
	if abs := FromEnviron([]string{"DATA_DIR=/tmp/x"}, "/repo"); abs.DataDir != "/tmp/x" {
		t.Errorf("absolute DATA_DIR changed: %q", abs.DataDir)
	}
}

func TestLoadReadsDotEnvAndEnvironmentWins(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	if err := os.WriteFile(env, []byte("OPENAI_MODEL=do-arquivo\nHTTP_ADDR=127.0.0.1:9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_MODEL", "do-ambiente")
	t.Setenv("HTTP_ADDR", "") // empty environment values do not override the file
	got := Load(filepath.Join(dir, "nao-existe.env"), env)
	if got.OpenAIModel != "do-ambiente" || got.HTTPAddr != "127.0.0.1:9999" || got.BaseDir != dir {
		t.Errorf("got %+v", got)
	}
}
```

- [ ] **Step 3: Rodar e ver falhar**

Run: `cd backend && go test ./internal/config/...`
Expected: FAIL com `undefined: FromEnviron` / `undefined: Load` / `undefined: Config`.

- [ ] **Step 4: Implementar** — `backend/internal/config/config.go`

```go
// Package config loads settings from .env files and the process environment (spec §5).
package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds every setting of the application.
type Config struct {
	MySQLDSN     string
	MySQLTestDSN string
	OpenAIKey    string
	OpenAIModel  string
	PDFPasswords []string // values of every non-empty *_PDF_PASSWORD, ordered by variable name
	HTTPAddr     string
	DataDir      string
	SamplesDir   string
	BaseDir      string // directory relative paths are resolved from
}

// Load reads the given .env files that exist (earlier files win; the first one found sets
// BaseDir, otherwise the working directory is used) and overlays non-empty environment
// variables. It never modifies the process environment.
func Load(envFiles ...string) Config {
	merged := map[string]string{}
	base := ""
	for _, f := range envFiles {
		vals, err := godotenv.Read(f)
		if err != nil {
			continue
		}
		if base == "" {
			if abs, err := filepath.Abs(filepath.Dir(f)); err == nil {
				base = abs
			}
		}
		for k, v := range vals {
			if _, ok := merged[k]; !ok {
				merged[k] = v
			}
		}
	}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && v != "" {
			merged[k] = v
		}
	}
	if base == "" {
		base, _ = os.Getwd()
	}
	environ := make([]string, 0, len(merged))
	for k, v := range merged {
		environ = append(environ, k+"="+v)
	}
	return FromEnviron(environ, base)
}

// FromEnviron builds a Config from KEY=VALUE pairs, applying the defaults of the spec.
func FromEnviron(environ []string, baseDir string) Config {
	env := map[string]string{}
	var pwKeys []string
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		env[k] = v
		if strings.HasSuffix(k, "_PDF_PASSWORD") && v != "" {
			pwKeys = append(pwKeys, k)
		}
	}
	sort.Strings(pwKeys)
	get := func(k, def string) string {
		if v := env[k]; v != "" {
			return v
		}
		return def
	}
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(baseDir, p)
	}
	c := Config{
		MySQLDSN:     get("MYSQL_DSN", "finance:finance@tcp(127.0.0.1:3307)/finance?parseTime=true&loc=UTC"),
		MySQLTestDSN: get("MYSQL_TEST_DSN", "finance:finance@tcp(127.0.0.1:3307)/finance_test?parseTime=true&loc=UTC"),
		OpenAIKey:    env["OPENAI_API_KEY"],
		OpenAIModel:  get("OPENAI_MODEL", "gpt-5-mini"),
		HTTPAddr:     get("HTTP_ADDR", "127.0.0.1:8080"),
		DataDir:      resolve(get("DATA_DIR", "data")),
		SamplesDir:   resolve(env["SAMPLES_DIR"]),
		BaseDir:      baseDir,
	}
	for _, k := range pwKeys {
		c.PDFPasswords = append(c.PDFPasswords, env[k])
	}
	return c
}
```

Run: `cd backend && go mod tidy && go test ./internal/config/...` → Expected: `ok  	jgfinance/internal/config`

- [ ] **Step 5: Escrever o teste do provider que falha** — `backend/internal/provider/itau/itau_test.go`

```go
package itau

import (
	"math"
	"testing"

	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
)

// page lays out each column's lines as one run per line (x = 40 or 352), 12pt apart.
func page(cols ...[]string) pdftext.Page {
	p := pdftext.Page{Width: 595.28, Height: 841.89}
	for c, lines := range cols {
		x := 40.0
		if c == 1 {
			x = 352
		}
		for i, text := range lines {
			y := 60 + 12*float64(i)
			p.Runs = append(p.Runs, pdftext.Run{Text: text, X0: x, Y0: y - 4, X1: x + 4.5*float64(len(text)), Y1: y + 4})
		}
	}
	return p
}

func TestDetect(t *testing.T) {
	fatura := &pdftext.Document{Pages: []pdftext.Page{page([]string{"Resumo da fatura em R$", "Total desta fatura 10,00", "Banco Itaú S.A."})}}
	extrato := &pdftext.Document{Pages: []pdftext.Page{page([]string{"extrato conta / lançamentos", "02/03/2026 SALDO DO DIA 1,00", "www.itau.com.br"})}}
	other := &pdftext.Document{Pages: []pdftext.Page{page([]string{"Nubank fatura"})}}
	p := New()
	if k, ok := p.Detect(fatura); !ok || k != provider.CardStatement {
		t.Errorf("fatura: %v %v", k, ok)
	}
	if k, ok := p.Detect(extrato); !ok || k != provider.BankStatement {
		t.Errorf("extrato: %v %v", k, ok)
	}
	if _, ok := p.Detect(other); ok {
		t.Error("other document must not be detected")
	}
}

func TestFaturaSplit(t *testing.T) {
	two := &pdftext.Document{Pages: []pdftext.Page{page([]string{"capa"}), page([]string{"DATA VALOR EM R$"}, []string{"DATA ESTABELECIMENTO VALOR EM R$"})}}
	if got := faturaSplit(two); got != 337 {
		t.Errorf("split = %v, want 337", got)
	}
	one := &pdftext.Document{Pages: []pdftext.Page{page([]string{"capa"}), page([]string{"DATA ESTABELECIMENTO VALOR EM R$"})}}
	if got := faturaSplit(one); math.Abs(got-595.28*0.565) > 0.01 {
		t.Errorf("fallback split = %v", got)
	}
}

func TestParseCardStatementFromTwoColumnDocument(t *testing.T) {
	header := page([]string{
		"Platinum",
		"Resumo da fatura em R$",
		"S Saldo financiado 0,00",
		"L Lançamentos atuais 30,00",
		"Postagem: 24/02/2026",
		"Vencimento: 03/03/2026 = Total desta fatura 30,00",
		"Cartão 4111.XXXX.XXXX.1234",
		"Banco Itaú S.A.",
	})
	body := page(
		[]string{
			"Pagamentos efetuados",
			"DATA VALOR EM R$",
			"01/02 Pagamento via conta -100,00",
			"P Total dos pagamentos -100,00",
			"Lançamentos: compras e saques",
			"FULANO DE TAL",
			"DATA ESTABELECIMENTO VALOR EM R$",
			"05/02 LOJA A 10,00",
			"vestuário SAO PAULO",
		},
		[]string{
			"Lançamentos: compras e saques",
			"DATA ESTABELECIMENTO VALOR EM R$",
			"06/02 LOJA B 20,00",
			"lazer SAO PAULO",
			"Lançamentos no cartão 30,00",
			"L Total dos lançamentos atuais 30,00",
		},
	)
	doc := &pdftext.Document{Pages: []pdftext.Page{header, body}}
	kind, ok := New().Detect(doc)
	if !ok || kind != provider.CardStatement {
		t.Fatalf("Detect = %v, %v", kind, ok)
	}
	st, err := New().Parse(doc, kind)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(st.Transactions) != 3 || st.Transactions[1].Description != "LOJA A" || st.Transactions[2].Description != "LOJA B" ||
		st.Transactions[2].Holder != "FULANO DE TAL" || st.Transactions[2].BankCategory != "lazer" {
		t.Errorf("transactions = %+v", st.Transactions)
	}
}
```

- [ ] **Step 6: Rodar e ver falhar**

Run: `cd backend && go test ./internal/provider/itau/... -run 'Detect|Split|TwoColumn'`
Expected: FAIL com `undefined: New` / `undefined: faturaSplit`.

- [ ] **Step 7: Implementar** — `backend/internal/provider/itau/itau.go`

```go
// Package itau reads Itaú credit card invoices (fatura) and checking account statements (extrato).
package itau

import (
	"fmt"
	"sort"
	"strings"

	"jgfinance/internal/layout"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
)

// Itau implements provider.Provider.
type Itau struct{}

var _ provider.Provider = (*Itau)(nil)

// New returns the Itaú provider.
func New() *Itau { return &Itau{} }

// ID implements provider.Provider.
func (*Itau) ID() string { return "itau" }

// Detect implements provider.Provider (spec §7.1).
func (*Itau) Detect(doc *pdftext.Document) (provider.DocKind, bool) {
	text := documentText(doc)
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(text, "Resumo da fatura em R$") && strings.Contains(text, "Total desta fatura") &&
		(strings.Contains(lower, "itau unibanco") || strings.Contains(lower, "banco itaú")):
		return provider.CardStatement, true
	case strings.Contains(lower, "extrato conta") && strings.Contains(text, "SALDO DO DIA") &&
		strings.Contains(lower, "itau.com.br"):
		return provider.BankStatement, true
	}
	return "", false
}

// Parse implements provider.Provider. The returned Statement is already reconciled.
func (*Itau) Parse(doc *pdftext.Document, kind provider.DocKind) (*provider.Statement, error) {
	switch kind {
	case provider.CardStatement:
		if len(doc.Pages) < 2 {
			return nil, &provider.LayoutError{Msg: "fatura com menos de 2 páginas"}
		}
		split := faturaSplit(doc)
		var body []layout.Line
		for _, p := range doc.Pages[1:] {
			body = append(body, layout.Lines(p, []float64{split})...)
		}
		return parseFaturaLines(layout.PageText(doc.Pages[0]), body)
	case provider.BankStatement:
		var lines []layout.Line
		for _, p := range doc.Pages {
			lines = append(lines, layout.Lines(p, nil)...)
		}
		return parseExtratoLines(lines)
	}
	return nil, fmt.Errorf("itau: tipo de documento não suportado: %s", kind)
}

func documentText(doc *pdftext.Document) string {
	var b strings.Builder
	for _, p := range doc.Pages {
		b.WriteString(layout.PageText(p))
		b.WriteByte('\n')
	}
	return b.String()
}

// faturaSplit finds the X that separates the two body columns: the right-most "DATA" header
// on the first body page with two header clusters (> 100pt apart), minus a 15pt margin.
// Fallback: 56.5% of the page width (the position measured on real invoices).
func faturaSplit(doc *pdftext.Document) float64 {
	for _, p := range doc.Pages[1:] {
		var xs []float64
		for _, r := range p.Runs {
			if r.Text == "DATA" || strings.HasPrefix(r.Text, "DATA ") {
				xs = append(xs, r.X0)
			}
		}
		if len(xs) < 2 {
			continue
		}
		sort.Float64s(xs)
		if xs[len(xs)-1]-xs[0] > 100 {
			return xs[len(xs)-1] - 15
		}
	}
	return doc.Pages[0].Width * 0.565
}
```

- [ ] **Step 8: Rodar e ver passar**

Run: `cd backend && go test ./internal/provider/itau/...`
Expected: `ok  	jgfinance/internal/provider/itau`

- [ ] **Step 9: Teste de regressão com PDFs reais** — `backend/internal/provider/itau/samples_test.go`

```go
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
```

- [ ] **Step 10: Copiar os PDFs reais para `samples/` (fora do git) e adicionar o alvo no `Makefile`**

```bash
mkdir -p samples/itau && cp ~/Downloads/Fatura_VISA_*_01-10-2026.pdf ~/Downloads/itau_extrato_082026.pdf samples/itau/
git status --short samples   # Expected: nada (samples/ é ignorado)
```

Acrescentar ao `Makefile` (e incluir `test-samples` no `.PHONY`):

```make
# Regressão com PDFs reais em samples/ (fora do git). Senha via ITAU_PDF_PASSWORD no ambiente ou no .env.
test-samples:
	cd backend && SAMPLES_DIR=$(CURDIR)/samples go test ./internal/provider/itau -run TestSamples -count=1 -v
```

Run: `make test-samples`
Expected: o extrato **PASS** (`Itaú conta 030595-4 (bank_statement): 38 transações`); as duas faturas **SKIP** ("PDF com senha") se `ITAU_PDF_PASSWORD` não estiver definida. Com a senha no ambiente ou no `.env`, as três passam: faturas com `total R$ 4.052,86` e `total R$ 6.637,45`.

- [ ] **Step 11: Commit**

```bash
cd backend && gofmt -w internal && go vet ./... && go test ./... && cd ..
git add Makefile backend/go.mod backend/go.sum backend/internal/config backend/internal/provider/itau
git commit -m "feat(itau): provider completo, config e regressão com PDFs reais

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: CLI `inspect`, `.env.example` e README

**Files:**
- Create: `backend/cmd/inspect/main.go`
- Test: `backend/cmd/inspect/main_test.go`
- Create: `.env.example`
- Create: `README.md`
- Modify: `Makefile` (alvo `inspect`)

**Interfaces:**
- Consumes: `config.Load` (Task 8), `provider.NewRegistry`, `itau.New` (Task 8), `pdftext.Open` (Task 2), `money.FormatBRL` (Task 1).
- Produces:
  - `func parseFile(reg *provider.Registry, passwords []string, path string) (*provider.Statement, error)`
  - `func summary(name string, st *provider.Statement, verbose bool) string`
  - Uso: `cd backend && go run ./cmd/inspect [-v] arquivo.pdf...` (código de saída 1 se algum arquivo falhar).

- [ ] **Step 1: Escrever o teste que falha** — `backend/cmd/inspect/main_test.go`

```go
package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"jgfinance/internal/provider"
	"jgfinance/internal/provider/itau"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func cardStatement() *provider.Statement {
	return &provider.Statement{
		Kind:        provider.CardStatement,
		Account:     provider.AccountRef{DefaultName: "Itaú Platinum •••• 1234"},
		ClosingDate: day(2026, 2, 24),
		DueDate:     day(2026, 3, 3),
		TotalCents:  116489,
		Transactions: []provider.Transaction{
			{Date: day(2026, 2, 10), Description: "MERCADO CENTRAL", Key: "MERCADO CENTRAL", Kind: provider.Expense, AmountCents: 15000},
			{Date: day(2026, 2, 12), Description: "aliexpressLOJA", Key: "ALIEXPRESSLOJA", Kind: provider.Refund, AmountCents: -4000},
			{Date: day(2026, 2, 1), Description: "Pagamento via conta", Key: "PAGAMENTO VIA CONTA", Kind: provider.BillPayment, AmountCents: -100000},
		},
		Upcoming: []provider.UpcomingInstallment{{AmountCents: 15000}},
		Warnings: []string{"projeção difere"},
	}
}

func TestSummary(t *testing.T) {
	out := summary("fatura.pdf", cardStatement(), false)
	for _, want := range []string{
		"✓ fatura.pdf — Itaú Platinum •••• 1234",
		"fechamento 24/02/2026",
		"vencimento 03/03/2026",
		"R$ 1.164,89",
		"gastos",
		"R$ 150,00",
		"estornos",
		"-R$ 40,00",
		"parcelas na próxima fatura: 1 · R$ 150,00",
		"⚠ projeção difere",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "MERCADO CENTRAL") {
		t.Error("non-verbose summary must not list transactions")
	}
	if !strings.Contains(summary("fatura.pdf", cardStatement(), true), "MERCADO CENTRAL") {
		t.Error("verbose summary must list transactions")
	}
}

func TestParseFileUnknownDocument(t *testing.T) {
	_, err := parseFile(provider.NewRegistry(itau.New()), nil, "../../internal/pdftext/testdata/plain.pdf")
	if !errors.Is(err, provider.ErrUnknownDocument) {
		t.Errorf("err = %v, want ErrUnknownDocument", err)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `cd backend && go test ./cmd/inspect/...`
Expected: FAIL com `undefined: summary` / `undefined: parseFile`.

- [ ] **Step 3: Implementar** — `backend/cmd/inspect/main.go`

```go
// Command inspect parses bank PDFs and prints what would be imported, without a database.
//
//	cd backend && go run ./cmd/inspect [-v] arquivo.pdf...
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"jgfinance/internal/config"
	"jgfinance/internal/money"
	"jgfinance/internal/pdftext"
	"jgfinance/internal/provider"
	"jgfinance/internal/provider/itau"
)

func main() {
	verbose := flag.Bool("v", false, "lista todas as transações")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "uso: go run ./cmd/inspect [-v] arquivo.pdf...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	cfg := config.Load("../.env", ".env")
	reg := provider.NewRegistry(itau.New())
	failed := false
	for _, path := range flag.Args() {
		st, err := parseFile(reg, cfg.PDFPasswords, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ %s: %v\n", filepath.Base(path), err)
			failed = true
			continue
		}
		fmt.Print(summary(filepath.Base(path), st, *verbose))
	}
	if failed {
		os.Exit(1)
	}
}

// parseFile opens, detects and parses (with reconciliation) one PDF.
func parseFile(reg *provider.Registry, passwords []string, path string) (*provider.Statement, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := pdftext.Open(data, passwords)
	if err != nil {
		return nil, err
	}
	p, kind, err := reg.Detect(doc)
	if err != nil {
		return nil, err
	}
	return p.Parse(doc, kind)
}

var kindLabels = []struct {
	kind  provider.TxKind
	label string
}{
	{provider.Expense, "gastos"},
	{provider.Refund, "estornos"},
	{provider.BillPayment, "pagam. fatura"},
	{provider.Investment, "investimentos"},
	{provider.Income, "entradas"},
}

// summary renders a human-readable report of a parsed statement.
func summary(name string, st *provider.Statement, verbose bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "✓ %s — %s\n", name, st.Account.DefaultName)
	switch st.Kind {
	case provider.CardStatement:
		fmt.Fprintf(&b, "  fatura: fechamento %s · vencimento %s · total %s (conferido)\n",
			date(st.ClosingDate), date(st.DueDate), money.FormatBRL(st.TotalCents))
	case provider.BankStatement:
		fmt.Fprintf(&b, "  extrato: %s a %s (saldos diários conferidos)\n", date(st.PeriodStart), date(st.PeriodEnd))
	}
	counts := map[provider.TxKind]int{}
	sums := map[provider.TxKind]int64{}
	for _, tx := range st.Transactions {
		counts[tx.Kind]++
		sums[tx.Kind] += tx.AmountCents
	}
	for _, k := range kindLabels {
		if counts[k.kind] > 0 {
			fmt.Fprintf(&b, "  %-14s %3d  %s\n", k.label, counts[k.kind], money.FormatBRL(sums[k.kind]))
		}
	}
	if len(st.Upcoming) > 0 {
		var next int64
		for _, u := range st.Upcoming {
			next += u.AmountCents
		}
		fmt.Fprintf(&b, "  parcelas na próxima fatura: %d · %s\n", len(st.Upcoming), money.FormatBRL(next))
	}
	for _, w := range st.Warnings {
		fmt.Fprintf(&b, "  ⚠ %s\n", w)
	}
	if verbose {
		for _, tx := range st.Transactions {
			fmt.Fprintf(&b, "    %s  %-12s %14s  %-34s %s\n",
				date(tx.Date), tx.Kind, money.FormatBRL(tx.AmountCents), tx.Description, tx.Key)
		}
	}
	return b.String()
}

func date(t time.Time) string { return t.Format("02/01/2006") }
```

- [ ] **Step 4: Rodar e ver passar**

Run: `cd backend && go test ./cmd/inspect/...`
Expected: `ok  	jgfinance/cmd/inspect`

- [ ] **Step 5: `.env.example`, `README.md` e alvo `inspect`**

`.env.example`:

```
# Copie para .env (que fica fora do git) e preencha.
MYSQL_DSN=finance:finance@tcp(127.0.0.1:3307)/finance?parseTime=true&loc=UTC
MYSQL_TEST_DSN=finance:finance@tcp(127.0.0.1:3307)/finance_test?parseTime=true&loc=UTC
OPENAI_API_KEY=
OPENAI_MODEL=gpt-5-mini
# Senha dos PDFs de fatura do Itaú
ITAU_PDF_PASSWORD=
HTTP_ADDR=127.0.0.1:8080
# Caminhos relativos são resolvidos a partir da pasta deste arquivo
DATA_DIR=data
SAMPLES_DIR=samples
```

`README.md`:

````markdown
# JG Finance Tracker

Organiza meus gastos a partir das faturas de cartão e dos extratos da conta (Itaú primeiro):
extrai as transações dos PDFs, confere cada documento centavo a centavo contra os totais
impressos, categoriza com ajuda de IA e mostra dashboards por categoria, recorrentes e o
que falta pagar.

Design: `docs/superpowers/specs/2026-09-26-finance-tracker-design.md`.

## Requisitos

- Go 1.26+ (se o Go instalado for mais antigo, ele baixa o 1.26 sozinho)
- Python 3 + PyMuPDF apenas para regenerar os PDFs sintéticos de teste

## Configuração

```bash
cp .env.example .env   # preencha ITAU_PDF_PASSWORD
```

## Conferir PDFs sem banco de dados

Coloque os PDFs em `samples/itau/` (pasta fora do git) e rode:

```bash
cd backend && go run ./cmd/inspect -v ../samples/itau/*.pdf
```

Cada arquivo mostra conta, datas, totais por tipo e, com `-v`, todas as transações. Se a
conferência falhar, a mensagem mostra a regra, o valor calculado, o impresso e a diferença.

## Testes

```bash
make test           # unitários
make test-samples   # regressão com os PDFs reais de samples/
```
````

Acrescentar ao `Makefile` (e incluir `inspect` no `.PHONY`):

```make
# Ex.: make inspect ARGS="-v ../samples/itau/*.pdf"
inspect:
	cd backend && go run ./cmd/inspect $(ARGS)
```

Run: `make inspect ARGS="../samples/itau/itau_extrato_082026.pdf"`
Expected: `✓ itau_extrato_082026.pdf — Itaú conta 030595-4`, período `27/08/2026 a 26/09/2026` e linhas `gastos`, `pagam. fatura`, `investimentos`, `entradas`, `estornos`.

- [ ] **Step 6: Commit**

```bash
cd backend && gofmt -w cmd internal && go vet ./... && go test ./... && cd ..
git add .env.example README.md Makefile backend/cmd
git commit -m "feat(inspect): CLI para conferir PDFs reais antes de importar

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
