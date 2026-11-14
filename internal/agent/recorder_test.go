package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

type captureRecorder struct {
	events []capturedEvent
}

type capturedEvent struct {
	typ  string
	data map[string]any
}

func (c *captureRecorder) Event(typ string, data map[string]any) {
	c.events = append(c.events, capturedEvent{typ: typ, data: data})
}

func (c *captureRecorder) types() []string {
	out := make([]string, 0, len(c.events))
	for _, event := range c.events {
		out = append(out, event.typ)
	}
	return out
}

func (c *captureRecorder) find(typ string) (capturedEvent, bool) {
	for _, event := range c.events {
		if event.typ == typ {
			return event, true
		}
	}
	return capturedEvent{}, false
}

type oneToolModel struct {
	called bool
}

func (m *oneToolModel) Next(context.Context, []message.Message) (message.Step, error) {
	if !m.called {
		m.called = true
		return message.ToolCallsStep([]message.ToolCall{
			{ID: "call-1", ToolName: "echo", Input: json.RawMessage(`{"text":"hi"}`)},
		}, "", message.ContentNone, message.Diagnostics{}), nil
	}
	return message.AssistantStep("done", message.ContentFinal, message.Diagnostics{StopReason: "end_turn"}), nil
}

func TestRunTurnRecordsModelAndToolEvents(t *testing.T) {
	registry := tools.NewRegistry([]tools.Definition{{
		Name:        "echo",
		Description: "返回传入的文本",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
			"required":   []any{"text"},
		},
		Run: func(_ context.Context, raw json.RawMessage, _ tools.Context) tools.Result {
			return tools.Success(string(raw))
		},
	}}, tools.Metadata{})

	recorder := &captureRecorder{}
	_, err := RunTurn(context.Background(), Args{
		Model:    &oneToolModel{},
		Tools:    registry,
		Messages: []message.Message{message.UserMessage("hi")},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}

	got := recorder.types()
	want := []string{"context.budget", "model.call", "tool.call", "tool.result", "context.budget", "model.call", "turn.end"}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %s, want %s", i, got[i], want[i])
		}
	}

	result, ok := recorder.find("tool.result")
	if !ok {
		t.Fatal("missing tool.result event")
	}
	if result.data["name"] != "echo" || result.data["ok"] != true {
		t.Fatalf("unexpected tool.result data: %v", result.data)
	}

	end, ok := recorder.find("turn.end")
	if !ok {
		t.Fatal("missing turn.end event")
	}
	if end.data["model_calls"] != 2 || end.data["tool_calls"] != 1 {
		t.Fatalf("unexpected turn.end counters: %v", end.data)
	}
}

func TestRunTurnWithoutRecorderDoesNotPanic(t *testing.T) {
	registry := tools.NewRegistry(nil, tools.Metadata{})
	_, err := RunTurn(context.Background(), Args{
		Model:    &oneToolModel{},
		Tools:    registry,
		Messages: []message.Message{message.UserMessage("hi")},
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}
}
