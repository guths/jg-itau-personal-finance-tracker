# JG Finance Tracker

Organiza meus gastos a partir das faturas de cartão e dos extratos da conta (Itaú primeiro):
extrai as transações dos PDFs, confere cada documento centavo a centavo contra os totais
impressos, categoriza com ajuda de IA e mostra dashboards por categoria e recorrentes.

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

## Rodar a API

Requer Docker (MySQL 8.4 em `127.0.0.1:3307`; a 3306 costuma estar ocupada por um MySQL nativo).

```bash
make db-up   # sobe o MySQL e espera ficar saudável (cria também o banco de testes)
make api     # aplica as migrations e escuta em http://127.0.0.1:8080
```

Sem `OPENAI_API_KEY` no `.env` a API funciona em modo sem IA: tudo que não tiver regra vira
pergunta sem sugestão. Os PDFs importados ficam em `data/uploads/` (e os que falharam na
conferência em `data/uploads/failed/`). Rotas em `docs/superpowers/specs/2026-09-26-finance-tracker-design.md` §12.

Exemplo:

```bash
curl -F files=@samples/itau/fatura.pdf http://127.0.0.1:8080/api/imports
curl 'http://127.0.0.1:8080/api/dashboard?month=2026-09'
```

`make db-down` para o MySQL (os dados ficam no volume `mysql-data`).

## Rodar o app

Requer também Node 24+ e npm. Em três terminais:

```bash
make db-up   # MySQL em 127.0.0.1:3307
make api     # API em http://127.0.0.1:8080
make web     # app em http://localhost:5173 (instala as dependências na primeira vez)
```

O Vite encaminha `/api` para a API, então basta abrir http://localhost:5173. Fluxo mensal:
**Importar** os PDFs → responder o que ficou em dúvida em **Perguntas** → ver o **Dashboard**.
Correções ficam em **Lançamentos** (troca de categoria na linha, só este ou todos da chave) e em
**Categorias e regras** (taxonomia, cores, regras aprendidas e nomes das contas).

`make web-build` gera o build de produção em `web/dist/`.

## Testes

```bash
make test           # unitários + integração (os de banco são pulados sem o MySQL de `make db-up`)
make test-samples   # regressão com os PDFs reais de samples/
make web-test       # lint + Vitest do app web
```

## Backup (restic → Backblaze B2 ou Cloudflare R2)

O banco local é copiado, **criptografado na sua máquina**, para um bucket na nuvem. O provedor
nunca vê o conteúdo. Retenção: 7 diários, 8 semanais, 12 mensais e 3 anuais.

1. Crie um bucket **privado** no B2 (ou R2) e uma chave de acesso restrita a ele.
2. Preencha `RESTIC_PASSWORD`, `RESTIC_REPOSITORY` e as credenciais no `.env` (modelo no `.env.example`).
   **Guarde a `RESTIC_PASSWORD` num gerenciador de senhas: sem ela o backup não pode ser restaurado.**
3. Rode uma vez:

```bash
make backup-init      # cria o repositório
make backup           # primeiro backup
make backup-schedule  # backup diário às 21:00 (launchd; log em data/backup.log)
```

Restaurar (o banco atual é copiado antes para `data/backups/`):

```bash
make restore                              # último snapshot → banco finance (pede confirmação)
make restore RESTORE_DB=finance_check     # restaura em outro banco, sem tocar nos dados
```
