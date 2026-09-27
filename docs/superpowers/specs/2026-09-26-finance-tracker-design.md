# JG Finance Tracker — Design v1

- **Data:** 2026-09-26
- **Status:** design aprovado em conversa (3 partes); implementação autorizada
- **Autor do produto:** Jonathan (uso pessoal)

> **Nota (2026-09-26):** o recurso "falta pagar" (faturas em aberto e projeção de parcelas futuras) foi removido a pedido do usuário — as faturas sempre são importadas antes de serem pagas, então essa projeção não agregava valor. As partes deste documento que o descreviam foram removidas ou ajustadas; ficam marcadas onde relevante.

## 1. Objetivo

Saber **exatamente quanto gasto por categoria, mês a mês**, com foco em **gastos recorrentes**.

Fluxo mensal: faço upload das faturas de cartão e do extrato da conta → o sistema extrai as transações, confere os números contra os totais impressos, categoriza sozinho o que tem certeza e me pergunta num chat, item a item, só o que ficou em dúvida → cada resposta vira uma regra salva, então a mesma coisa não é perguntada de novo → dashboards mostram para onde o dinheiro vai.

Critérios de sucesso:
1. Todo PDF importado **reconcilia centavo a centavo** com os totais impressos (ou é rejeitado com a diferença explicada).
2. No segundo mês de uso, a maioria dos lançamentos é categorizada sem pergunta.
3. O dashboard responde: quanto gastei em cada categoria no mês, quanto é custo fixo.

## 2. Decisões tomadas

| # | Decisão | Escolha |
|---|---|---|
| D1 | Mês em que um gasto de cartão conta | **Mês do fechamento da fatura.** Conta corrente: data do lançamento. Parcela conta no mês da fatura em que aparece. |
| D2 | Autonomia da IA | **Categoriza sozinha quando tem certeza** e pergunta só o incerto. PIX para pessoa sempre pergunta na primeira vez. Automáticos ficam numa lista revisável. |
| D3 | Extração | **Parser determinístico em Go por banco** (coordenadas do PDF) + **GPT como agente de categorização/perguntas**. O GPT nunca extrai números. |
| D4 | Escopo financeiro | Só **gastos** (saídas) e estornos. Pagamento de fatura, investimento, rendimento, receita e transferência entre contas próprias ficam fora dos dashboards (mas são guardados). |
| D5 | Stack | Go (API + extração), React + shadcn/ui (web), MySQL em Docker local. Nuvem depois. |
| D6 | Bancos | Arquitetura de providers genérica; **Itaú** primeiro (fatura de cartão + extrato de conta). |
| D7 | Usuários | Um usuário (eu), sem login na v1, API só em `127.0.0.1`. |

## 3. Escopo

**Dentro da v1:** provider Itaú (fatura + extrato), upload múltiplo, conferência de totais, deduplicação, categorização (regras → GPT → perguntas), chat de perguntas, regras/apelidos, detecção de recorrentes, dashboard, tabela de lançamentos com recategorização, edição de categorias e regras.

**Fora da v1:** outros bancos (a interface já prevê), login/multiusuário, deploy na nuvem, orçamento/metas, receitas e investimentos nos dashboards, dividir um lançamento entre categorias, app mobile, processamento assíncrono de importação.

## 4. Evidência que fundamenta o design (spike de 2026-09-26)

Com os PDFs reais (2 faturas Itaú + 1 extrato), usando go-pdfium (WASM) e o algoritmo de layout da seção 6:

| Documento | Extraído | Impresso |
|---|---|---|
| Fatura Platinum •••• 8778 | 52 compras = R$ 4.052,86; próxima fatura (parcelas) = R$ 36,47 | 4.052,86 / 36,47 ✓ |
| Fatura Infinite •••• 4229 | 62 compras R$ 4.614,10 + 2 internacionais R$ 1.954,93 + IOF R$ 68,42 = R$ 6.637,45; próxima fatura = R$ 618,78 | todos ✓ |
| Extrato conta 7293/030595-4 | 38 lançamentos, 14 linhas "SALDO DO DIA" | 13 reconciliações diárias ✓ |

Aprendizados que viram requisitos:
- Coordenadas do PDFium são em pontos com **eixo Y para cima** (origem embaixo à esquerda).
- **Letras acentuadas vêm em trechos (rects) separados**: `Lan` + `ç` + `amentos`. Trechos colados (vão < 1pt) juntam sem espaço.
- A cedilha desce abaixo da linha: agrupar linhas pelo **centro vertical**, com tolerância, e não pela base do glifo.
- A fatura tem **duas colunas**; texto extraído "em ordem de leitura" mistura colunas (`69,8909/09 CelsoFerreira…`). Separar por coordenada X resolve.
- Inicialização do runtime WASM ≈ 1,4 s (uma vez por processo); extração ≈ 15–65 ms por PDF.
- A porta 3306 da máquina já tem um MySQL nativo → o MySQL do Docker usa **3307**.

## 5. Arquitetura

