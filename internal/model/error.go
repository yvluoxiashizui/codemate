package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/yvluoxiashizui/codemate/internal/message"
)

type ErrorModel struct {
	Err error
}

func (m ErrorModel) Next(_ context.Context, _ []message.Message) (message.Step, error) {
	if m.Err == nil {
		return message.Step{}, errors.New("model is not configured")
	}
	return message.Step{}, m.Err
}

// Complete 直接返回配置错误，用于配置缺失时给出明确提示。
func (m ErrorModel) Complete(_ context.Context, _ string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return "", fmt.Errorf("模型未配置")
}
