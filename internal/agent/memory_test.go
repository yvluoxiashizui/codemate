package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/memory"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func TestRunInjectsMemoryIntoTurnedPrompt(t *testing.T) {
	store, err := memory.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	store.RememberFile("auth.go", "负责登录校验", "1:1")
	store.AppendNote("token 过期时间写错了")

	model := &scriptedCompleter{responses: []string{"<final>好</final>"}}
	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "prefix",
		UserInput: "token 的问题怎么查",
		Memory:    store,
	})

	if len(model.prompts) == 0 {
		t.Fatal("应有请求")
	}
	prompt := model.prompts[0]
	if !strings.Contains(prompt, "任务摘要") {
		t.Fatal("任务摘要应进入 prompt")
	}
	if !strings.Contains(prompt, "auth.go") {
		t.Fatal("文件摘要应进入 prompt")
	}
	if !strings.Contains(prompt, "token 过期") {
		t.Fatal("相关笔记应进入 prompt")
	}
}

func TestRunRemembersTouchedFile(t *testing.T) {
	store, _ := memory.Open(t.TempDir())
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"read_file","args":{"path":"a.go"}}</tool>`,
		"<final>读完</final>",
	}}
	root := t.TempDir()

	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.Builtins(root, nil, nil),
		Prefix:    "p",
		UserInput: "读 a.go",
		CWD:       root,
		Memory:    store,
		Freshness: func(string) string { return "1:1" },
	})

	state := store.State()
	if _, ok := state.FileSummaries["a.go"]; !ok {
		t.Fatalf("访问过的文件应写入记忆: %#v", state.FileSummaries)
	}
}