```
jg-finance-tracker/
├── Makefile                  atalhos: db-up, api, web, test
├── docker-compose.yml        MySQL 8.4 em 127.0.0.1:3307 (volume nomeado)
├── .env.example              modelo de configuração (o .env real é ignorado pelo git)
├── backend/                  módulo Go "jgfinance" (Go 1.26)
│   ├── cmd/api/              main: config, migrations, wiring, servidor HTTP
│   └── internal/
│       ├── config/           lê .env/ambiente
│       ├── money/            parse de "1.234,56" → centavos (int64); formatação
│       ├── pdftext/          abre PDF (com senha) via go-pdfium WASM → páginas com trechos de texto + coordenadas
│       ├── layout/           trechos → linhas por coluna, em ordem de leitura
│       ├── provider/         interface Provider, tipos Statement/Transaction, registry, helpers (NormalizeKey)
│       │   └── itau/         Detect + ParseFatura + ParseExtrato + conferências
│       ├── importer/         pipeline de importação (hash, abrir, detectar, parse, dedupe, gravar)
│       ├── categorize/       regras, chamada ao GPT, perguntas, respostas, interpretação de texto livre
│       ├── insights/         consultas do dashboard: resumo, categorias, recorrentes, evolução, top
│       ├── llm/              interface JSON-com-schema + implementação OpenAI
│       ├── store/            MySQL: migrations embutidas (goose), repositórios
│       └── api/              handlers HTTP/JSON (net/http, padrões de rota do Go 1.22+)
├── web/                      Vite + React + TypeScript + Tailwind v4 + shadcn/ui
├── samples/                  PDFs reais para testes locais (ignorado pelo git)
├── data/                     uploads/ e uploads/failed/ (ignorado pelo git)
└── docs/superpowers/         specs e planos
```

**Dependências Go:** `github.com/klippa-app/go-pdfium` (webassembly, sem cgo), `github.com/go-sql-driver/mysql`, `github.com/pressly/goose/v3`, `github.com/openai/openai-go` (SDK oficial), `github.com/joho/godotenv`, `golang.org/x/text` (remoção de acentos), `github.com/google/go-cmp` (testes). Sem ORM, sem framework HTTP.

**Frontend:** React 19, TypeScript, Vite, Tailwind CSS v4, shadcn/ui (inclui charts sobre Recharts), TanStack Query v5, React Router v7, lucide-react, sonner. Em dev, o Vite (porta 5173) faz proxy de `/api` para a API (porta 8080).

**Configuração (`.env`):**

| Variável | Default | Uso |
|---|---|---|
| `MYSQL_DSN` | `finance:finance@tcp(127.0.0.1:3307)/finance?parseTime=true&loc=UTC` | banco principal |
| `MYSQL_TEST_DSN` | `finance:finance@tcp(127.0.0.1:3307)/finance_test?parseTime=true&loc=UTC` | testes de integração |
| `OPENAI_API_KEY` | vazio | sem chave → modo sem IA (tudo vira pergunta sem sugestão) |
| `OPENAI_MODEL` | `gpt-5-mini` | modelo para categorizar/interpretar |
| `ITAU_PDF_PASSWORD` | vazio | senha dos PDFs de fatura; qualquer variável `*_PDF_PASSWORD` entra na lista de senhas tentadas |
| `HTTP_ADDR` | `127.0.0.1:8080` | endereço da API |
| `DATA_DIR` | `./data` | onde ficam os PDFs importados |
| `SAMPLES_DIR` | vazio | pasta de PDFs reais para o teste de regressão |

**Convenções:** dinheiro sempre em centavos `int64` (nunca float). Datas são datas civis, representadas como `time.Time` à meia-noite UTC; DSN com `loc=UTC`. Mês de referência é `DATE` com dia 1. Código e identificadores em inglês; textos de UI em pt-BR.

## 6. Leitura do PDF (`pdftext` e `layout`)

`pdftext`:
- Um único pool go-pdfium WebAssembly por processo (inicialização preguiçosa, reutilizado).
- `Open(data []byte, passwords []string) (*Document, error)`: tenta abrir sem senha; se o PDF exigir senha, tenta cada senha da lista. Erros tipados: `ErrPasswordRequired` (exige senha e nenhuma configurada), `ErrWrongPassword` (nenhuma senha funcionou).
- Para cada página, usa `GetPageTextStructured` em modo **rects** e devolve `Run{Text, X0, Y0, X1, Y1}` já convertido para **origem no topo** (`Y = alturaDaPágina − yPDF`), com `Text` sem espaços nas pontas; trechos vazios são descartados.

`layout`:
- `Lines(page, splits []float64) []Line`: separa os trechos em colunas pelos limites X (`splits` vazio = uma coluna só); dentro de cada coluna, agrupa em linhas quando o **centro vertical** difere ≤ 2,5 pt da linha corrente; ordena cada linha por X e junta os trechos **sem espaço quando o vão horizontal é < 1,0 pt** e com um espaço caso contrário. Ordem de leitura: coluna 0 de cima para baixo, depois coluna 1.
- `Line{Text string; Column int; Y float64}`.
- `PageText(page) string`: linhas de largura total separadas por `\n` (usado para detecção e para campos de cabeçalho).

## 7. Providers

