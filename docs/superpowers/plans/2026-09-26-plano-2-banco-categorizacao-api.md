# Plano 2 — Banco, importação, categorização (GPT) e API

> Execução em lote (preferência do usuário por velocidade): um implementador executa as tarefas em ordem, um commit por tarefa, TDD; uma revisão ao final.

**Goal:** API HTTP local que importa PDFs (usando o plano 1), grava no MySQL, categoriza (regras → GPT → perguntas), responde perguntas e serve os dados do dashboard.

**Spec:** `docs/superpowers/specs/2026-09-26-finance-tracker-design.md` — seções 5, 8, 9, 10, 11, 12, 14, 15, 16, 17 são requisitos vinculantes; este plano só ordena o trabalho e fixa interfaces.

**Tech:** Go 1.26 (módulo `jgfinance`), `github.com/go-sql-driver/mysql`, `github.com/pressly/goose/v3` (migrations embutidas), `github.com/openai/openai-go` (versão major mais recente disponível; Structured Outputs com JSON schema strict), net/http (padrões de rota do Go 1.22+). Sem ORM, sem framework HTTP.

## Global Constraints
- Tudo das Global Constraints do plano 1 continua valendo (centavos int64, datas UTC, inglês no código / pt-BR para o usuário, nenhum dado real versionado, nunca escrever senha de PDF nem chave de API em arquivo).
- MySQL do Docker em `127.0.0.1:3307` (a 3306 já está ocupada na máquina). DSN com `parseTime=true&loc=UTC`.
- Testes de integração usam `MYSQL_TEST_DSN` e fazem `t.Skip` se o banco não responder. O GPT é sempre substituído por um fake nos testes (nunca chamar a API real em teste).
- Commits `tipo(escopo): descrição pt-BR` + trailer `Co-Authored-By` do seu modelo.

## Tarefas

### Task 1 — docker-compose + store (migrations, seed, conexão)
- `docker-compose.yml` na raiz: `mysql:8.4`, porta `127.0.0.1:3307:3306`, volume nomeado, `MYSQL_DATABASE=finance`, usuário `finance`/`finance`, script em `docker/mysql-init/01-test-db.sql` criando `finance_test` com grant para `finance`; healthcheck. Makefile: `db-up`, `db-down`.
- `backend/internal/store/migrations/*.sql` (goose, embed): schema exato da spec §11; migration de seed com a taxonomia da §15 (cada categoria com cor de uma paleta categórica fixa de 13 cores legíveis em tema claro/escuro; subcategorias sem cor) e as 3 regras semeadas da §9.2 (`REPASSE DE IOF EM R$` → Taxas › IOF; `MENSALIDADE - PLANO DO` e `REDUCAO MENSALIDADE - P` → Taxas › Tarifas e anuidade; source `user`).
- `store.Open(dsn) (*Store, error)` aplica migrations; `store.Store` com `DB *sql.DB`.
- Helper de teste `storetest.New(t) *store.Store` que conecta no `MYSQL_TEST_DSN` (via `config.Load`), aplica migrations, limpa todas as tabelas exceto categorias/regras semeadas (re-semeia) e faz Skip se indisponível.
- Teste: migrations sobem; 13 categorias e suas subcategorias existem; 3 regras semeadas.

### Task 2 — importer
- `importer.New(store, registry, passwords, dataDir, categorizer)`; `Import(ctx, files []File{Name string; Data []byte}) (BatchResult, error)`.
- Pipeline e regras exatamente da spec §8.1–8.3 (hash, duplicate, pdftext.Open, Detect, Parse, failed → copiar para `data/uploads/failed/<sha>.pdf`, transação MySQL por arquivo, upsert de conta, fatura duplicada por (conta, fechamento), `data/uploads/<sha>.pdf`, `INSERT IGNORE` por dedupe_key com contagem novas/existentes, reference_month, dedupe_key, projeção de parcelas).
- Ao fim do lote chama `categorizer.Run(ctx, newTxIDs)`; erro dele não desfaz importação (vai para o resultado como aviso).
- `FileResult{Name, Status ("imported"|"duplicate"|"failed"), AccountName, Kind, ReferenceMonth, NewCount, ExistingCount, TotalCents, Warnings, Error}`.
- Testes de integração: usar um `provider.Provider` fake + `pdftext` real com o `testdata/plain.pdf` (o fake detecta pelo texto "Topo" e devolve um Statement fixo) para cobrir: import ok, arquivo repetido, fatura repetida com bytes diferentes (outro Statement igual com outro arquivo → mude os bytes com um PDF diferente: gere um segundo fixture sintético se precisar), extrato sobreposto (dedupe), falha de conferência (fake devolve `*provider.ReconciliationError`) grava em failed/, projeção de parcelas (n=4,N=10 → 7 linhas em installments com due_month M+1…M+7).

