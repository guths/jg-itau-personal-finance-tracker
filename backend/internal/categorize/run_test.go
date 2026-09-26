package categorize

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSeededRuleApplies(t *testing.T) {
	f := newFixture(t)
	fake := &fakeLLM{}
	c := New(f.s, fake)
	id := f.tx("REPASSE DE IOF EM R$", 6842, false)
	if r := run(t, c, id); r != (Result{Auto: 1}) {
		t.Errorf("result = %+v", r)
	}
	if s := f.state(id); s.CategoryID != f.sub("Taxas", "IOF") || s.By != "rule" {
		t.Errorf("state = %+v", s)
	}
	if len(fake.reqs) != 0 {
		t.Error("LLM must not be called when every key has a rule")
	}
}

func TestRuleKindOverrideApplies(t *testing.T) {
	f := newFixture(t)
	f.exec(`INSERT INTO rules (merchant_key, kind_override, source) VALUES ('PIX TRANSF JONATHAN', 'own_transfer', 'user')`)
	id := f.tx("PIX TRANSF JONATHAN", 100000, true)
	run(t, New(f.s, nil), id)
	if s := f.state(id); s.Kind != "own_transfer" || s.CategoryID != 0 || s.By != "rule" {
		t.Errorf("state = %+v", s)
	}
}

func TestAskRuleCreatesOneQuestionPerTransaction(t *testing.T) {
	f := newFixture(t)
	therapy := f.sub("Saúde", "Terapia")
	f.exec(`INSERT INTO rules (merchant_key, category_id, mode, source) VALUES ('PIX TRANSF ANA', ?, 'ask', 'user')`, therapy)
	a, b := f.tx("PIX TRANSF ANA", 20000, true), f.tx("PIX TRANSF ANA", 5000, true)
	if r := run(t, New(f.s, &fakeLLM{}), a, b); r != (Result{Questions: 2}) {
		t.Errorf("result = %+v", r)
	}
	qa, qb := f.state(a).QuestionID, f.state(b).QuestionID
	if qa == 0 || qb == 0 || qa == qb {
		t.Fatalf("questions %d %d", qa, qb)
	}
	if q := f.question(qa); !q.PerTx || !reflect.DeepEqual(q.Suggestions, []int64{therapy}) || q.TxCount != 1 {
		t.Errorf("question = %+v", q)
	}
}

func TestCertainLLMAnswerAppliesAndCreatesRule(t *testing.T) {
	f := newFixture(t)
	app, streaming := f.sub("Transporte", "Aplicativo"), f.sub("Assinaturas", "Streaming")
	fake := &fakeLLM{items: map[string]map[string]any{
		"UBER TRIP":   {"subcategory_id": app, "certain": true, "alias": "Uber"},
		"NETFLIX.COM": {"subcategory_id": streaming, "certain": true, "alias": "Netflix", "likely_subscription": true},
	}}
	c := New(f.s, fake)
	u1, u2, n := f.tx("UBER TRIP", 2350, false), f.tx("UBER TRIP", 1800, false), f.tx("NETFLIX.COM", 5590, false)
	if r := run(t, c, u1, u2, n); r != (Result{Auto: 3}) {
		t.Errorf("result = %+v", r)
	}
	for _, id := range []int64{u1, u2} {
		if s := f.state(id); s.CategoryID != app || s.By != "llm" {
			t.Errorf("uber %d = %+v", id, s)
		}
	}
	if r, ok := f.rule("UBER TRIP"); !ok || r.Source != "llm" || r.Mode != "always" || r.Alias != "Uber" || r.CategoryID != app || r.Recurring != "auto" {
		t.Errorf("uber rule = %+v", r)
	}
	if r, _ := f.rule("NETFLIX.COM"); r.Recurring != "yes" {
		t.Errorf("netflix rule = %+v", r)
	}
	if len(fake.reqs) != 1 {
		t.Fatalf("calls = %d", len(fake.reqs))
	}
	req := fake.reqs[0]
	if !strings.Contains(req.User, `"UBER TRIP"`) || !strings.Contains(req.User, "Transporte › Aplicativo") ||
		!strings.Contains(req.User, "REPASSE DE IOF EM R$") || strings.Contains(req.User, "8778") {
		t.Errorf("user message missing keys/subcategories/examples or leaking the card number:\n%s", req.User)
	}
	if !strings.Contains(req.System, "certain") || req.Schema["additionalProperties"] != false {
		t.Error("system prompt / schema")
	}
}

func TestUncertainLLMAnswerCreatesGroupedQuestion(t *testing.T) {
	f := newFixture(t)
	online, casa, eletr, roupas := f.sub("Compras", "Online"), f.sub("Compras", "Casa"), f.sub("Compras", "Eletrônicos"), f.sub("Compras", "Roupas")
	fake := &fakeLLM{items: map[string]map[string]any{
		"MERCADOLIVRE": {"subcategory_id": online, "certain": false, "question": "O que você comprou no Mercado Livre?",
			"suggestions": []int64{casa, 999999, eletr, roupas}},
	}}
	ids := []int64{f.tx("MERCADOLIVRE", 10000, false), f.tx("MERCADOLIVRE", 2000, false), f.tx("MERCADOLIVRE", 3000, false)}
	if r := run(t, New(f.s, fake), ids...); r != (Result{Questions: 1}) {
		t.Errorf("result = %+v", r)
	}
	qid := f.state(ids[0]).QuestionID
	q := f.question(qid)
	if q.PerTx || q.TxCount != 3 || q.Prompt != "O que você comprou no Mercado Livre?" ||
		!reflect.DeepEqual(q.Suggestions, []int64{online, casa, eletr}) {
		t.Errorf("question = %+v", q)
	}
	if _, ok := f.rule("MERCADOLIVRE"); ok {
		t.Error("uncertain answers must not create rules")
	}
}

