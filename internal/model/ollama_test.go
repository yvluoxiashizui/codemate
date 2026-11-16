package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/config"
)

func TestOllamaCompleteReadsMessageContent(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("路径 = %s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"message":{"content":"<final>本地模型</final>"}}`))
	}))
	defer server.Close()

	client := NewOllama(func(context.Context) (config.Runtime, error) {
		return config.Runtime{BaseURL: server.URL, Model: "qwen", MaxOutputTokens: 64}, nil
	}, nil)

	got, err := client.Complete(context.Background(), "hi")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got != "<final>本地模型</final>" {
		t.Fatalf("应答 = %q", got)
	}
	if captured["stream"] != false {
		t.Fatalf("应显式关闭流式: %v", captured["stream"])
	}
	options, ok := captured["options"].(map[string]any)
	if !ok || options["num_predict"] != float64(64) {
		t.Fatalf("options = %#v", captured["options"])
	}
}

func TestOllamaReportsServiceError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer server.Close()

	client := NewOllama(func(context.Context) (config.Runtime, error) {
		return config.Runtime{BaseURL: server.URL, Model: "missing"}, nil
	}, nil)

	_, err := client.Complete(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "model not found") {
		t.Fatalf("应回传服务端错误: %v", err)
	}
}

func TestFactoryBuildsOllama(t *testing.T) {
	client, err := NewFromRuntime(config.Runtime{Provider: "ollama", Model: "qwen"}, nil)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if _, ok := client.(*Ollama); !ok {
		t.Fatalf("应构造 Ollama 适配器: %T", client)
	}
}