```go
type DocKind string   // "card_statement" | "bank_statement"
type TxKind string    // "expense" | "refund" | "bill_payment" | "own_transfer" | "investment" | "income" | "ignored" | "reversed"
                      // só expense e refund contam como gasto; "reversed" = estorno integral (seção 8.4)

type Provider interface {
    ID() string                                             // "itau"
    Detect(doc *pdftext.Document) (DocKind, bool)
    Parse(doc *pdftext.Document, kind DocKind) (*Statement, error) // inclui a conferência; erro se não bater
}

type Statement struct {
    Kind        DocKind
    Account     AccountRef          // Provider, Kind ("credit_card"|"checking"), ExternalID, DefaultName
    PeriodStart, PeriodEnd time.Time // extrato; zero na fatura
    ClosingDate, DueDate   time.Time // fatura; zero no extrato
    TotalCents  int64               // total impresso (fatura); 0 no extrato
    Transactions []Transaction
    Upcoming     []UpcomingInstallment // "Compras parceladas - próximas faturas", como impresso
    Warnings     []string
}

type Transaction struct {
    Seq                  int    // ordem no documento (1..n)
    Date                 time.Time
    Description          string // como impresso
    Key                  string // chave de estabelecimento normalizada (seção 9.1)
    AmountCents          int64  // > 0 dinheiro saindo; < 0 entrando/estorno
    Kind                 TxKind
    PersonTransfer       bool   // PIX para pessoa física: sempre pergunta na 1ª vez
    InstallmentN, InstallmentTotal int // 0 quando não parcelado
    BankCategory, City, Holder     string
    OriginalAmountCents  int64  // compras internacionais
    OriginalCurrency     string
}

type UpcomingInstallment struct {
    PurchaseDate time.Time; Description, Key string; AmountCents int64; N, Total int
}
```

O `registry` guarda os providers; `Detect` pergunta a cada um. Banco novo = novo pacote que implementa `Provider` e se registra. Helpers comuns: `provider.NormalizeKey(s)` = maiúsculas, sem acentos (NFD + remoção de marcas), espaços colapsados, trim; `provider.MarkReversals(txs)` marca os estornos integrais de um documento (seção 8.4).

### 7.1 Itaú — detecção

- **Fatura:** texto contém `Resumo da fatura em R$` e `Total desta fatura`, e (`ITAU UNIBANCO` ou `Banco Itaú`).
- **Extrato:** texto contém `extrato conta` e `SALDO DO DIA` e `itau.com.br` (comparação sem diferenciar maiúsculas).

### 7.2 Itaú — fatura de cartão

**Cabeçalho (página 1, `PageText`, busca por regex sem âncora):**
- `Cartão\s+\d{4}\.XXXX\.XXXX\.(\d{4})` → `ExternalID` (4 últimos dígitos).
- Nível do cartão: primeira ocorrência de `Platinum|Infinite|Gold|Black|Signature|Grafite|Nanquim|Internacional` → `DefaultName = "Itaú <nível> •••• <final>"` (sem nível: `"Itaú •••• <final>"`).
- `Postagem:\s*(dd/mm/aaaa)` → `ClosingDate`; `Vencimento:\s*(dd/mm/aaaa)` → `DueDate`.
- `Total desta fatura\s+(valor)`, `Lançamentos atuais\s+(valor)`, `Saldo financiado\s+(valor)` → totais impressos.

**Corpo (páginas 2+):**
- Divisão de colunas: X do trecho `DATA` mais à direita na primeira página que tiver dois agrupamentos de cabeçalho `DATA` (distância > 100 pt), **menos 15 pt**. Sem isso: 56,5 % da largura da página. Aplicado a todas as páginas 2+.
- Máquina de estados sobre as linhas, por prefixo:

| Linha começa com | Seção |
|---|---|
| `Pagamentos efetuados` | pagamentos |
| `Lançamentos: compras e saques` | compras |
| `Lançamentos: produtos e serviços` | produtos |
| `Lançamentos internacionais` | internacionais |
| `Compras parceladas - próximas faturas` | próximas |
| `Encargos cobrados`, `Limites de crédito`, `Novo teto`, `Simulação`, `Credito Rotativo`, `Fique atento`, `Lançamentos no cartão` | nenhuma (fim de seção) |

- **Linha de transação:** `^(\d{2}/\d{2})\s+(.+?)\s+(-?\d{1,3}(?:\.\d{3})*,\d{2})$`.
- **Totais impressos:** `P Total dos pagamentos`, `Lançamentos no cartão`, `Lançamentos produtos e serviços`, `Total transações inter. em R$`, `Repasse de IOF em R$`, `Total lançamentos inter. em R$`, `L Total dos lançamentos atuais`, `Próxima fatura`, `Demais faturas`, `Total para próximas faturas` (cada um seguido de um valor).
- **Linha de detalhe** (compras): a primeira linha após uma transação que não é transação, total, cabeçalho nem seção → 1ª palavra = categoria do banco (`restaurante`, `HEALTH`…), resto = cidade.
- **Internacionais:** linha seguinte `^(valor)\s+([A-Z]{3})\s+(valor)$` → valor original + moeda (o 1º valor na moeda indicada); `Dólar de Conversão` é ignorada.
- **Titular:** ao encontrar a linha de cabeçalho `DATA …` dentro de compras/internacionais, a linha imediatamente anterior, se for só letras maiúsculas e espaços, vira o `Holder` corrente.
- **Parcela:** descrição terminando em ` NN/NN` → `InstallmentN/Total` (compras e próximas).
- **Ano das datas:** `dd/mm` recebe o ano do fechamento; se a data resultante for posterior ao fechamento, ano − 1.
- **Tipo e sinal:** pagamentos → `bill_payment` (valor como impresso, negativo); compras/produtos/internacionais → valor como impresso: positivo = `expense`, negativo = `refund`. `Repasse de IOF em R$` vira uma transação `expense` com descrição `Repasse de IOF em R$`, data = fechamento.
- **Chave:** `NormalizeKey(descrição)` removendo o sufixo ` NN/NN` de parcela.

