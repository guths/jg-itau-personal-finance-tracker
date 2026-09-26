package categorize

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"jgfinance/internal/llm"
)

// slowLLM measures how many calls run at the same time before delegating to next.
type slowLLM struct {
	next     llm.Client
	delay    time.Duration
	mu       sync.Mutex
	inFlight int
	maxSeen  int
}

func (s *slowLLM) JSON(ctx context.Context, req llm.Request, out any) error {
	s.mu.Lock()
	s.inFlight++
	s.maxSeen = max(s.maxSeen, s.inFlight)
	s.mu.Unlock()
	time.Sleep(s.delay)
	s.mu.Lock()
	s.inFlight--
	s.mu.Unlock()
	return s.next.JSON(ctx, req, out)
}

// An interrupted run leaves spending without category, rule or question; the next run must
// pick it up even though it is not part of the new batch.
func TestRunSweepsOrphansLeftByInterruptedRun(t *testing.T) {
	f := newFixture(t)
	orphan := f.tx("LOJA ESQUECIDA", 1000, false)
	fresh := f.tx("LOJA NOVA", 2000, false)
	if r := run(t, New(f.s, nil), fresh); r != (Result{Questions: 2}) {
		t.Errorf("result = %+v", r)
	}
	if s := f.state(orphan); s.QuestionID == 0 {
		t.Errorf("orphan not swept: %+v", s)
	}
}

func TestSuggestRecoversOrphans(t *testing.T) {
	f := newFixture(t)
	online := f.sub("Compras", "Online")
	orphan := f.tx("AMAZON", 5000, false)
	fake := &fakeLLM{items: map[string]map[string]any{"AMAZON": {"subcategory_id": online, "certain": true}}}
	if _, err := New(f.s, fake).Suggest(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s := f.state(orphan); s.CategoryID != online || s.By != "llm" {
		t.Errorf("orphan = %+v", s)
	}

	// Without AI the orphan still reaches the question queue.
	other := f.tx("LOJA SEM IA", 100, false)
	if _, err := New(f.s, nil).Suggest(context.Background()); !errors.Is(err, ErrNoLLM) {
		t.Errorf("without LLM: %v", err)
	}
	if s := f.state(other); s.QuestionID == 0 {
		t.Errorf("orphan without LLM = %+v", s)
	}
}

func TestSuggestAsksBatchesConcurrently(t *testing.T) {
	f := newFixture(t)
	var ids []int64
	for i := range 45 {
		ids = append(ids, f.tx(fmt.Sprintf("LOJA %02d", i), 100, false))
	}
	run(t, New(f.s, nil), ids...) // questions without suggestions
	online := f.sub("Compras", "Online")
	items := map[string]map[string]any{}
	for i := range 45 {
		items[fmt.Sprintf("LOJA %02d", i)] = map[string]any{"subcategory_id": online, "certain": false}
	}
	slow := &slowLLM{next: &fakeLLM{items: items}, delay: 100 * time.Millisecond}
	n, err := New(f.s, slow).Suggest(context.Background())
	if err != nil || n != 45 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if slow.maxSeen < 2 || slow.maxSeen > llmConcurrency {
		t.Errorf("concurrent calls = %d, want 2..%d", slow.maxSeen, llmConcurrency)
	}
	if q := f.question(f.state(ids[44]).QuestionID); !reflect.DeepEqual(q.Suggestions, []int64{online}) {
		t.Errorf("question = %+v", q)
	}
}
