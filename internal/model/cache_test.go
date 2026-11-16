package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/config"
)

func TestCompleteWithCacheSendsCacheKey(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<final>ok</final>"}}],"usage":{"prompt_tokens_details":{"cached_tokens":128}}}`))
	}))
	defer server.Close()

	client := &OpenAI{
		runtime: func(context.Context) (config.Runtime, error) {
			return config.Runtime{BaseURL: server.URL + "/openai.com"}, nil
		},
		client: server.Client(),
	}

	_, cached, err := client.CompleteWithCache(context.Background(), "hi", "abc123", "")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if captured["prompt_cache_key"] != "abc123" {
		t.Fatalf("payload 未带缓存键: %#v", captured["prompt_cache_key"])
	}
	if cached != 128 {
		t.Fatalf("cached tokens = %d", cached)
	}
}

func TestCompleteWithCacheSkipsUnsupportedProvider(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	client := &OpenAI{
		runtime: func(context.Context) (config.Runtime, error) {
			return config.Runtime{BaseURL: server.URL}, nil
		},
		client: server.Client(),
	}

	if _, _, err := client.CompleteWithCache(context.Background(), "hi", "abc123", ""); err != nil {
		t.Fatalf("complete: %v", err)
	}
	if _, exists := captured["prompt_cache_key"]; exists {
		t.Fatal("不支持的供应商不应发送缓存键")
	}
}
