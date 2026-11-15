package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/config"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func TestOpenAICompleteSendsSingleUserPrompt(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"<final>ok</final>"}}]}`))
	}))
	defer server.Close()

	adapter := NewOpenAI(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "openai", Model: "gpt", BaseURL: server.URL, APIKey: "k"}, nil
	}, tools.NewRegistry(nil, tools.Metadata{}))

	text, err := adapter.Complete(context.Background(), "PROMPT-BODY")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if text != "<final>ok</final>" {
		t.Fatalf("text = %q", text)
	}

	messages, _ := captured["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("应只发送一条消息: %#v", captured["messages"])
	}
	first, _ := messages[0].(map[string]any)
	if first["role"] != "user" || first["content"] != "PROMPT-BODY" {
		t.Fatalf("消息内容不符: %#v", first)
	}
	if _, ok := captured["tools"]; ok {
		t.Fatal("文本协议下不应再发送工具声明")
	}
}

func TestAnthropicCompleteJoinsTextBlocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"前段"},{"type":"thinking"},{"type":"text","text":"后段"}]}`))
	}))
	defer server.Close()

	adapter := NewAnthropic(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "anthropic", Model: "claude", BaseURL: server.URL, APIKey: "k"}, nil
	}, tools.NewRegistry(nil, tools.Metadata{}))

	text, err := adapter.Complete(context.Background(), "PROMPT")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if text != "前段后段" {
		t.Fatalf("text = %q", text)
	}
}

func TestCompleteReportsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer server.Close()

	adapter := NewOpenAI(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "openai", Model: "gpt", BaseURL: server.URL, APIKey: "k"}, nil
	}, tools.NewRegistry(nil, tools.Metadata{}))

	_, err := adapter.Complete(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("应返回服务端错误: %v", err)
	}
}

func TestOpenAICompleteHandlesEmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	adapter := NewOpenAI(func(context.Context) (config.Runtime, error) {
		return config.Runtime{Provider: "openai", Model: "gpt", BaseURL: server.URL, APIKey: "k"}, nil
	}, tools.NewRegistry(nil, tools.Metadata{}))

	if _, err := adapter.Complete(context.Background(), "x"); err == nil {
		t.Fatal("空 choices 应报错")
	}
}
