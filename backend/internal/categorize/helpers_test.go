package categorize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"jgfinance/internal/llm"
	"jgfinance/internal/store"
	"jgfinance/internal/store/storetest"
)

// fakeLLM answers categorization requests from a table keyed by merchant key.
type fakeLLM struct {
	items  map[string]map[string]any // key -> item fields (key is filled in)
	extra  []map[string]any          // items appended as-is (e.g. invented keys)
	interp map[string]any
	err    error
	reqs   []llm.Request
}

func (f *fakeLLM) JSON(_ context.Context, req llm.Request, out any) error {
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return f.err
	}
	var resp any
	switch req.SchemaName {
	case "categorizacao":
		var in struct {
			Keys []struct{ Key string } `json:"keys"`
		}
		if err := json.Unmarshal([]byte(req.User), &in); err != nil {
			return err
		}
		items := []map[string]any{}
		for _, k := range in.Keys {
			if it, ok := f.items[k.Key]; ok {
				full := map[string]any{"key": k.Key, "alias": "", "likely_subscription": false, "question": "", "suggestions": []int64{}, "certain": false}
				for name, v := range it {
					full[name] = v
				}
				items = append(items, full)
			}
		}
		items = append(items, f.extra...)
		resp = map[string]any{"items": items}
	case "interpretacao":
		resp = f.interp
	default:
		return errors.New("schema inesperado")
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

type fixture struct {
	t        *testing.T
	s        *store.Store
	card     int64
	checking int64
	doc      int64
	seq      int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s := storetest.New(t)
	f := &fixture{t: t, s: s}
	f.card = f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'credit_card', '8778', 'Cartão')`)
	f.checking = f.exec(`INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau', 'checking', '1/2', 'Conta')`)
	f.doc = f.exec(`INSERT INTO documents (account_id, kind, file_sha256, file_name, stored_path) VALUES (?, 'card_statement', 'x', 'f.pdf', 'p')`, f.card)
	return f
}

func (f *fixture) exec(q string, args ...any) int64 {
	f.t.Helper()
	r, err := f.s.DB.Exec(q, args...)
	if err != nil {
		f.t.Fatal(err)
	}
	id, _ := r.LastInsertId()
	return id
}

// tx inserts a spending transaction on the card (or a PIX on the checking account).
func (f *fixture) tx(key string, cents int64, person bool) int64 {
	f.t.Helper()
	f.seq++
	acc := f.card
	if person {
		acc = f.checking
	}
	kind := "expense"
	if cents < 0 {
		kind = "refund"
	}
	return f.exec(`INSERT INTO transactions (document_id, account_id, seq, occurred_on, reference_month, description,
		merchant_key, amount_cents, kind, person_transfer, dedupe_key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.doc, acc, f.seq, time.Date(2026, 2, f.seq%28+1, 0, 0, 0, 0, time.UTC), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		key+" DESC", key, cents, kind, person, fmt.Sprintf("dedupe-%d", f.seq))
}

// sub returns the id of the subcategory "cat › sub".
func (f *fixture) sub(cat, sub string) int64 {
	f.t.Helper()
	var id int64
	if err := f.s.DB.QueryRow(`SELECT s.id FROM categories s JOIN categories c ON c.id = s.parent_id WHERE c.name = ? AND s.name = ?`,
		cat, sub).Scan(&id); err != nil {
		f.t.Fatalf("sub %s › %s: %v", cat, sub, err)
	}
	return id
}

type txState struct {
	Kind       string
	CategoryID int64 // 0 = none
	By         string
	QuestionID int64
}

func (f *fixture) state(id int64) txState {
	f.t.Helper()
	var s txState
	var cat, q *int64
	var by *string
	if err := f.s.DB.QueryRow(`SELECT kind, category_id, categorized_by, question_id FROM transactions WHERE id = ?`, id).
		Scan(&s.Kind, &cat, &by, &q); err != nil {
		f.t.Fatal(err)
	}
	if cat != nil {
		s.CategoryID = *cat
	}
	if by != nil {
		s.By = *by
	}
	if q != nil {
		s.QuestionID = *q
	}
	return s
}

type questionState struct {
	Key         string
	PerTx       bool
	Status      string
	Suggestions []int64
	Prompt      string
	TxCount     int
}

func (f *fixture) question(id int64) questionState {
	f.t.Helper()
	var q questionState
	var sugg, prompt *string
	if err := f.s.DB.QueryRow(`SELECT merchant_key, per_transaction, status, suggestions, prompt,
		(SELECT COUNT(*) FROM transactions WHERE question_id = questions.id) FROM questions WHERE id = ?`, id).
		Scan(&q.Key, &q.PerTx, &q.Status, &sugg, &prompt, &q.TxCount); err != nil {
		f.t.Fatal(err)
	}
	if sugg != nil {
		_ = json.Unmarshal([]byte(*sugg), &q.Suggestions)
	}
	if prompt != nil {
		q.Prompt = *prompt
	}
	return q
}

type ruleState struct {
	ID           int64
	CategoryID   int64
	KindOverride string
	Mode, Source string
	Alias        string
	Recurring    string
}

func (f *fixture) rule(key string) (ruleState, bool) {
	f.t.Helper()
	var r ruleState
	var cat *int64
	var ko, alias *string
	err := f.s.DB.QueryRow(`SELECT id, category_id, kind_override, mode, source, alias, recurring FROM rules WHERE merchant_key = ?`, key).
		Scan(&r.ID, &cat, &ko, &r.Mode, &r.Source, &alias, &r.Recurring)
	if err != nil {
		return r, false
	}
	if cat != nil {
		r.CategoryID = *cat
	}
	if ko != nil {
		r.KindOverride = *ko
	}
	if alias != nil {
		r.Alias = *alias
	}
	return r, true
}

func (f *fixture) count(q string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.s.DB.QueryRow(q, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func run(t *testing.T, c *Categorizer, ids ...int64) Result {
	t.Helper()
	r, err := c.Run(context.Background(), ids)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
