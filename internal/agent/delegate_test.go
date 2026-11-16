package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/tools"
)

// parentThenChild 先委派，拿到子 agent 结论后再收尾。
func TestDelegateReturnsChildConclusion(t *testing.T) {
	// 顺序：父发起委派 → 子给出结论 → 父收尾
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"delegate","args":{"task":"看看仓库结构"}}</tool>`,
		"<final>子 agent 的结论</final>",
		"<final>父 agent 完成</final>",
	}}

	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.Builtins(t.TempDir(), nil, nil),
		Prefix:    "p",
		UserInput: "调查一下",
	})

	if result.State.Status != StatusCompleted {
		t.Fatalf("status = %s", result.State.Status)
	}
	if len(result.History) == 0 {
		t.Fatal("应留下历史")
	}
	found := false
	for _, entry := range result.History {
		if strings.Contains(entry.Text, "delegate_result:") && strings.Contains(entry.Text, "子 agent 的结论") {
			found = true
		}
	}
	if !found {
		t.Fatalf("历史里应含子 agent 的结论: %#v", result.History)
	}
	if result.State.ToolSteps != 1 {
		t.Fatalf("父 agent 工具步数 = %d", result.State.ToolSteps)
	}
}

func TestDelegateRunsChildReadOnly(t *testing.T) {
	root := t.TempDir()
	// 顺序：父发起委派 → 子尝试写文件 → 子收尾 → 父收尾
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"delegate","args":{"task":"写个文件"}}</tool>`,
		`<tool>{"name":"write_file","args":{"path":"a.txt","content":"x"}}</tool>`,
		"<final>子完成</final>",
		"<final>父完成</final>",
	}}

	result := Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.Builtins(root, nil, nil),
		Prefix:    "p",
		UserInput: "让子 agent 写文件",
		CWD:       root,
	})

	if result.State.Status != StatusCompleted {
		t.Fatalf("status = %s", result.State.Status)
	}
	if _, err := os.Stat(filepath.Join(root, "a.txt")); err == nil {
		t.Fatal("子 agent 在只读模式下不应写文件")
	}
}

func TestDelegateRejectsEmptyTask(t *testing.T) {
	registry := tools.Builtins(t.TempDir(), nil, nil)
	result := registry.Execute(context.Background(), "delegate", map[string]any{
		"task": "   ",
	}, tools.Context{CWD: t.TempDir(), Spawn: func(context.Context, string, int) (string, error) {
		return "", nil
	}})

	if result.OK {
		t.Fatal("空任务应被拒绝")
	}
}

func TestDelegateWithoutSpawnFails(t *testing.T) {
	registry := tools.Builtins(t.TempDir(), nil, nil)
	result := registry.Execute(context.Background(), "delegate", map[string]any{
		"task": "看看",
	}, tools.Context{CWD: t.TempDir()})

	if result.OK {
		t.Fatal("没有派生能力时应报错")
	}
}