**Conferência (erro se falhar; regra ignorada quando o rótulo não existe):**
1. Σ compras = Σ `Lançamentos no cartão`. (Compras presentes sem esse total → erro "layout inesperado".)
2. Σ produtos = `Lançamentos produtos e serviços`.
3. Σ internacionais = `Total transações inter. em R$`; Σ internacionais + IOF = `Total lançamentos inter. em R$`.
4. Σ compras + Σ produtos + Σ internacionais + IOF = `L Total dos lançamentos atuais` = `Lançamentos atuais` (pág. 1).
5. `Total desta fatura` = `Saldo financiado` + `Lançamentos atuais`.
6. Σ próximas (como listadas) = `Próxima fatura`.
7. *Aviso (não erro):* projeção das parcelas restantes (seção 8.3) = `Demais faturas`.

A mensagem de erro traz a regra, o valor calculado, o impresso e a diferença.

### 7.3 Itaú — extrato de conta

- Cabeçalho: `período de visualização:\s*(dd/mm/aaaa)\s*até\s*(dd/mm/aaaa)` → período; `agência:\s*(\d+)` e `conta:\s*([\d-]+)` → `ExternalID = "<agência>/<conta>"`, `DefaultName = "Itaú conta <conta>"`.
- Linhas (coluna única): `^(\d{2}/\d{2}/\d{4})\s+(.+?)\s+(-?\d{1,3}(?:\.\d{3})*,\d{2})$`.
- `SALDO DO DIA` → saldo daquela data (não é transação).
- **Sinal:** `AmountCents = −valorImpresso` (saída vira positivo).
- **Tipo** (descrição normalizada):
  - começa com `FATURA PAGA` → `bill_payment`
  - começa com `APLICACAO` ou `RESGATE` → `investment`
  - começa com `JUROS POUP`, `REMUNER BASICA POUP`, `REND PAGO` ou `RENDIMENTO` → `income`
  - começa com `DEV PIX` ou contém `ESTORNO` → `refund`
  - senão: saída → `expense`; entrada → `income`
- `PersonTransfer = true` para saídas que começam com `PIX TRANSF`.
- **Chave:** `NormalizeKey(descrição)` removendo a data colada no fim (`\d{2}/\d{2}$`). Ex.: `PIX TRANSF LAURA N02/09` → `PIX TRANSF LAURA N`; `PIX TRANSF Cassiê 21/09` → `PIX TRANSF CASSIE`.
- **Conferência:** ordenar as datas com saldo; para cada par consecutivo (anterior, d): `saldo[d] = saldo[anterior] + Σ valorImpresso das transações em d`. Toda data de transação precisa ter linha de saldo. Qualquer divergência → erro com data, esperado e impresso.

## 8. Importação (`importer`)

### 8.1 Pipeline

`POST /api/imports` recebe vários PDFs; cada arquivo é processado em sequência e, no fim do lote, a categorização roda **uma vez** sobre todas as transações novas.

Por arquivo:
1. SHA-256 dos bytes; já existe em `documents` → resultado `duplicate`.
2. `pdftext.Open` com todas as senhas configuradas.
3. `registry.Detect` → nenhum provider reconhece → `failed` ("formato não reconhecido").
4. `Parse` (inclui conferência) → erro → `failed`; o PDF é copiado para `data/uploads/failed/<sha>.pdf`.
   - deu certo → `provider.MarkReversals` sobre as transações do documento (seção 8.4): os estornos integrais já entram com tipo `reversed` e nunca chegam à categorização.
5. Transação MySQL:
   - `upsert` da conta por (provider, kind, external_id);
   - fatura com (conta, fechamento) já existente → rollback e `duplicate` ("fatura já importada") — o Itaú gera bytes diferentes a cada download;
   - grava o PDF em `data/uploads/<sha>.pdf` e insere `documents` (com `reversals_checked = TRUE`);
   - insere transações com `INSERT IGNORE` pela `dedupe_key` (conta novas vs. já existentes);
   - commit.
6. Resultado por arquivo: `imported | duplicate | failed`, conta, tipo, mês de referência, contagens (novas/existentes), total, avisos, erro.

Falha na categorização (GPT fora do ar etc.) **não** desfaz importações; os itens viram perguntas sem sugestão.

### 8.2 Mês de referência e deduplicação

- Fatura: `reference_month` de todas as transações = 1º dia do mês de `ClosingDate`. Extrato: 1º dia do mês da data da transação.
- `dedupe_key` = SHA-256 hex de:
  - fatura: `card|<account_id>|<fechamento>|<seq>`
  - extrato: `bank|<account_id>|<data>|<descrição>|<valor>|<n>`, onde `n` é a ordem da ocorrência entre transações idênticas (data, descrição, valor) no mesmo documento — extratos com períodos sobrepostos não duplicam, e duas transações idênticas no mesmo dia continuam distintas.

### 8.3 Parcelas futuras (removido)

_Removido a pedido do usuário (2026-09-26): a projeção de parcelas futuras ("falta pagar") deixou de existir. A leitura da seção "Compras parceladas - próximas faturas" continua no parser só para a conferência do PDF (regra 6 e o aviso da regra 7, seção 7.2) — nada mais é gravado a partir dela._

### 8.4 Estornos integrais (`reversed`)

Compra e estorno que se anulam não são gasto, mas continuam como lançamento (tipo `reversed`, sem categoria nem pergunta). `provider.MarkReversals` roda sobre as transações de **um** documento, só `expense`/`refund`, agrupadas por (data, chave de estorno). Valor positivo = compra, negativo = estorno; tudo em centavos inteiros.

