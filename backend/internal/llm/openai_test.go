package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"
)

var testSchema = map[string]any{
	"type":                 "object",
	"properties":           map[string]any{"answer": map[string]any{"type": "string"}},
	"required":             []string{"answer"},
	"additionalProperties": false,
}

func TestParamsJSON(t *testing.T) {
	p := params("gpt-5-mini", Request{System: "sys", User: "usr", SchemaName: "resposta", Schema: testSchema})
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		ResponseFormat struct {
			Type       string `json:"type"`
			JSONSchema struct {
				Name   string         `json:"name"`
				Strict bool           `json:"strict"`
				Schema map[string]any `json:"schema"`
			} `json:"json_schema"`
		} `json:"response_format"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "gpt-5-mini" || len(got.Messages) != 2 || got.Messages[0].Role != "system" ||
		got.Messages[0].Content != "sys" || got.Messages[1].Role != "user" || got.Messages[1].Content != "usr" {
		t.Errorf("request = %s", raw)
	}
	rf := got.ResponseFormat
	if rf.Type != "json_schema" || rf.JSONSchema.Name != "resposta" || !rf.JSONSchema.Strict ||
		rf.JSONSchema.Schema["additionalProperties"] != false {
		t.Errorf("response_format = %s", raw)
	}
}

// fakeServer answers like the Chat Completions endpoint, on localhost only.
func fakeServer(t *testing.T, content, finish string, seen *http.Request, body *[]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r.Clone(context.Background())
		*body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id": "x", "object": "chat.completion", "created": 1, "model": "m",
			"choices": []any{map[string]any{
				"index": 0, "finish_reason": finish,
				"message": map[string]any{"role": "assistant", "content": content, "refusal": ""},
			}},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenAIJSONDecodesStructuredOutput(t *testing.T) {
	var seen http.Request
	var body []byte
	srv := fakeServer(t, `{"answer":"Padaria e café"}`, "stop", &seen, &body)
	c := NewOpenAI("chave-de-teste", "gpt-5-mini", option.WithBaseURL(srv.URL))

	var out struct{ Answer string }
	if err := c.JSON(context.Background(), Request{System: "s", User: "u", SchemaName: "r", Schema: testSchema}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Answer != "Padaria e café" {
		t.Errorf("out = %+v", out)
	}
	if !strings.HasSuffix(seen.URL.Path, "/chat/completions") || seen.Header.Get("Authorization") != "Bearer chave-de-teste" {
		t.Errorf("request %s auth=%q", seen.URL.Path, seen.Header.Get("Authorization"))
	}
	if !strings.Contains(string(body), `"strict":true`) || !strings.Contains(string(body), `"json_schema"`) {
		t.Errorf("body = %s", body)
	}
}

func TestOpenAIJSONInvalidOutput(t *testing.T) {
	for name, tc := range map[string]struct{ content, finish string }{
		"not json":  {`isto não é json`, "stop"},
		"truncated": {`{"answer":"Pad`, "length"},
	} {
		t.Run(name, func(t *testing.T) {
			var seen http.Request
			var body []byte
			srv := fakeServer(t, tc.content, tc.finish, &seen, &body)
			c := NewOpenAI("k", "m", option.WithBaseURL(srv.URL))
			var out map[string]any
			if err := c.JSON(context.Background(), Request{SchemaName: "r", Schema: testSchema}, &out); !errors.Is(err, ErrInvalidOutput) {
				t.Errorf("err = %v, want ErrInvalidOutput", err)
			}
		})
	}
}

func TestFromConfig(t *testing.T) {
	if c := FromConfig("", "m"); c != nil {
		t.Errorf("no key must give a nil Client, got %#v", c)
	}
	if c := FromConfig("k", "m"); c == nil {
		t.Error("key must give a client")
	}
}
