package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

type scriptedCompleter struct {
	responses []string
	index     int
	err       error
	prompts   []string
}

func (s *scriptedCompleter) Complete(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if s.err != nil {
		return "", s.err
	}
	if s.index >= len(s.responses) {
		return "<final>结束</final>", nil
	}
	out := s.responses[s.index]
	s.index++
	return out, nil
}

func echoRegistry() *tools.Registry {
	return tools.NewRegistry([]tools.Definition{{
		Name:        "echo",
		Description: "返回传入的文本",
		InputSchema: map[string]any{"type": "object"},
		Run: func(_ context.Context, raw json.RawMessage, _ tools.Context) tools.Result {
			return tools.Success(string(raw))
		},
	}}, tools.Metadata{})
}

func TestRunReturnsFinalAnswer(t *testing.T) {
	model := &scriptedCompleter{responses: []string{"<final>改好了</final>"}}
	recorder := &captureRecorder{}

	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "修一下",
		Recorder:  recorder,
	})

	if result.State.Status != StatusCompleted {
		t.Fatalf("status = %s", result.State.Status)
	}
	if result.State.StopReason != StopFinalAnswer || result.State.FinalAnswer != "改好了" {
		t.Fatalf("state = %#v", result.State)
	}
	if len(result.History) != 2 {
		t.Fatalf("历史应记下请求与回答: %#v", result.History)
	}
	if _, ok := recorder.find("model_requested"); !ok {
		t.Fatal("缺少 model.request 事件")
	}
}

func TestRunExecutesToolThenFinishes(t *testing.T) {
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"echo","args":{"text":"hi"}}</tool>`,
		"<final>好了</final>",
	}}
	recorder := &captureRecorder{}

	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "跑一下",
		Recorder:  recorder,
	})

	if result.State.ToolSteps != 1 {
		t.Fatalf("工具步数 = %d", result.State.ToolSteps)
	}
	if result.State.LastTool != "echo" {
		t.Fatalf("last tool = %s", result.State.LastTool)
	}
	if _, ok := recorder.find("tool_executed"); !ok {
		t.Fatal("缺少 tool_executed 事件")
	}
	if len(model.prompts) != 2 {
		t.Fatalf("应请求两次模型: %d", len(model.prompts))
	}
	if !strings.Contains(model.prompts[1], "echo") {
		t.Fatal("第二次 prompt 应包含上一轮的工具结果")
	}
}

func TestRunRetriesOnMalformedResponse(t *testing.T) {
	model := &scriptedCompleter{responses: []string{
		"<tool>{bad json}</tool>",
		"<final>好了</final>",
	}}
	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "试试",
	})

	if result.State.Status != StatusCompleted {
		t.Fatalf("格式错误后应能继续: %#v", result.State)
	}
	if len(model.prompts) != 2 {
		t.Fatalf("应重试一次: %d", len(model.prompts))
	}
	if !strings.Contains(model.prompts[1], "Runtime notice") {
		t.Fatal("重试时的 prompt 应包含格式提示")
	}
}

func TestRunStopsAtRetryLimit(t *testing.T) {
	model := &scriptedCompleter{responses: []string{
		"<tool>{bad}</tool>",
		"<tool>{bad}</tool>",
		"<tool>{bad}</tool>",
		"<tool>{bad}</tool>",
		"<tool>{bad}</tool>",
	}}
	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "一直错",
		MaxSteps:  1,
	})

	if result.State.Status != StatusStopped || result.State.StopReason != StopRetryLimit {
		t.Fatalf("应在重试上限处停下: %#v", result.State)
	}
}

func TestRunStopsAtStepLimit(t *testing.T) {
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"echo","args":{}}</tool>`,
		`<tool>{"name":"echo","args":{}}</tool>`,
		`<tool>{"name":"echo","args":{}}</tool>`,
		`<tool>{"name":"echo","args":{}}</tool>`,
	}}
	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "无限调用",
		MaxSteps:  3,
	})

	if result.State.StopReason != StopStepLimit {
		t.Fatalf("stop reason = %s", result.State.StopReason)
	}
	if result.State.ToolSteps != 3 {
		t.Fatalf("工具步数 = %d", result.State.ToolSteps)
	}
}

func TestRunReportsModelError(t *testing.T) {
	model := &scriptedCompleter{err: errors.New("boom")}
	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "x",
	})
	if result.State.Status != StatusFailed || result.State.StopReason != StopModelError {
		t.Fatalf("state = %#v", result.State)
	}
}

func TestRunKeepsPreviousHistory(t *testing.T) {
	model := &scriptedCompleter{responses: []string{"<final>ok</final>"}}
	history := []ctxpkg.HistoryEntry{{Role: ctxpkg.RoleUser, Text: "上一轮的问题"}}

	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		History:   history,
		UserInput: "这一轮",
	})

	if len(result.History) != 3 {
		t.Fatalf("历史应累积: %#v", result.History)
	}
	if !strings.Contains(model.prompts[0], "上一轮的问题") {
		t.Fatal("prompt 应包含上一轮历史")
	}
}

func TestRunRecordsToolPath(t *testing.T) {
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"echo","args":{"path":"a.go"}}</tool>`,
		"<final>ok</final>",
	}}
	var seenPath string
	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "读文件",
		OnTool:    func(name, output string, ok bool) { seenPath = name },
	})
	if seenPath != "echo" {
		t.Fatalf("OnTool 未收到工具名: %s", seenPath)
	}
}

func TestRunRecordsAttempts(t *testing.T) {
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"echo","args":{}}</tool>`,
		"<final>完</final>",
	}}
	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "两步",
	})
	if result.State.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", result.State.Attempts)
	}
}
