package categorize

import (
	"context"
	"errors"
	"testing"
)

func TestRuleCorrectionReappliesToRuleAndLLMButNotUser(t *testing.T) {
	f := newFixture(t)
	wrong, right := f.sub("Alimentação", "Restaurante"), f.sub("Alimentação", "Padaria e café")
	ruleID := f.exec(`INSERT INTO rules (merchant_key, category_id, source) VALUES ('CAFE DO PONTO', ?, 'llm')`, wrong)
	byRule, byLLM, byUser := f.tx("CAFE DO PONTO", 100, false), f.tx("CAFE DO PONTO", 200, false), f.tx("CAFE DO PONTO", 300, false)
	manual := f.sub("Lazer e esporte", "Bares e eventos")
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'rule' WHERE id = ?`, wrong, byRule)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'llm' WHERE id = ?`, wrong, byLLM)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'user' WHERE id = ?`, manual, byUser)

	c := New(f.s, nil)
	if err := c.UpdateRule(context.Background(), ruleID, RuleUpdate{CategoryID: &right, Alias: ptr("Café do Ponto"), Recurring: ptr("no")}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{byRule, byLLM} {
		if s := f.state(id); s.CategoryID != right || s.By != "rule" {
			t.Errorf("tx %d = %+v", id, s)
		}
	}
	if s := f.state(byUser); s.CategoryID != manual || s.By != "user" {
		t.Errorf("manual correction overwritten: %+v", s)
	}
	if r, _ := f.rule("CAFE DO PONTO"); r.Source != "user" || r.CategoryID != right || r.Alias != "Café do Ponto" || r.Recurring != "no" {
		t.Errorf("rule = %+v", r)
	}

	if err := c.UpdateRule(context.Background(), ruleID, RuleUpdate{KindOverride: ptr("ignored")}); err != nil {
		t.Fatal(err)
	}
	if s := f.state(byLLM); s.Kind != "ignored" || s.CategoryID != 0 {
		t.Errorf("override not reapplied: %+v", s)
	}
	if err := c.UpdateRule(context.Background(), ruleID, RuleUpdate{CategoryID: &right}); err != nil {
		t.Fatal(err)
	}
	if s := f.state(byLLM); s.Kind != "expense" || s.CategoryID != right {
		t.Errorf("back to spending: %+v", s)
	}

	ctx := context.Background()
	if err := c.UpdateRule(ctx, ruleID, RuleUpdate{Mode: ptr("never")}); !errors.Is(err, ErrInvalidRule) {
		t.Errorf("bad mode: %v", err)
	}
	if err := c.UpdateRule(ctx, 999999, RuleUpdate{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	if err := c.DeleteRule(ctx, ruleID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteRule(ctx, ruleID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
	if s := f.state(byLLM); s.CategoryID != right {
		t.Errorf("deleting a rule must keep categories: %+v", s)
	}
}

func TestSetTransactionCategoryScopeOne(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	a, b := f.tx("LOJA", 100, false), f.tx("LOJA", 200, false)
	casa := f.sub("Compras", "Casa")
	ctx := context.Background()
	if err := c.SetTransactionCategory(ctx, a, &casa, "", "one"); err != nil {
		t.Fatal(err)
	}
	if s := f.state(a); s.CategoryID != casa || s.By != "user" {
		t.Errorf("a = %+v", s)
	}
	if s := f.state(b); s.CategoryID != 0 {
		t.Errorf("b changed: %+v", s)
	}
	if _, ok := f.rule("LOJA"); ok {
		t.Error("scope one must not create a rule")
	}
	if err := c.SetTransactionCategory(ctx, b, nil, "ignored", "one"); err != nil {
		t.Fatal(err)
	}
	if s := f.state(b); s.Kind != "ignored" || s.By != "user" {
		t.Errorf("b = %+v", s)
	}
	if err := c.SetTransactionCategory(ctx, a, &casa, "", "all"); !errors.Is(err, ErrInvalidScope) {
		t.Errorf("scope: %v", err)
	}
	if err := c.SetTransactionCategory(ctx, 999999, &casa, "", "one"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: %v", err)
	}
	if err := c.SetTransactionCategory(ctx, a, nil, "banana", "one"); !errors.Is(err, ErrInvalidKind) {
		t.Errorf("kind: %v", err)
	}
}

func TestSetTransactionCategoryScopeKey(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	restaurant, bakery, bar := f.sub("Alimentação", "Restaurante"), f.sub("Alimentação", "Padaria e café"), f.sub("Lazer e esporte", "Bares e eventos")
	edited, byLLM, byUser, pending := f.tx("KOMPRAO", 100, false), f.tx("KOMPRAO", 200, false), f.tx("KOMPRAO", 300, false), f.tx("KOMPRAO", 400, false)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'llm' WHERE id IN (?, ?)`, restaurant, edited, byLLM)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'user' WHERE id = ?`, bar, byUser)
	f.exec(`INSERT INTO rules (merchant_key, category_id, alias, source) VALUES ('KOMPRAO', ?, 'Komprão', 'llm')`, restaurant)

	if err := c.SetTransactionCategory(context.Background(), edited, &bakery, "", "key"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{edited, byLLM} {
		if s := f.state(id); s.CategoryID != bakery {
			t.Errorf("tx %d = %+v", id, s)
		}
	}
	if s := f.state(byUser); s.CategoryID != bar {
		t.Errorf("user tx overwritten: %+v", s)
	}
	if s := f.state(pending); s.CategoryID != 0 {
		t.Errorf("uncategorized tx changed: %+v", s)
	}
	if r, _ := f.rule("KOMPRAO"); r.Source != "user" || r.Mode != "always" || r.CategoryID != bakery || r.Alias != "Komprão" {
		t.Errorf("rule = %+v", r)
	}
	if err := c.SetTransactionCategory(context.Background(), edited, nil, "income", "key"); !errors.Is(err, ErrInvalidKind) {
		t.Errorf("income rule: %v", err)
	}
}

func TestSetTransactionCategoryScopeKeyResolvesOpenQuestions(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	bakery := f.sub("Alimentação", "Padaria e café")
	done := f.tx("KOMPRAO", 100, false)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'llm' WHERE id = ?`, f.sub("Alimentação", "Restaurante"), done)
	qid, waiting := questionFor(t, f, c, "KOMPRAO", 2)

	if err := c.SetTransactionCategory(context.Background(), done, &bakery, "", "key"); err != nil {
		t.Fatal(err)
	}
	for _, id := range waiting {
		if s := f.state(id); s.CategoryID != bakery || s.By != "rule" {
			t.Errorf("tx %d = %+v", id, s)
		}
	}
	if q := f.question(qid); q.Status != "answered" {
		t.Errorf("question = %+v", q)
	}
}

