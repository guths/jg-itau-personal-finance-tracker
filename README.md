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
