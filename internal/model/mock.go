package model

import (
	"context"
	"fmt"
)

// Mock 是离线模式下的模型实现，返回固定答案。
type Mock struct{}

// Complete 返回固定的最终答案，不访问网络。
func (m Mock) Complete(_ context.Context, _ string) (string, error) {
	return "<final>mock 模式：未接入模型。</final>", nil
}

// ErrorModel 在配置缺失时返回明确的错误。
type ErrorModel struct {
	Err error
}

// Complete 直接返回配置错误。
func (m ErrorModel) Complete(_ context.Context, _ string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return "", fmt.Errorf("模型未配置")
}
