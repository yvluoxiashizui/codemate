package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func allowlistRegistry() *Registry {
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

func TestAllowlistBlocksToolOutsideList(t *testing.T) {
	registry := allowlistRegistry()
	result := registry.Execute(context.Background(), "touch", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{Allowed: []string{"peek"}},
	})

	if result.OK {
		t.Fatal("白名单外的工具应被拒绝")
	}
	if result.Meta.ErrorCode != CodeToolNotAllowed {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}

func TestAllowlistPermitsListedTool(t *testing.T) {
	registry := allowlistRegistry()
	result := registry.Execute(context.Background(), "peek", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{Allowed: []string{"peek"}},
	})

	if !result.OK {
		t.Fatalf("白名单内的工具应放行: %s", result.Output)
	}
}

func TestAllowlistNilMeansEverything(t *testing.T) {
	registry := allowlistRegistry()
	result := registry.Execute(context.Background(), "touch", nil, Context{CWD: t.TempDir()})
	if !result.OK {
		t.Fatalf("未设白名单时应全部允许: %s", result.Output)
	}
}

func TestAllowlistWorksWithReadOnly(t *testing.T) {
	registry := allowlistRegistry()
	result := registry.Execute(context.Background(), "touch", nil, Context{
		CWD:    t.TempDir(),
		Policy: Policy{ReadOnly: true, Allowed: []string{"touch"}},
	})
	if result.OK {
		t.Fatal("只读模式优先于白名单")
	}
	if result.Meta.ErrorCode != CodeReadOnly {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}
