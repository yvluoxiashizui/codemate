package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/memory"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func TestRunPromotesDurableMemoryOnIntent(t *testing.T) {
	durable, err := memory.OpenDurable(t.TempDir())
	if err != nil {
		t.Fatalf("open durable: %v", err)
	}
	model := &scriptedCompleter{responses: []string{"<final>记下了</final>"}}

	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "记住\nDecision: 数据库用 mysql 8",
		Durable:   durable,
	})

	notes := durable.ByTopic(memory.TopicDecisions)
	if len(notes) != 1 {
		t.Fatalf("应写入长期记忆: %#v", durable.Notes())
	}
	if !strings.Contains(notes[0].Text, "mysql") {
		t.Fatalf("内容不符: %q", notes[0].Text)
	}
}

func TestRunIgnoresDurableWithoutIntent(t *testing.T) {
	durable, _ := memory.OpenDurable(t.TempDir())
	model := &scriptedCompleter{responses: []string{"<final>ok</final>"}}

	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "看下这个函数",
		Durable:   durable,
	})

	if len(durable.Notes()) != 0 {
		t.Fatalf("没有记住意图时不应提升: %#v", durable.Notes())
	}
}

func TestRunInjectsDurableIntoPrompt(t *testing.T) {
	durable, _ := memory.OpenDurable(t.TempDir())
	durable.Promote([]memory.Promotion{{Topic: memory.TopicPreferences, Text: "回答保持简洁"}})

	model := &scriptedCompleter{responses: []string{"<final>ok</final>"}}
	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "随便问问",
		Memory:    mustMemory(t),
		Durable:   durable,
	})

	if !strings.Contains(model.prompts[0], "回答保持简洁") {
		t.Fatalf("长期记忆应进入 prompt: %q", model.prompts[0])
	}
}

func mustMemory(t *testing.T) *memory.Store {
	t.Helper()
	store, err := memory.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	return store
}
