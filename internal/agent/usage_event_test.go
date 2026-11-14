package agent

import (
	"context"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

type usageModel struct {
	done bool
}

func (m *usageModel) Next(context.Context, []message.Message) (message.Step, error) {
	m.done = true
	return message.AssistantStep("done", message.ContentFinal, message.Diagnostics{StopReason: "end_turn"}).
		WithUsage(message.Usage{InputTokens: 900, OutputTokens: 45, TotalTokens: 945}), nil
}

func TestRunTurnRecordsModelUsage(t *testing.T) {
	recorder := &captureRecorder{}
	_, err := RunTurn(context.Background(), Args{
		Model:    &usageModel{},
		Tools:    tools.NewRegistry(nil, tools.Metadata{}),
		Messages: []message.Message{message.UserMessage("hi")},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("run turn: %v", err)
	}

	call, ok := recorder.find("model.call")
	if !ok {
		t.Fatal("missing model.call event")
	}
	if call.data["input_tokens"] != 900 || call.data["output_tokens"] != 45 {
		t.Fatalf("unexpected usage in event: %v", call.data)
	}
}