### Task 3 — llm
- `llm.Client` interface e `llm.Request` exatamente como a spec §9.3; implementação `llm.NewOpenAI(apiKey, model)` usando o SDK oficial com Structured Outputs (json_schema, strict) e timeout de 60 s. Sem chave → `categorize` recebe `nil` e opera em modo sem IA.
- Teste unitário da montagem da requisição apenas onde possível sem rede (não chamar a API).

### Task 4 — categorize
- `categorize.New(store, llm.Client /* pode ser nil */)` com:
  - `Run(ctx, txIDs []int64) (Result{Auto, Questions int}, error)` — spec §9.2 (regras → GPT em lotes de 40 chaves → aplica/pergunta; `PersonTransfer` sempre pergunta; sem GPT → perguntas sem sugestão; `likely_subscription` com certeza → regra `recurring=yes`).
  - `Answer(ctx, questionID, Answer{CategoryID *int64; Kind string; Remember bool; Alias string}) error` — spec §9.4.
  - `Skip(ctx, questionID) error`; `Interpret(ctx, questionID, text) (Interpretation{Kind string; CategoryID *int64; Alias string}, error)` — spec §9.3.
  - `Suggest(ctx) error` — refaz sugestões de perguntas pendentes sem sugestão.
  - `SetTransactionCategory(ctx, txID, categoryID *int64, kind string, scope "one"|"key") error` e `UpdateRule(...)` com a reaplicação da §9.4 (nunca sobrescreve `categorized_by=user`).
- Prompts em pt-BR com as instruções da §9.3; schema com enum dos ids de subcategorias ativas; validação das chaves devolvidas.
- Testes com fake LLM: regra existente aplica; modo ask gera pergunta por transação; GPT certo aplica + cria regra llm; GPT incerto gera pergunta agrupada com sugestões; PIX pessoa com GPT certo ainda pergunta; sem GPT → pergunta sem sugestão; resposta remember=true/false cria regra always/ask e categoriza todas as ligadas; resposta own_transfer muda o kind; correção de regra reaplica em rule/llm mas não em user.

### Task 5 — insights (dashboard)
- `insights.Dashboard(ctx, month time.Time, today time.Time) (Dashboard, error)` com `Summary`, `ByCategory`, `Recurring`, `Upcoming`, `Evolution`, `TopMerchants` exatamente como spec §10 e §12 (só `expense` e `refund`; `prev3_avg` só meses com dados; recorrência com janela de 6 meses, sequência ≥ 2 meses, ≤ 2 lançamentos/mês, ±20 % da mediana, status ok/aumentou/ausente; regra yes/no sobrepõe; parcelas só da fatura mais recente de cada cartão; faturas a vencer `due_date ≥ today`).
- `insights.Months(ctx)` — meses com dados, mais recente primeiro.
- Testes de integração com dados inseridos diretamente: soma por categoria, recorrente detectado/ausente/aumentou, parcelas usando só a fatura mais recente, prev3_avg.

### Task 6 — API HTTP + cmd/api
- Rotas exatamente da spec §12 (`/api/...`), JSON em snake_case, erros `{"error": "..."}` em pt-BR, upload multipart `files` (só PDF, 20 MB/arquivo), timeout de escrita 5 min, escuta em `HTTP_ADDR` (127.0.0.1).
- `cmd/api/main.go`: `config.Load("../.env", ".env")`, `store.Open`, registry com `itau.New()`, llm OpenAI se houver `OPENAI_API_KEY`, importer, categorize, insights, servidor. Makefile: `api`.
- Testes de handlers com `httptest` + store de teste + fake LLM: upload (fake provider), listar/responder/pular perguntas, PATCH transação escopo one/key, dashboard 200 com mês, erros 400/404.
- README: seção "Rodar a API" (make db-up, make api).

## Review Focus
1. Dois uploads concorrentes/seguidos do mesmo PDF → um `imported`, outro `duplicate`, nada duplicado.
2. Importar faturas antigas fora de ordem → "falta pagar" usa só a fatura mais recente de cada cartão.
3. Resposta do GPT com chave inventada/omitida ou id inexistente → itens viram perguntas sem sugestão, sem erro 500.
4. Corrigir uma regra não altera lançamentos recategorizados manualmente (`categorized_by=user`).
5. Extrato cobrindo dois meses → cada transação no mês da sua data; dashboard de um mês não mistura o outro.
