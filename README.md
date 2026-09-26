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

## Testes

```bash
make test           # unitários + integração (os de banco são pulados sem o MySQL de `make db-up`)
make test-samples   # regressão com os PDFs reais de samples/
```
