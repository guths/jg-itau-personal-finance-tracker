package categorize

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func questionFor(t *testing.T, f *fixture, c *Categorizer, key string, n int) (int64, []int64) {
	t.Helper()
	var ids []int64
	for range n {
		ids = append(ids, f.tx(key, 1000, strings.HasPrefix(key, "PIX")))
	}
	run(t, c, ids...)
	return f.state(ids[0]).QuestionID, ids
}

func TestAnswerRememberCreatesAlwaysRuleAndCategorizesAll(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	therapy := f.sub("Saúde", "Terapia")
	qid, ids := questionFor(t, f, c, "PIX TRANSF LAURA N", 2)
	if err := c.Answer(context.Background(), qid, Answer{CategoryID: &therapy, Remember: true, Alias: "Psicóloga Laura"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if s := f.state(id); s.CategoryID != therapy || s.By != "user" || s.Kind != "expense" {
			t.Errorf("tx %d = %+v", id, s)
		}
	}
	if r, _ := f.rule("PIX TRANSF LAURA N"); r.Mode != "always" || r.Source != "user" || r.Alias != "Psicóloga Laura" || r.CategoryID != therapy {
		t.Errorf("rule = %+v", r)
	}
	if q := f.question(qid); q.Status != "answered" {
		t.Errorf("question = %+v", q)
	}
	// Next month the rule applies without asking.
	next := f.tx("PIX TRANSF LAURA N", 25000, true)
	if r := run(t, c, next); r != (Result{Auto: 1}) {
		t.Errorf("next month = %+v", r)
	}
	if err := c.Answer(context.Background(), qid, Answer{CategoryID: &therapy}); !errors.Is(err, ErrAlreadyAnswered) {
		t.Errorf("answering twice: %v", err)
	}
}

func TestAnswerWithoutRememberCreatesAskRule(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	gift := f.sub("Pessoas e presentes", "Presentes")
	qid, _ := questionFor(t, f, c, "PIX TRANSF ANTONIO", 1)
	if err := c.Answer(context.Background(), qid, Answer{CategoryID: &gift}); err != nil {
		t.Fatal(err)
	}
	if r, _ := f.rule("PIX TRANSF ANTONIO"); r.Mode != "ask" || r.Source != "user" || r.CategoryID != gift {
		t.Errorf("rule = %+v", r)
	}
	next := f.tx("PIX TRANSF ANTONIO", 5000, true)
	run(t, c, next)
	q := f.question(f.state(next).QuestionID)
	if !q.PerTx || !reflect.DeepEqual(q.Suggestions, []int64{gift}) {
		t.Errorf("next question = %+v", q)
	}
}

func TestAnswerOwnTransferChangesKind(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	qid, ids := questionFor(t, f, c, "PIX TRANSF JONATHAN", 2)
	if err := c.Answer(context.Background(), qid, Answer{Kind: "own_transfer", Remember: true}); err != nil {
		t.Fatal(err)
	}
	if s := f.state(ids[1]); s.Kind != "own_transfer" || s.CategoryID != 0 || s.By != "user" {
		t.Errorf("tx = %+v", s)
	}
	if r, _ := f.rule("PIX TRANSF JONATHAN"); r.KindOverride != "own_transfer" || r.CategoryID != 0 {
		t.Errorf("rule = %+v", r)
	}
}

func TestAnswerRememberResolvesOtherOpenQuestionsOfKey(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	f.exec(`INSERT INTO rules (merchant_key, mode, category_id, source) VALUES ('PIX TRANSF MAE', 'ask', ?, 'user')`, f.sub("Pessoas e presentes", "Ajuda familiar"))
	a, b := f.tx("PIX TRANSF MAE", 100, true), f.tx("PIX TRANSF MAE", 200, true)
	run(t, c, a, b)
	help := f.sub("Pessoas e presentes", "Ajuda familiar")
	if err := c.Answer(context.Background(), f.state(a).QuestionID, Answer{CategoryID: &help, Remember: true}); err != nil {
		t.Fatal(err)
	}
	if s := f.state(b); s.CategoryID != help || s.By != "rule" || f.question(s.QuestionID).Status != "answered" {
		t.Errorf("sibling = %+v", s)
	}
}

func TestAnswerValidation(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	qid, _ := questionFor(t, f, c, "LOJA", 1)
	var topLevel int64
	if err := f.s.DB.QueryRow(`SELECT id FROM categories WHERE parent_id IS NULL LIMIT 1`).Scan(&topLevel); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cases := map[string]struct {
		id   int64
		a    Answer
		want error
	}{
		"not found":    {999999, Answer{CategoryID: ptr(f.sub("Outros", "Outros"))}, ErrNotFound},
		"top category": {qid, Answer{CategoryID: &topLevel}, ErrInvalidCategory},
		"no category":  {qid, Answer{}, ErrInvalidCategory},
		"bad kind":     {qid, Answer{Kind: "income"}, ErrInvalidKind},
	}
	for name, tc := range cases {
		if err := c.Answer(ctx, tc.id, tc.a); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	f.exec(`UPDATE categories SET archived = TRUE WHERE id = ?`, f.sub("Compras", "Casa"))
	if err := c.Answer(ctx, qid, Answer{CategoryID: ptr(f.sub("Compras", "Casa"))}); !errors.Is(err, ErrInvalidCategory) {
		t.Errorf("archived: %v", err)
	}
	if q := f.question(qid); q.Status != "pending" {
		t.Errorf("failed answers must not change the question: %+v", q)
	}
}

func TestSkip(t *testing.T) {
	f := newFixture(t)
	c := New(f.s, nil)
	qid, _ := questionFor(t, f, c, "LOJA", 1)
	ctx := context.Background()
	if err := c.Skip(ctx, qid); err != nil {
		t.Fatal(err)
	}
	if f.question(qid).Status != "skipped" {
		t.Error("not skipped")
	}
	if err := c.Skip(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("not found: %v", err)
	}
	// A skipped question can still be answered.
	if err := c.Answer(ctx, qid, Answer{CategoryID: ptr(f.sub("Outros", "Outros"))}); err != nil {
		t.Fatal(err)
	}
	if err := c.Skip(ctx, qid); !errors.Is(err, ErrAlreadyAnswered) {
		t.Errorf("skip answered: %v", err)
	}
}

func TestInterpret(t *testing.T) {
	f := newFixture(t)
	therapy := f.sub("Saúde", "Terapia")
	fake := &fakeLLM{interp: map[string]any{"kind": "expense", "subcategory_id": therapy, "alias": "Psicóloga"}}
	c := New(f.s, fake)
	qid, _ := questionFor(t, f, c, "PIX TRANSF LAURA N", 1)
	got, err := c.Interpret(context.Background(), qid, "é a psicóloga")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "expense" || got.CategoryID == nil || *got.CategoryID != therapy || got.Alias != "Psicóloga" {
		t.Errorf("interpretation = %+v", got)
	}
	req := fake.reqs[len(fake.reqs)-1]
	if req.SchemaName != "interpretacao" || !strings.Contains(req.User, "é a psicóloga") || !strings.Contains(req.User, "PIX TRANSF LAURA N") {
		t.Errorf("request = %+v", req)
	}
	if f.question(qid).Status != "pending" {
		t.Error("interpret must not save anything")
	}

	fake.interp = map[string]any{"kind": "expense", "subcategory_id": 987654, "alias": ""}
	if got, _ := c.Interpret(context.Background(), qid, "sei lá"); got.CategoryID != nil {
		t.Errorf("invalid id must be dropped: %+v", got)
	}
	fake.interp = map[string]any{"kind": "own_transfer", "subcategory_id": nil, "alias": ""}
	if got, _ := c.Interpret(context.Background(), qid, "é minha conta"); got.Kind != "own_transfer" {
		t.Errorf("own transfer: %+v", got)
	}
	if _, err := New(f.s, nil).Interpret(context.Background(), qid, "x"); !errors.Is(err, ErrNoLLM) {
		t.Errorf("without LLM: %v", err)
	}
}

func TestSuggestFillsQuestionsWithoutSuggestions(t *testing.T) {
	f := newFixture(t)
	qid, _ := questionFor(t, f, New(f.s, nil), "MERCADOLIVRE", 2)
	online := f.sub("Compras", "Online")
	fake := &fakeLLM{items: map[string]map[string]any{
		"MERCADOLIVRE": {"subcategory_id": online, "certain": true, "question": "O que foi?"},
	}}
	c := New(f.s, fake)
	n, err := c.Suggest(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	q := f.question(qid)
	if !reflect.DeepEqual(q.Suggestions, []int64{online}) || q.Prompt != "O que foi?" || q.Status != "pending" {
		t.Errorf("question = %+v", q)
	}
	if n, _ := c.Suggest(context.Background()); n != 0 {
		t.Errorf("second run = %d", n)
	}
	if _, err := New(f.s, nil).Suggest(context.Background()); !errors.Is(err, ErrNoLLM) {
		t.Errorf("without LLM: %v", err)
	}
}
