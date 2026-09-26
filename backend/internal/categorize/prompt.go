package categorize

import (
	"encoding/json"
	"strings"

	"jgfinance/internal/llm"
	"jgfinance/internal/money"
)

// llmItem is the LLM's answer for one key (spec §9.3).
type llmItem struct {
	Key                string  `json:"key"`
	SubcategoryID      int64   `json:"subcategory_id"`
	Certain            bool    `json:"certain"`
	Alias              string  `json:"alias"`
	LikelySubscription bool    `json:"likely_subscription"`
	Question           string  `json:"question"`
	Suggestions        []int64 `json:"suggestions"`
}

type llmBatch struct {
	Items []llmItem `json:"items"`
}

type ruleExample struct {
	Key      string `json:"key"`
	Category string `json:"category"`
	Alias    string `json:"alias,omitempty"`
}

// keyInput is what the LLM sees about one key: only descriptions, amounts, dates and the
// bank's category/city (spec §17) — never document, card or account numbers.
type keyInput struct {
	Key            string   `json:"key"`
	Descriptions   []string `json:"descriptions"`
	Count          int      `json:"count"`
	Total          string   `json:"total"`
	Dates          []string `json:"dates"`
	Source         string   `json:"source"`
	BankCategory   string   `json:"bank_category,omitempty"`
	City           string   `json:"city,omitempty"`
	PersonTransfer bool     `json:"pix_to_person"`
}

type subcategoryInput struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

const categorizeSystem = `Você categoriza gastos pessoais de um brasileiro a partir de lançamentos de fatura de cartão e de extrato de conta corrente.
Para cada chave (estabelecimento ou pessoa) recebida, escolha a subcategoria mais provável entre as subcategorias fornecidas (use somente os ids da lista).

Regras:
- A categoria do banco (bank_category) é só uma dica e costuma errar (por exemplo "outros"); não confie nela sozinha.
- Marque certain = true apenas quando o nome deixa a categoria inequívoca (ex.: "UBER" → Transporte › Aplicativo, "IFOOD" → Alimentação › Delivery).
- PIX para pessoa (pix_to_person = true), marketplaces sem dica específica (Mercado Livre, Amazon, Shopee, AliExpress, PicPay…) e nomes ambíguos → certain = false.
- alias: nome curto e legível do estabelecimento, como uma pessoa escreveria (ex.: "Komprão", "99", "iFood · Nagasaki Sushi").
- likely_subscription: true quando parece uma assinatura ou cobrança mensal (streaming, software, academia, plano).
- question: pergunta curta em pt-BR para o usuário quando você não tiver certeza (ex.: "Quem é Laura N? Para que foi este PIX?"); pode ser vazia quando certain = true.
- suggestions: até 3 ids de subcategoria, do mais para o menos provável.
- Os exemplos (examples) são regras que o usuário já definiu; use-os para entender o estilo dele.
- Responda com exatamente um item por chave recebida, repetindo a chave sem alterações.`

// categorizeRequest builds the batch categorization call for the given key groups.
func categorizeRequest(tax taxonomy, examples []ruleExample, groups [][]pendingTx) (llm.Request, error) {
	items := make([]keyInput, 0, len(groups))
	for _, g := range groups {
		in := keyInput{Key: g[0].Key, Count: len(g), BankCategory: g[0].BankCategory, City: g[0].City}
		var total int64
		seenDesc := map[string]bool{}
		for _, t := range g {
			total += t.AmountCents
			in.PersonTransfer = in.PersonTransfer || t.PersonTransfer
			if !seenDesc[t.Description] && len(in.Descriptions) < 3 {
				seenDesc[t.Description] = true
				in.Descriptions = append(in.Descriptions, t.Description)
			}
			if len(in.Dates) < 5 {
				in.Dates = append(in.Dates, t.Date.Format("2006-01-02"))
			}
		}
		in.Total = money.FormatBRL(total)
		in.Source = sourceOf(g[0])
		items = append(items, in)
	}
	subs := make([]subcategoryInput, len(tax.list))
	for i, s := range tax.list {
		subs[i] = subcategoryInput{ID: s.ID, Path: s.Path}
	}
	if examples == nil {
		examples = []ruleExample{}
	}
	user, err := json.MarshalIndent(map[string]any{
		"subcategories": subs,
		"examples":      examples,
		"keys":          items,
	}, "", " ")
	if err != nil {
		return llm.Request{}, err
	}
	return llm.Request{
		System:     categorizeSystem,
		User:       string(user),
		SchemaName: "categorizacao",
		Schema:     categorizeSchema(tax.ids()),
	}, nil
}

func sourceOf(t pendingTx) string {
	switch {
	case t.AccountKind == "credit_card":
		return "cartão de crédito"
	case t.PersonTransfer || strings.HasPrefix(t.Key, "PIX"):
		return "pix"
	}
	return "conta corrente"
}

// categorizeSchema is the strict JSON schema of spec §9.3.
func categorizeSchema(ids []int64) map[string]any {
	subID := map[string]any{"type": "integer", "enum": ids}
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"key":                 map[string]any{"type": "string"},
						"subcategory_id":      subID,
						"certain":             map[string]any{"type": "boolean"},
						"alias":               map[string]any{"type": "string"},
						"likely_subscription": map[string]any{"type": "boolean"},
						"question":            map[string]any{"type": "string"},
						"suggestions":         map[string]any{"type": "array", "items": subID},
					},
					"required":             []string{"key", "subcategory_id", "certain", "alias", "likely_subscription", "question", "suggestions"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"items"},
		"additionalProperties": false,
	}
}

// interpretation is the LLM's reading of a free-text answer.
type interpretation struct {
	Kind          string `json:"kind"`
	SubcategoryID *int64 `json:"subcategory_id"`
	Alias         string `json:"alias"`
}

const interpretSystem = `Você ajuda a categorizar gastos pessoais. O usuário respondeu em texto livre a uma pergunta sobre um lançamento.
Interprete a resposta:
- kind = "expense" quando é um gasto; escolha então subcategory_id entre as subcategorias fornecidas (use somente os ids da lista).
- kind = "own_transfer" quando é transferência para uma conta do próprio usuário; kind = "ignored" quando não deve contar como gasto (ex.: reembolso entre amigos que será devolvido). Nesses casos subcategory_id = null.
- Se não der para escolher uma subcategoria com segurança, use subcategory_id = null.
- alias: nome curto e legível para a chave (ex.: "Psicóloga Ana"), ou vazio.`

func interpretRequest(tax taxonomy, context map[string]any, text string) (llm.Request, error) {
	subs := make([]subcategoryInput, len(tax.list))
	for i, s := range tax.list {
		subs[i] = subcategoryInput{ID: s.ID, Path: s.Path}
	}
	user, err := json.MarshalIndent(map[string]any{
		"subcategories": subs,
		"question":      context,
		"answer":        text,
	}, "", " ")
	if err != nil {
		return llm.Request{}, err
	}
	enum := make([]any, 0, len(tax.list)+1)
	for _, id := range tax.ids() {
		enum = append(enum, id)
	}
	enum = append(enum, nil)
	return llm.Request{
		System:     interpretSystem,
		User:       string(user),
		SchemaName: "interpretacao",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":           map[string]any{"type": "string", "enum": []string{"expense", "own_transfer", "ignored"}},
				"subcategory_id": map[string]any{"type": []string{"integer", "null"}, "enum": enum},
				"alias":          map[string]any{"type": "string"},
			},
			"required":             []string{"kind", "subcategory_id", "alias"},
			"additionalProperties": false,
		},
	}, nil
}
