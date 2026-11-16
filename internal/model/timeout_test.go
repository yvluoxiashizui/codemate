package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yvluoxiashizui/codemate/internal/config"
)

func TestCompleteHonorsConfiguredTimeout(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<final>ok</final>"}}]}`))
	}))
	defer slow.Close()

	client := &OpenAI{
		runtime: func(context.Context) (config.Runtime, error) {
			return config.Runtime{BaseURL: slow.URL, Model: "m", TimeoutSeconds: 1}, nil
		},
		client: slow.Client(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Complete(ctx, "hi"); err != nil {
		t.Fatalf("一秒超时足够完成慢响应: %v", err)
	}
}

func TestCompleteFailsWhenTimeoutExceeded(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(400 * time.Millisecond)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"late"}}]}`))
	}))
	defer slow.Close()

	client := &OpenAI{
		runtime: func(context.Context) (config.Runtime, error) {
			return config.Runtime{BaseURL: slow.URL, Model: "m", TimeoutSeconds: 1}, nil
		},
		client: slow.Client(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := client.Complete(ctx, "hi"); err == nil {
		t.Fatal("调用方更短的上下文应能中断请求")
	}
}
