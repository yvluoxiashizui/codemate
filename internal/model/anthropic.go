package model

import (
	"net/http"

	"github.com/yvluoxiashizui/codemate/internal/tools"
)

// Anthropic 是 Anthropic 兼容接口的适配器。
type Anthropic struct {
	runtime RuntimeProvider
	tools   *tools.Registry
	client  *http.Client
}

// NewAnthropic 构造 Anthropic 兼容适配器。
func NewAnthropic(runtime RuntimeProvider, registry *tools.Registry) *Anthropic {
	return &Anthropic{runtime: runtime, tools: registry, client: http.DefaultClient}
}
