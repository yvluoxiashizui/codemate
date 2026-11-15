package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestExecuteReportsUnknownTool(t *testing.T) {
	registry := NewRegistry(nil, Metadata{})
	result := registry.Execute(context.Background(), "nope", nil, Context{CWD: t.TempDir()})

	if result.OK {
		t.Fatal("未知工具应失败")
	}
	if result.Meta.Status != StatusReject || result.Meta.ErrorCode != CodeUnknownTool {
		t.Fatalf("meta = %#v", result.Meta)
	}
	if result.Meta.RiskLevel != RiskHigh {
		t.Fatalf("未知工具应按高风险处理: %s", result.Meta.RiskLevel)
	}
}

func TestExecuteReportsInvalidInput(t *testing.T) {
	registry := NewRegistry([]Definition{{
		Name:        "echo",
		Description: "回显",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []any{"text"},
		},
		Risk:     RiskLow,
		ReadOnly: true,
		Run: func(_ context.Context, _ json.RawMessage, _ Context) Result {
			return Success("ok")
		},
	}}, Metadata{})

	result := registry.Execute(context.Background(), "echo", map[string]any{"other": 1}, Context{CWD: t.TempDir()})
	if result.OK {
		t.Fatal("缺少必填参数应失败")
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
	if result.Meta.Status != StatusError {
		t.Fatalf("status = %s", result.Meta.Status)
	}
}

func TestExecuteRecordsReadOnlyTool(t *testing.T) {
	registry := NewRegistry([]Definition{{
		Name:        "peek",
		Description: "看一眼",
		Risk:        RiskLow,
		ReadOnly:    true,
		Run: func(_ context.Context, _ json.RawMessage, _ Context) Result {
			return Success("data")
		},
	}}, Metadata{})

	result := registry.Execute(context.Background(), "peek", nil, Context{CWD: t.TempDir()})
	if !result.Meta.ReadOnly || result.Meta.WorkspaceChanged {
		t.Fatalf("meta = %#v", result.Meta)
	}
	if result.Meta.RiskLevel != RiskLow {
		t.Fatalf("risk = %s", result.Meta.RiskLevel)
	}
}

func TestExecuteRecordsAffectedPaths(t *testing.T) {
	registry := NewRegistry([]Definition{{
		Name:        "touch",
		Description: "声明改动",
		Risk:        RiskHigh,
		Run: func(_ context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Path string `json:"path"`
			}
			_ = json.Unmarshal(raw, &input)
			tc.ReportAffected(input.Path)
			return Success("done")
		},
	}}, Metadata{})

	result := registry.Execute(context.Background(), "touch", map[string]any{"path": "a.go"}, Context{CWD: t.TempDir()})
	if len(result.Meta.AffectedPaths) != 1 || result.Meta.AffectedPaths[0] != "a.go" {
		t.Fatalf("affected = %#v", result.Meta.AffectedPaths)
	}
	if !result.Meta.WorkspaceChanged {
		t.Fatal("写类工具改了文件应标记工作区已变")
	}
}

func TestExecuteRecoversPanicWithMeta(t *testing.T) {
	registry := NewRegistry([]Definition{{
		Name:        "boom",
		Description: "崩溃",
		Run: func(_ context.Context, _ json.RawMessage, _ Context) Result {
			panic("坏了")
		},
	}}, Metadata{})

	result := registry.Execute(context.Background(), "boom", nil, Context{CWD: t.TempDir()})
	if result.OK {
		t.Fatal("panic 应转成失败结果")
	}
	if result.Meta.ErrorCode != CodeToolPanic {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
	if !strings.Contains(result.Output, "坏了") {
		t.Fatalf("output = %q", result.Output)
	}
}
