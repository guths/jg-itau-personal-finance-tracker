// Package categorize assigns subcategories to spending transactions: saved rules first, then
// the LLM, and finally questions for the user (spec §9).
package categorize

import (
	"errors"
	"sync"

	"jgfinance/internal/llm"
	"jgfinance/internal/store"
)

// Result counts what one categorization run did.
type Result struct {
	Auto      int // transactions categorized (or excluded) without asking
	Questions int // questions created
}

// Errors returned to the API layer (messages are shown to the user).
var (
	ErrNotFound        = errors.New("não encontrado")
	ErrAlreadyAnswered = errors.New("esta pergunta já foi respondida")
	ErrInvalidCategory = errors.New("escolha uma subcategoria ativa")
	ErrInvalidKind     = errors.New("tipo inválido")
	ErrInvalidScope    = errors.New("escopo inválido: use \"one\" ou \"key\"")
	ErrNoLLM           = errors.New("IA indisponível: configure OPENAI_API_KEY no .env")
	ErrInvalidRule     = errors.New("regra inválida")
	ErrEmptyText       = errors.New("escreva uma resposta para interpretar")
	// ErrLLMFailed wraps model failures (unavailable, timeout, invalid output).
	ErrLLMFailed = errors.New("a IA não respondeu direito; tente de novo em instantes")
)

// Categorizer runs the categorization pipeline and applies the user's answers.
type Categorizer struct {
	store *store.Store
	llm   llm.Client // nil = no AI: every item becomes a question without suggestions
	runMu sync.Mutex // serializes sweeps of pending transactions
}

// New returns a Categorizer; client may be nil.
func New(st *store.Store, client llm.Client) *Categorizer {
	return &Categorizer{store: st, llm: client}
}

// batchSize is the maximum number of keys sent to the LLM per call (spec §9.2).
const batchSize = 20

// llmConcurrency bounds how many batch calls run at the same time.
const llmConcurrency = 4