- **Chave de estorno:** a chave da transação; no extrato, `DEV PIX <NOME>`, `PIX QRS <NOME>` e `PIX TRANSF <NOME>` viram `PIX <NOME>`, então a devolução encontra o pagamento (`PIX QRS DAL SOTO SO` ↔ `DEV PIX DAL SOTO SO`).
- **Regra A:** grupo com ao menos uma compra e um estorno cuja soma é exatamente zero → todo o grupo vira `reversed`. Ex. real (fatura, 22/09, `ALIEXPRESSSAO PAULOBR`): +419,86, −17,42, −402,44.
- **Regra B:** grupo que não zera → cada estorno, na ordem `seq`, anula a primeira compra restante de mesmo valor absoluto; o resto fica como está. Ex.: compras de 50,00 e 30,00 e estorno de 30,00 → só o par de 30,00 vira `reversed`.
- Datas diferentes nunca se anulam; documentos diferentes também não. Rodar de novo não muda nada.
- **Extratos sobrepostos:** se um lançamento marcado `reversed` já existe no banco (o `INSERT IGNORE` o pula pela `dedupe_key`, vindo de um extrato anterior), a linha gravada — se ainda for `expense`/`refund` — também vira `reversed`, com as mesmas limpezas da varredura abaixo, dentro da mesma transação MySQL do arquivo. Assim o pagamento importado antes da devolução não fica contando como gasto.

**Documentos já importados:** `documents.reversals_checked` diz se o documento já foi avaliado. Na inicialização, a API varre os documentos com `reversals_checked = FALSE` (os importados antes desta regra), aplica `MarkReversals` às linhas `expense`/`refund` gravadas e marca o documento como avaliado — uma vez só, então a varredura é idempotente e nunca desfaz uma mudança manual posterior. Linha que vira `reversed` perde `category_id`, `categorized_by` e `question_id`; pergunta pendente/pulada que fica sem transações é marcada `answered`. Linhas categorizadas por mim (`user`) também são anuladas (compra estornada não é gasto) e cada uma é registrada no log.

`reversed` não é tipo de regra (`kind_override`) nem opção de "Não é gasto"; na tabela de lançamentos dá para voltar uma linha anulada a gasto (vira pergunta, como os outros tipos excluídos).

## 9. Categorização (`categorize`)

### 9.1 Chave

A chave (seções 7.2/7.3) identifica "o mesmo estabelecimento/pessoa" entre meses. Chaves são únicas em `rules`. Grafias que só diferem em acento/maiúscula viram a mesma chave (`Antônio`/`ANTONIO`); se forem pessoas diferentes, a regra usa o modo "perguntar".

### 9.2 Pipeline (após cada lote de importação)

Entrada: transações com tipo `expense` ou `refund`, sem categoria, sem `categorized_by` e sem pergunta — as novas do lote e qualquer outra que tenha ficado assim (ex.: execução interrompida). `POST /api/questions/suggest` faz a mesma varredura.

1. **Regra existe para a chave:**
   - `kind_override` definido → aplica o tipo (`own_transfer`/`ignored`), `categorized_by = rule`.
   - modo `always` → aplica a subcategoria, `categorized_by = rule`.
   - modo `ask` → uma pergunta **por transação**, com a subcategoria da regra como 1ª sugestão.
2. **Sem regra** → chaves vão ao GPT em lotes de até 40 chaves por chamada.
3. Resposta do GPT por chave:
   - `certain = true` e não é `PersonTransfer` → aplica a subcategoria (`categorized_by = llm`) e cria regra `source = llm`, modo `always`, com o apelido sugerido; se `likely_subscription`, a regra nasce com `recurring = yes`.
   - senão → uma pergunta por chave (agrupa todas as transações pendentes da chave) com até 3 sugestões e o texto da pergunta.
4. **Sem GPT** (sem chave de API, erro, timeout, JSON inválido) → uma pergunta por chave, sem sugestões. `POST /api/questions/suggest` tenta de novo depois.

**Regras semeadas** (source `user`): `REPASSE DE IOF EM R$` → Taxas › IOF; `MENSALIDADE - PLANO DO` e `REDUCAO MENSALIDADE - P` → Taxas › Tarifas e anuidade.

### 9.3 Contrato com o GPT

Interface mínima, trocável por outro modelo depois:

```go
type Client interface {
    // Gera JSON que obedece ao schema (Structured Outputs, strict) e decodifica em out.
    JSON(ctx context.Context, req Request, out any) error
}
type Request struct { System, User, SchemaName string; Schema map[string]any }
```

**Categorizar (lote):**
- Envia por chave: descrição(ões) de exemplo, quantidade, total, datas, fonte (cartão/pix), categoria e cidade do banco, se é PIX para pessoa; mais a lista de subcategorias (`id` + caminho "Categoria › Subcategoria") e até 40 regras recentes minhas como exemplos.
- Schema de saída: `{"items":[{"key":string, "subcategory_id":integer(enum dos ids válidos), "certain":boolean, "alias":string, "likely_subscription":boolean, "question":string, "suggestions":[integer(enum)] }]}` — todos os campos obrigatórios, `additionalProperties: false`.
- Instruções principais (pt-BR): categoria do banco é só uma dica e costuma errar ("outros"); `certain` apenas quando o nome deixa a categoria inequívoca; PIX para pessoa, marketplaces sem dica específica e nomes ambíguos → `certain = false`; `alias` curto e legível ("Komprão", "99", "iFood · Nagasaki Sushi"); `question` curta em pt-BR; `suggestions` do mais ao menos provável.
- Validação no Go: chaves devolvidas = chaves enviadas; ids existentes; itens faltando viram perguntas sem sugestão.

