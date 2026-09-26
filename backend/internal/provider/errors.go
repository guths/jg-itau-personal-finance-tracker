package provider

import (
	"errors"
	"fmt"

	"jgfinance/internal/money"
)

// ErrUnknownDocument is returned when no provider recognizes a document.
var ErrUnknownDocument = errors.New("nenhum banco reconheceu este arquivo")

// ReconciliationError means extracted amounts disagree with a total printed on the document.
type ReconciliationError struct {
	Rule     string
	Computed int64
	Printed  int64
}

func (e *ReconciliationError) Error() string {
	return fmt.Sprintf("conferência falhou (%s): calculado %s, impresso %s, diferença %s",
		e.Rule, money.FormatBRL(e.Computed), money.FormatBRL(e.Printed), money.FormatBRL(e.Computed-e.Printed))
}

// LayoutError means the document was recognized but its structure is not what the parser expects.
type LayoutError struct {
	Msg string
}

func (e *LayoutError) Error() string { return "layout inesperado: " + e.Msg }
