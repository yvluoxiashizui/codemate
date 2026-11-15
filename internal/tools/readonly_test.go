package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func readOnlyRegistry() *Registry {
	return NewRegistry([]Definition{
		{
			Name:     "peek",
			Risk:     RiskLow,
			ReadOnly: true,
			Run: func(_ context.Context, _ json.RawMessage, _ Context) Result {
				return Success("data")
			},
		},
		{
			Name: "touch",
			Risk: RiskHigh,
			Run: func(_ context.Context, _ json.RawMessage, _ Context) Result {
				return Success("written")
			},
		},
	}, Metadata{})
}

func TestReadOnlyPolicyBlocksWriteTools(t *testing.T) {
	registry := readOnlyRegistry()
	result := registry.Execute(context.Background(), "touch", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{ReadOnly: true},
	})

	if result.OK {
		t.Fatal("只读模式下写工具应被拒绝")
	}
	if result.Meta.ErrorCode != CodeReadOnly {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
	if result.Meta.Status != StatusReject {
		t.Fatalf("status = %s", result.Meta.Status)
	}
}

func TestReadOnlyPolicyAllowsReadTools(t *testing.T) {
	registry := readOnlyRegistry()
	result := registry.Execute(context.Background(), "peek", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{ReadOnly: true},
	})

	if !result.OK {
		t.Fatalf("只读工具应放行: %s", result.Output)
	}
}