func TestSetTransactionCategoryScopeKeyKeepsRuleMode(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	therapy, other := f.sub("Saúde", "Terapia"), f.sub("Saúde", "Farmácia")
	f.exec(`INSERT INTO rules (merchant_key, category_id, mode, source) VALUES ('PIX TRANSF ANA', ?, 'ask', 'user')`, therapy)
	id := f.tx("PIX TRANSF ANA", 20000, true)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'user' WHERE id = ?`, therapy, id)
	if err := c.SetTransactionCategory(context.Background(), id, &other, "", "key"); err != nil {
		t.Fatal(err)
	}
	if r, _ := f.rule("PIX TRANSF ANA"); r.Mode != "ask" || r.CategoryID != other {
		t.Errorf("rule = %+v", r)
	}
	if _, ok := f.rule("NOVA CHAVE"); ok {
		t.Fatal("unexpected rule")
	}
	fresh := f.tx("NOVA CHAVE", 100, false)
	f.exec(`UPDATE transactions SET category_id = ?, categorized_by = 'user' WHERE id = ?`, therapy, fresh)
	if err := c.SetTransactionCategory(context.Background(), fresh, &other, "", "key"); err != nil {
		t.Fatal(err)
	}
	if r, _ := f.rule("NOVA CHAVE"); r.Mode != "always" {
		t.Errorf("new rule = %+v", r)
	}
}

func TestUpdateRuleAlwaysResolvesOpenQuestions(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	therapy := f.sub("Saúde", "Terapia")
	ruleID := f.exec(`INSERT INTO rules (merchant_key, category_id, mode, source) VALUES ('PIX TRANSF ANA', ?, 'ask', 'user')`, therapy)
	id := f.tx("PIX TRANSF ANA", 20000, true)
	run(t, c, id)
	qid := f.state(id).QuestionID
	ctx := context.Background()
	if err := c.UpdateRule(ctx, ruleID, RuleUpdate{Alias: ptr("Ana")}); err != nil {
		t.Fatal(err)
	}
	if q := f.question(qid); q.Status != "pending" {
		t.Errorf("mode ask must keep asking: %+v", q)
	}
	if err := c.UpdateRule(ctx, ruleID, RuleUpdate{Mode: ptr("always")}); err != nil {
		t.Fatal(err)
	}
	if s := f.state(id); s.CategoryID != therapy || s.By != "rule" {
		t.Errorf("tx = %+v", s)
	}
	if q := f.question(qid); q.Status != "answered" {
		t.Errorf("question = %+v", q)
	}
}

func TestSetTransactionCategoryScopeOneLeavesOpenQuestion(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	casa := f.sub("Compras", "Casa")
	qid, ids := questionFor(t, f, c, "LOJA", 2)
	ctx := context.Background()
	if err := c.SetTransactionCategory(ctx, ids[0], &casa, "", "one"); err != nil {
		t.Fatal(err)
	}
	if s := f.state(ids[0]); s.QuestionID != 0 || s.CategoryID != casa || s.By != "user" {
		t.Errorf("tx = %+v", s)
	}
	if q := f.question(qid); q.Status != "pending" || q.TxCount != 1 {
		t.Errorf("question with one left = %+v", q)
	}
	if err := c.SetTransactionCategory(ctx, ids[1], nil, "ignored", "one"); err != nil {
		t.Fatal(err)
	}
	if s := f.state(ids[1]); s.QuestionID != 0 || s.Kind != "ignored" {
		t.Errorf("tx = %+v", s)
	}
	if q := f.question(qid); q.Status != "answered" || q.TxCount != 0 {
		t.Errorf("empty question = %+v", q)
	}
}
