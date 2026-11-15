package agent

import (
	"context"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/tools"
)

type captureState struct {
	snapshots []map[string]any
}

func (c *captureState) SaveState(snapshot map[string]any) error {
	c.snapshots = append(c.snapshots, snapshot)
	return nil
}

func TestRunWritesTaskStateSnapshots(t *testing.T) {
	state := &captureState{}
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"echo","args":{}}</tool>`,
		"<final>完</final>",
	}}

	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "prefix",
		UserInput: "跑两步",
		State:     state,
	})

	if len(state.snapshots) < 3 {
		t.Fatalf("快照数 = %d, 至少应有开始、工具后、结束三次", len(state.snapshots))
	}
	final := state.snapshots[len(state.snapshots)-1]
	if final["status"] != string(StatusCompleted) {
		t.Fatalf("最后一份快照状态应为完成: %v", final["status"])
	}
	if final["tool_steps"] != 1 {
		t.Fatalf("工具步数 = %v", final["tool_steps"])
	}
	if final["final_answer"] != result.State.FinalAnswer {
		t.Fatalf("快照里的答案与结果不一致")
	}
}

func TestRunWithoutStateSinkDoesNotPanic(t *testing.T) {
	model := &scriptedCompleter{responses: []string{"<final>ok</final>"}}
	if _, err := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "x",
	}), (error)(nil); err != nil {
		t.Fatal(err)
	}
}
