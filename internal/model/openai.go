package model

import (
	"context"
	"net/http"

	"github.com/yvluoxiashizui/codemate/internal/config"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

// RuntimeProvider 在每次请求前给出当前运行配置。
type RuntimeProvider func(context.Context) (config.Runtime, error)

// OpenAI 是 OpenAI 兼容接口的适配器。
type OpenAI struct {
	runtime RuntimeProvider
	tools   *tools.Registry
	client  *http.Client
}

// NewOpenAI 构造 OpenAI 兼容适配器。
func NewOpenAI(runtime RuntimeProvider, registry *tools.Registry) *OpenAI {
	return &OpenAI{runtime: runtime, tools: registry, client: http.DefaultClient}
}
