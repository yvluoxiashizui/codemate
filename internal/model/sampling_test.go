package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/config"
)

func capturePayload(t *testing.T, runtime config.Runtime) map[string]any {
	t.Helper()
	captured := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"<final>ok</final>"}}]}`))
	}))
	t.Cleanup(server.Close)

	runtime.BaseURL = server.URL
	client := &OpenAI{
		runtime: func(context.Context) (config.Runtime, error) { return runtime, nil },
		client:  server.Client(),
	}
	if _, err := client.Complete(context.Background(), "hi"); err != nil {
		t.Fatalf("complete: %v", err)
	}
	return captured
}

func TestPayloadCarriesSamplingParameters(t *testing.T) {
	payload := capturePayload(t, config.Runtime{Model: "m", Temperature: 0.7, TopP: 0.95})

	if payload["temperature"] != 0.7 {
		t.Fatalf("temperature = %v", payload["temperature"])
	}
	if payload["top_p"] != 0.95 {
		t.Fatalf("top_p = %v", payload["top_p"])
	}
}

func TestPayloadOmitsSamplingWhenUnset(t *testing.T) {
	payload := capturePayload(t, config.Runtime{Model: "m"})

	if _, ok := payload["temperature"]; ok {
		t.Fatalf("未配置时不应发送 temperature: %v", payload["temperature"])
	}
	if _, ok := payload["top_p"]; ok {
		t.Fatalf("未配置时不应发送 top_p: %v", payload["top_p"])
	}
}

func TestPayloadCarriesMaxTokens(t *testing.T) {
	payload := capturePayload(t, config.Runtime{Model: "m", MaxOutputTokens: 128})
	if payload["max_tokens"] != float64(128) {
		t.Fatalf("max_tokens = %v", payload["max_tokens"])
	}
}
