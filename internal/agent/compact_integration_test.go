package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

const summaryMarker = "请把它压缩成一段摘要"

type compactingModel struct {
	sawSummaryRequest bool
	failSummary       bool
}

func (m *compactingModel) Next(_ context.Context, messages []message.Message) (message.Step, error) {
	if len(messages) > 0 && strings.Contains(messages[len(messages)-1].Content, summaryMarker) {
		m.sawSummaryRequest = true
		if m.failSummary {
			return message.Step{}, errors.New("summary request failed")
		}
		return message.AssistantStep("用户要求检查仓库并修复失败的测试。", message.ContentNone, message.Diagnostics{}), nil
	}
	return message.AssistantStep("done", message.ContentFinal, message.Diagnostics{StopReason: "end_turn"}), nil
}

func longHistory(size int) []message.Message {
	return []message.Message{
		message.SystemMessage("system prompt"),
		message.UserMessage(strings.Repeat("a", size)),
		message.AssistantMessage("ok"),
	}
}

func TestRunTurnCompactsWhenContextIsTight(t *testing.T) {
	model := &compactingModel{}
	recorder := &captureRecorder{}

	_, err := RunTurn(context.Background(), Args{
		Model:    model,
		Tools:    tools.NewRegistry(nil, tools.Metadata{}),
		Messages: longHistory(4000),
		Window:   2000,
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if !model.sawSummaryRequest {
		t.Fatal("上下文紧张时应发出摘要请求")
	}

	event, ok := recorder.find("compact.done")
	if !ok {
		t.Fatal("缺少 compact.done 事件")
	}
	before, _ := event.data["tokens_before"].(int)
	after, _ := event.data["tokens_after"].(int)
	if before <= after {
		t.Fatalf("压缩后应更小: before=%d after=%d", before, after)
	}
}

func TestRunTurnKeepsHistoryWhenCompactionFails(t *testing.T) {
	model := &compactingModel{failSummary: true}
	recorder := &captureRecorder{}

	messages, err := RunTurn(context.Background(), Args{
		Model:    model,
		Tools:    tools.NewRegistry(nil, tools.Metadata{}),
		Messages: longHistory(4000),
		Window:   2000,
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("压缩失败不应中断任务: %v", err)
	}
	if _, ok := recorder.find("compact.failed"); !ok {
		t.Fatal("缺少 compact.failed 事件")
	}
	if _, ok := recorder.find("compact.done"); ok {
		t.Fatal("失败时不应记录 compact.done")
	}
	found := false
	for _, item := range messages {
		if strings.Contains(item.Content, strings.Repeat("a", 100)) {
			found = true
		}
	}
	if !found {
		t.Fatal("压缩失败后应保留原始历史")
	}
}

func TestRunTurnSkipsCompactionWhenRoomIsEnough(t *testing.T) {
	model := &compactingModel{}
	recorder := &captureRecorder{}

	_, err := RunTurn(context.Background(), Args{
		Model:    model,
		Tools:    tools.NewRegistry(nil, tools.Metadata{}),
		Messages: longHistory(4000),
		Window:   180000,
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
	if model.sawSummaryRequest {
		t.Fatal("预算充足时不应压缩")
	}
	if _, ok := recorder.find("compact.done"); ok {
		t.Fatal("预算充足时不应产出 compact.done")
	}
}
