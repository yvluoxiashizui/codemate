package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func signatureRegistry(description string) *Registry {
	return NewRegistry([]Definition{{
		Name:        "peek",
		Description: description,
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"path": map[string]any{"type": "string"}},
		},
		Risk:     RiskLow,
		ReadOnly: true,
		Run: func(context.Context, json.RawMessage, Context) Result {
			return Success("ok")
		},
	}}, Metadata{})
}

func TestSignatureStableForSameTools(t *testing.T) {
	first := signatureRegistry("看一眼").Signature()
	second := signatureRegistry("看一眼").Signature()
	if first == "" || first != second {
		t.Fatalf("相同工具集签名应一致: %q %q", first, second)
	}
}

func TestSignatureChangesWithDescription(t *testing.T) {
	if signatureRegistry("看一眼").Signature() == signatureRegistry("看两眼").Signature() {
		t.Fatal("说明变化应改变签名")
	}
}

func TestSignatureChangesWithToolSet(t *testing.T) {
	base := Builtins(t.TempDir(), nil, nil)
	trimmed := NewRegistry(base.List()[:3], Metadata{})
	if base.Signature() == trimmed.Signature() {
		t.Fatal("工具数量变化应改变签名")
	}
}

func TestNilRegistrySignature(t *testing.T) {
	var registry *Registry
	if registry.Signature() != "" {
		t.Fatal("nil 注册表应返回空签名")
	}
}
