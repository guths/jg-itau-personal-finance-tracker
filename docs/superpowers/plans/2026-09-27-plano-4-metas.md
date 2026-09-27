# Plano 4 — Metas de redução de gastos, alavancas e simulador

Pedido do usuário (2026-09-27): "algo que eu coloque a meta (ex.: baixar 15%) e ele me ajude, me dê
insights para diminuir em áreas, gráficos novos". Design aprovado em conversa (pacote 1–5).
Tem faturas só de agosto/2026 em diante; julho e junho só têm extrato.

Spec vinculante: `docs/superpowers/specs/2026-09-26-finance-tracker-design.md` (atualizar com uma
seção "Metas" ao final). Convenções: centavos int64, datas UTC, pt-BR na UI, só `expense`+`refund`
contam como gasto, `reference_month` define o mês, modo privado deve mascarar todo valor novo.

## 1. Base e meta (backend)
- Migration: `goal_settings` (linha única: `reduction_pct` INT 1–90, `baseline_mode` ENUM('auto','manual'),
  `baseline_cents` BIGINT NULL, `updated_at`) e `category_budgets` (`category_id` = categoria de topo,
  `limit_cents` BIGINT, `updated_at`; ausência = usar o sugerido).
- **Mês completo**: para cada conta de cartão, a união dos intervalos (fechamento anterior, fechamento]
  das faturas importadas cobre o mês inteiro (primeira fatura de um cartão: início = fechamento − 1 mês);
  para cada conta corrente, a união dos períodos dos extratos cobre o mês inteiro. Contas sem nenhum
  documento não entram. Mês corrente (ainda não terminado) nunca é completo.
- **Base automática** do mês M = média dos totais (gasto) dos até 3 meses completos anteriores a M, e
  média por categoria de topo nos mesmos meses. Sem meses completos → base ausente (a UI pede valor manual).
  Modo manual: `baseline_cents` informado; por categoria usa a proporção média dos meses com dados.
- **Meta** = base × (1 − pct/100).
- **Categorias fixas × variáveis**: fixa = categoria cujo gasto nos meses base é ≥ 70 % recorrente
  (detecção de recorrência existente) ou "Moradia"/"Taxas"; o resto é variável.
- **Orçamento sugerido**: fixas mantêm a base; o corte total (base − meta) é distribuído entre as variáveis
  proporcionalmente à base de cada uma (nenhuma abaixo de 0). O usuário pode sobrescrever o limite por categoria.
- **Progresso do mês M**: gasto até agora; dias decorridos/total (mês passado = completo); projeção
  = gasto/dias decorridos × dias do mês (mês corrente) ou o próprio total; quanto ainda pode gastar por dia
  até o fim do mês para bater a meta; série diária acumulada (queima) + linha da meta.
- Por categoria: base, sugerido, limite efetivo, gasto, % do limite, status `ok` (<80 %), `atencao` (80–100 %), `estourou` (>100 %).
- API: `GET /api/goals?month=AAAA-MM` (tudo acima + meses usados na base e meses completos),
  `PUT /api/goals` `{reduction_pct, baseline_mode, baseline_cents?}`, `PUT /api/goals/budgets`
  `{budgets: [{category_id, limit_cents|null}]}`.

## 2. Alavancas (backend, determinístico)
`GET /api/insights/levers?month=` com:
- **Hábitos** (frequência × ticket): por chave/apelido no mês — quantidade, ticket médio, total, categoria,
  e economia de reduzir pela metade as visitas (só categorias discricionárias: Alimentação › Restaurante,
  Delivery, Padaria e café; Lazer e esporte; Compras; Transporte › Aplicativo; Cuidados pessoais).
- **Vazamento de pequenas compras**: quantidade e total de gastos < R$ 30 no mês.
- **Picos**: categorias com gasto no mês > 20 % acima da base.
- **Recorrentes que aumentaram** e lista de assinaturas (reusar insights de recorrência).
- **Dia da semana**: total e quantidade por dia.
- **Pareto**: estabelecimentos ordenados com % acumulado.
Cada alavanca com `estimated_savings_cents` quando fizer sentido; lista ordenada por economia.

## 3. Conselho do GPT
- `POST /api/insights/advice?month=` → envia só agregados (categorias, base, meta, top hábitos com
  apelidos, picos; nada de descrições brutas de PIX de pessoa) e recebe JSON strict
  `{summary, suggestions:[{title, detail, category, estimated_savings_cents}]}` (3–5 sugestões, pt-BR,
  práticas e específicas). Cache em tabela `advice` (month, content JSON, created_at); `GET` devolve o cache;
  botão "Gerar de novo". Sem chave OpenAI → mensagem amigável; a página funciona sem isso.

## 4. Web — página "Metas" (nova entrada na barra lateral)
- Configuração: meta em % (padrão 15), base (automática com os meses usados, ou manual em R$), aviso quando
  não houver meses completos suficientes.
- Card de progresso: gasto × meta, projeção, "pode gastar R$ X/dia", status colorido.
- Gráfico de queima (acumulado diário × linha da meta).
- Orçamentos por categoria: barras de consumo com limite, sugerido × editado, alerta em 80 % e 100 %,
  fixas marcadas; edição do limite inline.
- Alavancas: cards ordenados por economia estimada.
- Conselho do GPT: card com resumo + sugestões, gerar/regerar.
- Gráficos: Pareto, dispersão frequência × ticket médio (tamanho = total), gasto por dia da semana.
- Simulador "e se": controles deslizantes por hábito (visitas por mês) e por categoria variável (% de corte)
  → economia projetada e nova projeção vs meta, tudo no cliente.
- Dashboard: card compacto "Meta do mês" com progresso e link para Metas; alerta quando alguma categoria estourar.
- Tudo mascarado no modo privado; responsivo; estados vazios.

## Testes
Backend TDD: completude de mês (cartão com lacuna, extrato parcial, mês corrente), base automática/manual,
distribuição do corte (fixas intocadas, soma = meta), progresso/projeção/diária, status por categoria,
alavancas (hábitos, vazamento, picos, pareto), handlers (validação 422), advice com LLM falso.
Web: vitest para cálculos do simulador e formatação; build/lint.
