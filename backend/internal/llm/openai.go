package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// Timeout bounds each call, retries included (spec §9.3).
const Timeout = 3 * time.Minute

// OpenAI implements Client with the official SDK (Chat Completions + json_schema, strict).
type OpenAI struct {
	client openai.Client
	model  string
}

var _ Client = (*OpenAI)(nil)

// NewOpenAI returns a client for model authenticated with apiKey. Extra options (e.g. a base
// URL pointing to a local test server) are applied last.
func NewOpenAI(apiKey, model string, opts ...option.RequestOption) *OpenAI {
	base := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithRequestTimeout(Timeout),
		option.WithMaxRetries(1),
	}
	return &OpenAI{client: openai.NewClient(append(base, opts...)...), model: model}
}

// JSON implements Client.
func (o *OpenAI) JSON(ctx context.Context, req Request, out any) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	resp, err := o.client.Chat.Completions.New(ctx, params(o.model, req))
	if err != nil {
		return fmt.Errorf("llm: chamada ao modelo falhou: %w", err)
	}
	if len(resp.Choices) == 0 {
		return fmt.Errorf("%w: nenhuma escolha", ErrInvalidOutput)
	}
	c := resp.Choices[0]
	if c.Message.Refusal != "" {
		return fmt.Errorf("%w: recusa do modelo", ErrInvalidOutput)
	}
	if c.FinishReason != "" && c.FinishReason != "stop" {
		return fmt.Errorf("%w: finish_reason %q", ErrInvalidOutput, c.FinishReason)
	}
	if err := json.Unmarshal([]byte(c.Message.Content), out); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	return nil
}

// params builds the Chat Completions request for req.
func params(model string, req Request) openai.ChatCompletionNewParams {
	return openai.ChatCompletionNewParams{
		Model: shared.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.System),
			openai.UserMessage(req.User),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   req.SchemaName,
					Strict: openai.Bool(true),
					Schema: req.Schema,
				},
			},
		},
	}
}
