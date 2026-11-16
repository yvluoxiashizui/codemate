package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yvluoxiashizui/codemate/internal/tools"
)

// Ollama 是本地 Ollama 服务的适配器。
type Ollama struct {
	runtime RuntimeProvider
	tools   *tools.Registry
	client  *http.Client
}

// NewOllama 构造 Ollama 适配器。
func NewOllama(runtime RuntimeProvider, registry *tools.Registry) *Ollama {
	return &Ollama{runtime: runtime, tools: registry, client: http.DefaultClient}
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error"`
}

// Complete 调用 Ollama 的对话接口，返回纯文本应答。
func (o *Ollama) Complete(ctx context.Context, prompt string) (string, error) {
	runtime, err := o.runtime(ctx)
	if err != nil {
		return "", err
	}
	ctx, cancel := withTimeout(ctx, runtime.TimeoutSeconds)
	defer cancel()

	options := map[string]any{}
	if runtime.MaxOutputTokens > 0 {
		options["num_predict"] = runtime.MaxOutputTokens
	}
	if runtime.Temperature > 0 {
		options["temperature"] = runtime.Temperature
	}
	if runtime.TopP > 0 {
		options["top_p"] = runtime.TopP
	}
	payload := map[string]any{
		"model":    runtime.Model,
		"messages": []map[string]any{{"role": "user", "content": prompt}},
		"stream":   false,
	}
	if len(options) > 0 {
		payload["options"] = options
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(runtime.BaseURL, "/") + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	res, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data ollamaResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if data.Error != "" {
			return "", fmt.Errorf("%s", data.Error)
		}
		return "", fmt.Errorf("ollama request failed: %d", res.StatusCode)
	}
	return data.Message.Content, nil
}
