package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func securityRegistry() *Registry {
	return NewRegistry([]Definition{
		{
			Name:     "peek",
			Risk:     RiskLow,
			ReadOnly: true,
			Run:      func(context.Context, json.RawMessage, Context) Result { return Success("ok") },
		},
		{
			Name: "touch",
			Risk: RiskHigh,
			Run:  func(context.Context, json.RawMessage, Context) Result { return Success("written") },
		},
	}, Metadata{})
}

func TestReadOnlyRejectionCarriesSecurityEvent(t *testing.T) {
	result := securityRegistry().Execute(context.Background(), "touch", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{ReadOnly: true},
	})
	if result.Meta.SecurityEvent != SecurityReadOnlyBlocked {
		t.Fatalf("security event = %q", result.Meta.SecurityEvent)
	}
}

func TestAllowlistRejectionCarriesSecurityEvent(t *testing.T) {
	result := securityRegistry().Execute(context.Background(), "touch", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{Allowed: []string{"peek"}},
	})
	if result.Meta.SecurityEvent != SecurityToolNotAllowed {
		t.Fatalf("security event = %q", result.Meta.SecurityEvent)
	}
}

func TestNormalCallHasNoSecurityEvent(t *testing.T) {
	result := securityRegistry().Execute(context.Background(), "peek", nil, Context{CWD: t.TempDir()})
	if result.Meta.SecurityEvent != "" {
		t.Fatalf("正常调用不应带安全事件: %q", result.Meta.SecurityEvent)
	}
}
