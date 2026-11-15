package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Complete 以单段 prompt 请求 OpenAI 兼容接口，返回纯文本应答。
// 文本协议下模型不返回结构化工具调用，因此这里只关心正文。
func (o *OpenAI) Complete(ctx context.Context, prompt string) (string, error) {
	runtime, err := o.runtime(ctx)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"model":    runtime.Model,
		"messages": []map[string]any{{"role": "user", "content": prompt}},
	}
	if runtime.MaxOutputTokens > 0 {
		payload["max_tokens"] = runtime.MaxOutputTokens
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(runtime.BaseURL, "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	if runtime.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.AuthToken)
	} else if runtime.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.APIKey)
	}

	res, err := o.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data openAIResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if data.Error.Message != "" {
			return "", fmt.Errorf("%s", data.Error.Message)
		}
		return "", fmt.Errorf("model request failed: %d", res.StatusCode)
	}
	if len(data.Choices) == 0 {
		return "", fmt.Errorf("model returned no choices")
	}
	return data.Choices[0].Message.Content, nil
}

// Complete 以单段 prompt 请求 Anthropic 兼容接口，返回纯文本应答。
func (a *Anthropic) Complete(ctx context.Context, prompt string) (string, error) {
	runtime, err := a.runtime(ctx)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"model":      runtime.Model,
		"max_tokens": maxTokensOrDefault(runtime.MaxOutputTokens),
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(runtime.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	if runtime.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+runtime.AuthToken)
	} else if runtime.APIKey != "" {
		req.Header.Set("x-api-key", runtime.APIKey)
	}

	res, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var data struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Content []contentBlock `json:"content"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if data.Error != nil && data.Error.Message != "" {
			return "", fmt.Errorf("%s", data.Error.Message)
		}
		return "", fmt.Errorf("model request failed: %d", res.StatusCode)
	}

	parts := make([]string, 0, len(data.Content))
	for _, block := range data.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.Join(parts, ""), nil
}

func maxTokensOrDefault(value int) int {
	if value > 0 {
		return value
	}
	return 4096
}
