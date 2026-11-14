package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/config"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func TestOpenAIAdapterReadsUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": {"prompt_tokens": 1200, "completion_tokens": 80, "total_tokens": 1280},
			"choices": [{"finish_reason": "stop", "message": {"role": "assistant", "content": "<final>done</final>"}}]
		}`))
	}))
	defer server.Close()

	registry := tools.NewRegistry(nil, tools.Metadata{})
	adapter := NewOpenAI(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "openai", Model: "gpt", BaseURL: server.URL, APIKey: "k"}, nil
	}, registry)

	step, err := adapter.Next(context.Background(), []message.Message{message.UserMessage("hi")})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if step.Usage.InputTokens != 1200 || step.Usage.OutputTokens != 80 || step.Usage.TotalTokens != 1280 {
		t.Fatalf("unexpected usage: %#v", step.Usage)
	}
}

func TestOpenAIFillsTotalWhenMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{
			"usage": {"prompt_tokens": 300, "completion_tokens": 20},
			"choices": [{"finish_reason": "stop", "message": {"role": "assistant", "content": "ok"}}]
		}`))
	}))
	defer server.Close()

	registry := tools.NewRegistry(nil, tools.Metadata{})
	adapter := NewOpenAI(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "openai", Model: "gpt", BaseURL: server.URL, APIKey: "k"}, nil
	}, registry)

	step, err := adapter.Next(context.Background(), []message.Message{message.UserMessage("hi")})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if step.Usage.TotalTokens != 320 {
		t.Fatalf("total = %d, want 320", step.Usage.TotalTokens)
	}
}

func TestAnthropicAdapterReadsUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{
			"stop_reason": "end_turn",
			"usage": {"input_tokens": 640, "output_tokens": 32},
			"content": [{"type": "text", "text": "<final>done</final>"}]
		}`))
	}))
	defer server.Close()

	registry := tools.NewRegistry(nil, tools.Metadata{})
	adapter := NewAnthropic(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "anthropic", Model: "claude", BaseURL: server.URL, APIKey: "k"}, nil
	}, registry)

	step, err := adapter.Next(context.Background(), []message.Message{message.UserMessage("hi")})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if step.Usage.InputTokens != 640 || step.Usage.OutputTokens != 32 || step.Usage.TotalTokens != 672 {
		t.Fatalf("unexpected usage: %#v", step.Usage)
	}
}

func TestUsageIsZeroWhenProviderOmitsIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices": [{"finish_reason": "stop", "message": {"role": "assistant", "content": "ok"}}]}`))
	}))
	defer server.Close()

	registry := tools.NewRegistry(nil, tools.Metadata{})
	adapter := NewOpenAI(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "openai", Model: "gpt", BaseURL: server.URL, APIKey: "k"}, nil
	}, registry)

	step, err := adapter.Next(context.Background(), []message.Message{message.UserMessage("hi")})
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !step.Usage.IsZero() {
		t.Fatalf("expected zero usage, got %#v", step.Usage)
	}
}
