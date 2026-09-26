// Package llm is the minimal contract with a language model: produce JSON that obeys a JSON
// schema (spec §9.3). The OpenAI implementation uses Structured Outputs in strict mode; tests
// use fakes, so no test ever reaches the real API.
package llm

import (
	"context"
	"errors"
)

// Client generates JSON obeying req.Schema and decodes it into out.
type Client interface {
	JSON(ctx context.Context, req Request, out any) error
}

// Request is one structured-output call.
type Request struct {
	System, User string
	SchemaName   string
	Schema       map[string]any
}

// ErrInvalidOutput means the model answered with something that is not the requested JSON
// (refusal, truncated output, invalid JSON).
var ErrInvalidOutput = errors.New("llm: resposta inválida do modelo")

// FromConfig returns the OpenAI client, or a nil Client when there is no API key (the app then
// runs without AI: every item becomes a question without suggestions).
func FromConfig(apiKey, model string) Client {
	if apiKey == "" {
		return nil
	}
	return NewOpenAI(apiKey, model)
}