**Interpretar texto livre** (resposta digitada no chat):
- Entrada: contexto da pergunta + texto ("é a psicóloga") + subcategorias.
- Saída: `{"kind": "expense"|"own_transfer"|"ignored", "subcategory_id": integer|null, "alias": string}`. A UI mostra a interpretação e **só grava depois da minha confirmação**.

Modelo via `OPENAI_MODEL`; timeout de 60 s por chamada.

### 9.4 Perguntas e respostas

- Pergunta = chave + transações ligadas (`transactions.question_id`) + sugestões + texto. Status `pending | answered | skipped`; puladas voltam depois das pendentes.
- **Responder** (`category_id` ou `kind`, `remember`, `alias` opcional):
  - aplica em todas as transações ligadas: subcategoria ou tipo (`own_transfer`/`ignored`), `categorized_by = user`;
  - `remember = true` → regra modo `always`; `false` → regra modo `ask` (próxima vez pergunta de novo, sugerindo esta resposta). Regra com `source = user` (upsert pela chave);
  - marca a pergunta como `answered`.
- **Automáticos:** lista de transações com `categorized_by = llm`, agrupadas por chave. Corrigir = editar a regra (vira `source = user`) e **reaplicar em todas as transações da chave com `categorized_by` em (`rule`, `llm`)**; correções manuais por transação (`user`) nunca são sobrescritas.
- **Recategorizar na tabela:** escopo `one` (só a transação, `categorized_by = user`) ou `key` (edita/cria a regra e reaplica como acima).

## 10. Recorrentes (`insights`)

**Recorrente no mês M** (por chave, só `expense`):
- regra `recurring = yes` → sempre; `no` → nunca;
- `auto` → a chave aparece em uma sequência de meses consecutivos terminando em M com **pelo menos 2 meses**, **no máximo 2 lançamentos por mês**, e o total de cada mês dentro de **±20 % da mediana** da sequência (janela de até 6 meses).
- Status por item: `ok`; `aumentou` (total em M > 110 % do mês anterior); `ausente` (era recorrente em M−1 e não apareceu em M).
- Campos: chave, apelido (regra) ou última descrição, caminho da categoria, total em M, média da sequência, variação %, meses seguidos, status.
- **Custo fixo de M** = Σ totais em M dos recorrentes.

_"Falta pagar" (faturas a vencer + projeção de parcelas futuras) removido a pedido do usuário (2026-09-26)._

## 11. Modelo de dados (MySQL 8.4, utf8mb4)

```sql
CREATE TABLE accounts (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  provider VARCHAR(32) NOT NULL,
  kind ENUM('credit_card','checking') NOT NULL,
  external_id VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_accounts (provider, kind, external_id)
);

CREATE TABLE documents (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  account_id BIGINT NOT NULL,
  kind ENUM('card_statement','bank_statement') NOT NULL,
  file_sha256 CHAR(64) NOT NULL,
  file_name VARCHAR(255) NOT NULL,
  stored_path VARCHAR(512) NOT NULL,
  period_start DATE NULL, period_end DATE NULL,
  closing_date DATE NULL, due_date DATE NULL,
  reference_month DATE NULL,
  total_cents BIGINT NULL,
  reversals_checked BOOLEAN NOT NULL DEFAULT FALSE, -- estornos integrais já avaliados (8.4)
  imported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_documents_sha (file_sha256),
  UNIQUE KEY uq_documents_card_closing (account_id, closing_date),  -- NULL no extrato não conflita
  FOREIGN KEY (account_id) REFERENCES accounts(id)
);

CREATE TABLE categories (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  parent_id BIGINT NULL,                -- NULL = categoria; preenchido = subcategoria
  name VARCHAR(64) NOT NULL,
  color VARCHAR(16) NULL,               -- só em categorias
  position INT NOT NULL DEFAULT 0,
  archived BOOLEAN NOT NULL DEFAULT FALSE,
  FOREIGN KEY (parent_id) REFERENCES categories(id)
);

CREATE TABLE rules (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  merchant_key VARCHAR(191) NOT NULL,
  category_id BIGINT NULL,              -- subcategoria; NULL quando kind_override exclui
  kind_override ENUM('own_transfer','ignored') NULL,
  mode ENUM('always','ask') NOT NULL DEFAULT 'always',
  alias VARCHAR(128) NULL,
  recurring ENUM('auto','yes','no') NOT NULL DEFAULT 'auto',
  source ENUM('user','llm') NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  UNIQUE KEY uq_rules_key (merchant_key),
  FOREIGN KEY (category_id) REFERENCES categories(id)
);

CREATE TABLE questions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  merchant_key VARCHAR(191) NOT NULL,
  per_transaction BOOLEAN NOT NULL DEFAULT FALSE,
  suggestions JSON NULL,                -- [subcategory_id, ...] (até 3)
  prompt VARCHAR(512) NULL,
  status ENUM('pending','answered','skipped') NOT NULL DEFAULT 'pending',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  answered_at DATETIME NULL,
  KEY ix_questions_status (status)
);

CREATE TABLE transactions (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  document_id BIGINT NOT NULL,
  account_id BIGINT NOT NULL,
  seq INT NOT NULL,
  occurred_on DATE NOT NULL,
  reference_month DATE NOT NULL,
  description VARCHAR(255) NOT NULL,
  merchant_key VARCHAR(191) NOT NULL,
  amount_cents BIGINT NOT NULL,         -- > 0 saída; < 0 entrada/estorno
  kind ENUM('expense','refund','bill_payment','own_transfer','investment','income','ignored','reversed') NOT NULL,
  person_transfer BOOLEAN NOT NULL DEFAULT FALSE,
  category_id BIGINT NULL,              -- sempre subcategoria
  categorized_by ENUM('rule','llm','user') NULL,
  question_id BIGINT NULL,
  installment_n SMALLINT NULL,
  installment_total SMALLINT NULL,
  bank_category VARCHAR(64) NULL,
  city VARCHAR(64) NULL,
  holder VARCHAR(128) NULL,
  original_amount_cents BIGINT NULL,
  original_currency CHAR(3) NULL,
  dedupe_key CHAR(64) NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uq_transactions_dedupe (dedupe_key),
  KEY ix_transactions_month (reference_month),
  KEY ix_transactions_key (merchant_key),
  KEY ix_transactions_category (category_id),
  FOREIGN KEY (document_id) REFERENCES documents(id),
  FOREIGN KEY (account_id) REFERENCES accounts(id),
  FOREIGN KEY (category_id) REFERENCES categories(id),
  FOREIGN KEY (question_id) REFERENCES questions(id)
);

```

