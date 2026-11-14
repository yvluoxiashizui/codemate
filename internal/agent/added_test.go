package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func TestRunTurnReportsAddedMessages(t *testing.T) {
	registry := tools.NewRegistry([]tools.Definition{{
		Name:        "echo",
		Description: "返回传入的文本",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"text"},
			"properties": map[string]any{
				"text": map[string]any{"type": "string"},
			},
		},
		Run: func(_ context.Context, raw json.RawMessage, _ tools.Context) tools.Result {
			return tools.Success(string(raw))
		},
	}}, tools.Metadata{})

	var added []message.Message
	input := []message.Message{message.UserMessage("hi")}
	_, err := RunTurn(context.Background(), Args{
		Model:    &oneToolModel{},
		Tools:    registry,
		Messages: input,
		AddedOut: &added,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}

	if len(added) != 3 {
		t.Fatalf("新增消息数 = %d, want 3", len(added))
	}
	if added[0].Role != message.RoleAssistantToolCall {
		t.Fatalf("第一条应为工具调用: %s", added[0].Role)
	}
	if added[1].Role != message.RoleToolResult {
		t.Fatalf("第二条应为工具结果: %s", added[1].Role)
	}
	if added[2].Role != message.RoleAssistant {
		t.Fatalf("第三条应为最终回答: %s", added[2].Role)
	}
	if len(input) != 1 {
		t.Fatalf("输入切片不应被修改: %d", len(input))
	}
}

func TestRunTurnWithoutAddedOutDoesNotPanic(t *testing.T) {
	_, err := RunTurn(context.Background(), Args{
		Model:    &oneToolModel{},
		Tools:    tools.NewRegistry(nil, tools.Metadata{}),
		Messages: []message.Message{message.UserMessage("hi")},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
}
