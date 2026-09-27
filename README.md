# JG Finance Tracker

Organiza seus gastos a partir das **faturas de cartão** e dos **extratos da conta** do Itaú:
lê os PDFs, confere cada documento centavo a centavo contra os totais impressos, separa os
gastos em categorias (com ajuda de IA) e mostra um dashboard por mês com categorias e gastos
recorrentes. Tudo roda no seu computador.

> Hoje só o **Itaú** é suportado (fatura de cartão e extrato de conta em PDF).

---

## Usar o app (passo a passo com Docker)

Você não precisa instalar Go, Node nem MySQL: o Docker cuida de tudo. Leva uns 15 minutos na
primeira vez.

### 1. Instale o Docker Desktop

1. Baixe em https://www.docker.com/products/docker-desktop/ (Mac, Windows ou Linux) e instale.
2. Abra o Docker Desktop e espere aparecer **Engine running** (canto inferior esquerdo).
3. Confira num terminal (no Mac: app *Terminal*; no Windows: *PowerShell*):

   ```bash
   docker compose version
   ```

   Deve mostrar algo como `Docker Compose version v2.x`.

### 2. Baixe o projeto

Com git:

```bash
git clone <URL-DO-REPOSITÓRIO> jg-finance-tracker
cd jg-finance-tracker
```

Sem git: baixe o ZIP do repositório, descompacte e, no terminal, entre na pasta com `cd`.

### 3. Crie uma chave da OpenAI (para a IA sugerir categorias)

A IA lê só a **descrição, o valor e a data** de cada gasto para sugerir a categoria. O PDF, o
CPF e os números de cartão/conta nunca saem do seu computador.

1. Entre em https://platform.openai.com e faça login (pode ser a mesma conta do ChatGPT).
2. Em **Settings → Billing**, adicione crédito. A API é **pré-paga e separada** da assinatura
   do ChatGPT; US$ 5 duram muito tempo (categorizar um mês custa centavos).
3. Em **API keys** (https://platform.openai.com/api-keys), clique **Create new secret key**,
   dê um nome (ex.: `finance-tracker`) e **copie a chave** (começa com `sk-`). Ela só aparece
   uma vez.
4. Guarde a chave com cuidado: quem tiver a chave gasta o seu crédito.

Sem chave o app funciona igual, só que sem sugestões: tudo que ele não conhece vira pergunta
para você responder.

### 4. Configure o `.env`

Na pasta do projeto, copie o modelo:

```bash
cp .env.example .env        # Mac/Linux
copy .env.example .env      # Windows (PowerShell)
```

Abra o `.env` num editor de texto e preencha só estas duas linhas (deixe o resto como está):

```
OPENAI_API_KEY=sk-...sua-chave...
ITAU_PDF_PASSWORD=...senha dos PDFs da fatura...
```

- **`ITAU_PDF_PASSWORD`**: as faturas do Itaú vêm em PDF com senha. Normalmente são os
  **5 primeiros dígitos do CPF do titular** do cartão; se não abrir, veja a senha indicada no
  e-mail da fatura. O extrato da conta não tem senha.
- O `.env` fica só no seu computador (nunca vai para o git nem para dentro da imagem Docker).

### 5. Suba o app

```bash
docker compose --profile app up -d --build
```

Na primeira vez ele baixa e compila tudo (alguns minutos). Quando terminar, abra:

**http://localhost:8080**

### 6. Use todo mês

1. **Baixe os PDFs** no app ou site do Itaú: a **fatura** de cada cartão e o **extrato** da
   conta do mês.
2. **Importar**: arraste os PDFs (pode vários de uma vez, inclusive meses antigos, em qualquer
   ordem). Cada arquivo mostra se foi conferido; arquivo repetido é ignorado.
3. **Perguntas**: responda o que ficou em dúvida (principalmente PIX para pessoas). Cada
   resposta vira uma regra, então no mês seguinte ele já sabe.
4. **Dashboard**: veja o mês por categoria, os recorrentes e a evolução.
5. Corrija o que quiser em **Lançamentos** e em **Categorias e regras**.

### 7. Comandos do dia a dia

| Para… | Rode |
|---|---|
| Parar o app (os dados ficam guardados) | `docker compose --profile app down` |
| Ligar de novo | `docker compose --profile app up -d` |
| Atualizar para a versão nova do código | `git pull` e depois `docker compose --profile app up -d --build` |
| Ver os logs (se algo der errado) | `docker compose --profile app logs -f app` |
| Aplicar uma mudança no `.env` | `docker compose --profile app up -d` |

Com `make` instalado (Mac/Linux) há atalhos: `make up`, `make down`, `make logs`.

O app sobe sozinho quando o Docker Desktop abre (a menos que você o tenha parado com `down`).

### 8. Problemas comuns

| Mensagem / sintoma | O que fazer |
|---|---|
| `port is already allocated` na 8080 | Outro programa usa a porta 8080. Feche-o, ou troque `"127.0.0.1:8080:8080"` por `"127.0.0.1:8090:8080"` no `docker-compose.yml` e abra http://localhost:8090 |
| `port is already allocated` na 3307 | Troque `"127.0.0.1:3307:3306"` por outra porta (ex.: `3308`) no `docker-compose.yml` |
| "O PDF tem senha e nenhuma senha foi configurada" / "nenhuma das senhas configuradas abriu o PDF" | Confira `ITAU_PDF_PASSWORD` no `.env` e rode `docker compose --profile app up -d` |
| "Nenhum banco reconheceu este arquivo" | Só PDFs de fatura de cartão e de extrato de conta do Itaú são aceitos |
| "conferência falhou" | O layout do PDF mudou ou é um caso novo; mande a mensagem de erro para quem mantém o projeto (o PDF fica guardado e não é salvo nada errado) |
| Nenhuma sugestão da IA nas perguntas | Confira `OPENAI_API_KEY` e se há crédito na OpenAI; veja os logs |
| Página não abre | Veja se o Docker Desktop está aberto e rode `docker compose --profile app ps` |

### Privacidade e segurança

- O app **não tem login**: ele só aceita acessos do próprio computador (`localhost`). Não
  exponha a porta na rede nem na internet.
- Os dados ficam em volumes do Docker (`mysql-data` com o banco e `app-data` com os PDFs
  importados). `docker compose --profile app down -v` **apaga tudo** — use só se quiser
  começar do zero.

---

## Desenvolvimento

Design: `docs/superpowers/specs/2026-09-26-finance-tracker-design.md`.

### Requisitos

- Go 1.26+ (se o Go instalado for mais antigo, ele baixa o 1.26 sozinho)
- Python 3 + PyMuPDF apenas para regenerar os PDFs sintéticos de teste

### Configuração

```bash
cp .env.example .env   # preencha ITAU_PDF_PASSWORD
```

### Conferir PDFs sem banco de dados

Coloque os PDFs em `samples/itau/` (pasta fora do git) e rode:

```bash
cd backend && go run ./cmd/inspect -v ../samples/itau/*.pdf
```

Cada arquivo mostra conta, datas, totais por tipo e, com `-v`, todas as transações. Se a
conferência falhar, a mensagem mostra a regra, o valor calculado, o impresso e a diferença.

### Rodar a API

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

### Rodar o app

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

### Testes

```bash
make test           # unitários + integração (os de banco são pulados sem o MySQL de `make db-up`)
make test-samples   # regressão com os PDFs reais de samples/
make web-test       # lint + Vitest do app web
```

### Backup (restic → Backblaze B2 ou Cloudflare R2)

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