_Tabela `installments` (projeção de parcelas futuras) removida a pedido do usuário (2026-09-26) — ver migration `00005_drop_installments`._

Migrations com goose, embutidas no binário e aplicadas na inicialização da API. A migration de seed cria a taxonomia (seção 15) e as regras semeadas (9.2). A migration `00003_reversed` acrescenta `reversed` a `transactions.kind` e `documents.reversals_checked` (8.4). A migration `00005_drop_installments` remove a tabela `installments` (falta pagar removido, 2026-09-26).

## 12. API HTTP (JSON, prefixo `/api`)

| Método e rota | Função |
|---|---|
| `POST /api/imports` | multipart `files[]` → `{results:[…], categorization:{auto, questions}}` |
| `GET /api/documents` | documentos importados |
| `GET /api/accounts` · `PATCH /api/accounts/{id}` | listar · renomear |
| `GET /api/categories` · `POST` · `PATCH /api/categories/{id}` | árvore · criar · renomear/cor/ordem/arquivar |
| `GET /api/questions?status=pending\|skipped` | perguntas com transações ligadas, sugestões (id + caminho), total |
| `POST /api/questions/{id}/answer` | `{category_id?, kind?, remember, alias?}` |
| `POST /api/questions/{id}/skip` | pular |
| `POST /api/questions/{id}/interpret` | `{text}` → interpretação para confirmar |
| `POST /api/questions/suggest` | refaz sugestões do GPT para perguntas sem sugestão |
| `GET /api/transactions` | filtros: `month`, `account_id`, `category_id`, `kind`, `q`, `uncategorized`, `categorized_by` |
| `PATCH /api/transactions/{id}` | `{category_id?, kind?, scope: "one"\|"key"}` |
| `GET /api/rules` · `PATCH /api/rules/{id}` · `DELETE /api/rules/{id}` | regras |
| `GET /api/months` | meses com dados (mais recente primeiro) |
| `GET /api/dashboard?month=AAAA-MM` | `{summary, by_category, recurring, evolution, top_merchants}` |

Erros: `{"error": "mensagem em pt-BR"}` com status 400/404/409/422/500. Upload: só `application/pdf`, até 20 MB por arquivo. Timeout de escrita do servidor: 5 min (lotes grandes de dados antigos); `POST /api/imports` e `POST /api/questions/suggest` estendem o prazo para 30 min e vão até o fim mesmo se o cliente desconectar.

`summary` = `{total, prev3_avg, fixed, uncategorized:{count,total}}`; `prev3_avg` = média dos totais dos meses **com dados** entre M−3 e M−1 (`null` se nenhum). `by_category` = categorias com total e subcategorias. `evolution` = últimos 12 meses por categoria. `top_merchants` = 10 maiores chaves do mês (apelido, total, quantidade, categoria). Todos os valores em centavos; só entram `expense` e `refund`. (`open_bills`, `future_installments` e `upcoming` removidos a pedido do usuário, 2026-09-26.)

## 13. Web

Layout com barra lateral: **Dashboard**, **Importar**, **Perguntas** (com contador de pendentes), **Lançamentos**, **Categorias e regras**. Formatação pt-BR (`Intl.NumberFormat('pt-BR', {style:'currency', currency:'BRL'})`), tema claro/escuro do shadcn.

1. **Importar** — área de arrastar/soltar (vários PDFs); durante o envio, estado "importando e categorizando"; depois, um cartão por arquivo: conta, mês, "52 compras · R$ 4.052,86 ✓ conferido" / "já importado" / erro em destaque; botão "Responder N perguntas".
2. **Perguntas** — um card por vez, progresso "3 de 21": chave (ou descrição), valor total, quantidade, datas, conta, categoria/cidade do banco; até 3 sugestões numeradas + "Outra categoria…" (combobox com busca); campo de texto livre (interpretação com confirmação); campo de apelido; opção "Lembrar nas próximas" (ligada por padrão); "Não é gasto" (transferência pra conta minha / ignorar); "Pular". Atalhos: `1`–`3` escolhe sugestão, `Enter` confirma, `S` pula. Aba **Automáticos** com o que o GPT decidiu, agrupado por chave, com troca de categoria.
3. **Dashboard** (seletor de mês) — nesta ordem: indicadores (total do mês, comparação com a média dos 3 meses anteriores, custo fixo) com aviso de "sem categoria" quando houver; gasto por categoria (barras horizontais ordenadas, clique abre subcategorias e lançamentos); recorrentes (tabela com status); evolução (barras empilhadas por categoria, 12 meses); onde mais gastei (top 10). (Cartão "Falta pagar" e seção de parcelas futuras/faturas a vencer removidos a pedido do usuário, 2026-09-26.)
4. **Lançamentos** — tabela com filtros (mês, conta, categoria, tipo, texto, sem categoria), troca de categoria na linha com escolha de escopo ("só este" / "todos desta chave"). Estornos integrais aparecem com o selo discreto "Estorno anulado" (com explicação ao passar o mouse), linha esmaecida, fora dos totais; o filtro de tipo inclui "Estorno anulado". Dashboards só os excluem.
5. **Categorias e regras** — criar/renomear/arquivar categorias e subcategorias, cor das categorias; editar regra (subcategoria, tipo, modo, apelido, recorrente) ou apagar.