func TestPersonTransferAlwaysAsks(t *testing.T) {
	f := newFixture(t)
	therapy := f.sub("Saúde", "Terapia")
	fake := &fakeLLM{items: map[string]map[string]any{"PIX TRANSF LAURA N": {"subcategory_id": therapy, "certain": true}}}
	id := f.tx("PIX TRANSF LAURA N", 25000, true)
	if r := run(t, New(f.s, fake), id); r != (Result{Questions: 1}) {
		t.Errorf("result = %+v", r)
	}
	if q := f.question(f.state(id).QuestionID); !reflect.DeepEqual(q.Suggestions, []int64{therapy}) {
		t.Errorf("question = %+v", q)
	}
	if !strings.Contains(fake.reqs[0].User, `"pix_to_person": true`) {
		t.Error("the LLM must know it is a PIX to a person")
	}
}

func TestWithoutLLMEveryKeyBecomesQuestionWithoutSuggestions(t *testing.T) {
	f := newFixture(t)
	a, b, c := f.tx("PADARIA", 1000, false), f.tx("PADARIA", 900, false), f.tx("FARMACIA", 3000, false)
	if r := run(t, New(f.s, nil), a, b, c); r != (Result{Questions: 2}) {
		t.Errorf("result = %+v", r)
	}
	q := f.question(f.state(a).QuestionID)
	if q.TxCount != 2 || q.Suggestions != nil || q.Prompt != "" {
		t.Errorf("question = %+v", q)
	}
}

func TestBadLLMOutputsBecomeQuestionsWithoutSuggestions(t *testing.T) {
	f := newFixture(t)
	valid := f.sub("Alimentação", "Supermercado")
	fake := &fakeLLM{
		items: map[string]map[string]any{
			"INVALID ID": {"subcategory_id": 424242, "certain": true, "suggestions": []int64{535353}},
			"GOOD":       {"subcategory_id": valid, "certain": true},
		},
		extra: []map[string]any{
			{"key": "INVENTED", "subcategory_id": valid, "certain": true, "alias": "", "likely_subscription": false, "question": "", "suggestions": []int64{}},
			{"key": "GOOD", "subcategory_id": f.sub("Outros", "Outros"), "certain": true, "alias": "", "likely_subscription": false, "question": "", "suggestions": []int64{}},
		},
	}
	omitted, invalid, good := f.tx("OMITTED", 100, false), f.tx("INVALID ID", 200, false), f.tx("GOOD", 300, false)
	if r := run(t, New(f.s, fake), omitted, invalid, good); r != (Result{Auto: 1, Questions: 2}) {
		t.Errorf("result = %+v", r)
	}
	for _, id := range []int64{omitted, invalid} {
		if q := f.question(f.state(id).QuestionID); q.Suggestions != nil {
			t.Errorf("tx %d question = %+v", id, q)
		}
	}
	if s := f.state(good); s.CategoryID != valid {
		t.Errorf("the first answer for a key wins: %+v", s)
	}
	if _, ok := f.rule("INVENTED"); ok {
		t.Error("invented keys must be ignored")
	}

	fake.err = errors.New("timeout")
	down := f.tx("DOWN", 100, false)
	if r := run(t, New(f.s, fake), down); r != (Result{Questions: 1}) {
		t.Errorf("LLM error: %+v", r)
	}
}

func TestLLMBatchesOf40Keys(t *testing.T) {
	f := newFixture(t)
	fake := &fakeLLM{}
	var ids []int64
	for i := range 45 {
		ids = append(ids, f.tx(fmt.Sprintf("LOJA %02d", i), 100, false))
	}
	if r := run(t, New(f.s, fake), ids...); r.Questions != 45 {
		t.Errorf("result = %+v", r)
	}
	if len(fake.reqs) != 2 || strings.Count(fake.reqs[0].User, `"key": "LOJA`) != 40 || strings.Count(fake.reqs[1].User, `"key": "LOJA`) != 5 {
		t.Errorf("calls = %d", len(fake.reqs))
	}
}

func TestOnlyUncategorizedSpendingIsConsidered(t *testing.T) {
	f := newFixture(t)
	income := f.exec(`INSERT INTO transactions (document_id, account_id, seq, occurred_on, reference_month, description,
		merchant_key, amount_cents, kind, dedupe_key) VALUES (?, ?, 99, '2026-02-01', '2026-02-01', 'SALARIO', 'SALARIO', -500000, 'income', 'inc')`, f.doc, f.checking)
	done := f.tx("FEITO", 100, false)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'user' WHERE id = ?`, f.sub("Outros", "Outros"), done)
	if r := run(t, New(f.s, nil), income, done); r != (Result{}) {
		t.Errorf("result = %+v", r)
	}
}

func TestNewTransactionJoinsOpenQuestionOfSameKey(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	first := f.tx("LOJA", 100, false)
	run(t, c, first)
	second := f.tx("LOJA", 200, false)
	if r := run(t, c, second); r != (Result{}) {
		t.Errorf("result = %+v", r)
	}
	if f.state(second).QuestionID != f.state(first).QuestionID {
		t.Error("must join the existing question")
	}
}