## 14. Erros

| Situação | Comportamento |
|---|---|
| PDF pede senha e nenhuma configurada | `failed`: "O PDF tem senha: configure ITAU_PDF_PASSWORD no .env" |
| Senha errada | `failed`: "Nenhuma das senhas configuradas abriu este PDF" |
| Formato não reconhecido | `failed`: "Nenhum banco reconheceu este arquivo" |
| Conferência falhou | `failed` com regra, calculado, impresso e diferença; PDF em `data/uploads/failed/` |
| Arquivo ou fatura repetida | `duplicate` |
| Extrato sobreposto | `imported` com "N novos, M já existiam" |
| GPT indisponível/inválido | importação conclui; perguntas sem sugestão; botão "sugerir de novo" |
| Erro inesperado | 500 com mensagem genérica; detalhe no log (sem senha, sem chave de API) |

Cada arquivo é gravado numa transação MySQL única: entra inteiro ou não entra nada.

## 15. Taxonomia inicial (seed)

| Categoria | Subcategorias |
|---|---|
| Moradia | Aluguel · Condomínio · Energia · Água e gás · Celular e internet · Manutenção · Outros |
| Alimentação | Supermercado · Restaurante · Delivery · Padaria e café · Outros |
| Transporte | Aplicativo · Combustível · Estacionamento e pedágio · Manutenção do veículo · Outros |
| Saúde | Terapia · Farmácia · Consultas e exames · Plano de saúde · Outros |
| Lazer e esporte | Padel · Academia · Bares e eventos · Ingressos e shows · Games · Outros |
| Compras | Roupas · Eletrônicos · Casa · Online · Outros |
| Assinaturas | Software e IA · Streaming · Outros |
| Cuidados pessoais | Barbearia · Estética · Outros |
| Educação | Cursos · Livros · Outros |
| Viagens | Passagens · Hospedagem · Passeios · Outros |
| Pessoas e presentes | Presentes · Ajuda familiar · Divisão de contas · Outros |
| Taxas | IOF · Tarifas e anuidade · Juros · Outros |
| Outros | Outros |

Cada categoria recebe uma cor de uma paleta categórica validada para contraste nos temas claro e escuro.

## 16. Testes

- **Regressão com PDFs reais:** teste que percorre `SAMPLES_DIR` (pulado quando vazio/ausente), abre cada PDF (usando `ITAU_PDF_PASSWORD` se precisar; sem senha → pula aquele arquivo com aviso), detecta e faz parse — o parse inclui a conferência, então **todo PDF real precisa reconciliar**. Cada PDF antigo colocado em `samples/` vira um teste.
- **Unitários versionados:** `layout` com trechos sintéticos (colunas, acento em trecho separado, cedilha, ordem Y); parsers Itaú a partir de linhas de texto inventadas (duas colunas, parcela, estorno, internacional + IOF, anuidade, titular, saldo diário, conferência que falha); `pdftext` com um PDF sintético minúsculo criptografado (senha de teste) versionado em `testdata/`; `money`; `NormalizeKey`.
- **Categorização:** regras, perguntas e respostas com um `llm.Client` falso.
- **Store/API:** integração contra `MYSQL_TEST_DSN` (pulados se indisponível), migrations aplicadas, tabelas limpas entre testes; handlers com `httptest`.
- **Web:** `tsc`/lint e build como portão; Vitest para formatação e lógica pura.
- Desenvolvimento em TDD.

## 17. Segurança e privacidade

- API escuta só em `127.0.0.1`; sem login na v1.
- Contra outros sites abertos no navegador (DNS rebinding, CSRF): a API responde 403 a `Host` que não seja `localhost`/`127.0.0.1`/`[::1]` (qualquer porta); escritas (não-GET) exigem `Content-Type` `application/json` ou `multipart/form-data` (senão 415) e, havendo `Origin`, que seja local (senão 403). O proxy do Vite repassa o `Host` do dev server (`localhost:5173`), que é aceito.
- `.env`, `samples/`, `data/` e qualquer `*.pdf` (exceto o fixture sintético) ficam fora do git.
- Para a OpenAI vão apenas descrição, valor, data e categoria/cidade do banco de cada lançamento a categorizar — nunca o PDF, CPF, endereço, número de cartão ou conta.
- Senhas e chave de API nunca aparecem em logs nem em respostas da API.

## 18. Evolução para a nuvem (fora da v1)

O binário Go passa a servir o build do React (`embed`) → um container. MySQL gerenciado, uploads em storage compatível com S3 (atrás de uma interface), login. Nada do design atual precisa mudar para isso.
