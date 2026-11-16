package eval

import (
	"context"
	"strings"

	"github.com/yvluoxiashizui/codemate/internal/agent"
	"github.com/yvluoxiashizui/codemate/internal/memory"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

// MemoryProbe 是记忆依赖实验的结果。
type MemoryProbe struct {
	Fact           string
	WithMemory     bool
	WithoutMemory  bool
	PromptWithFact int
	PromptWithout  int
}

// DependsOnMemory 报告记忆开启时事实确实进入 prompt、关闭后不再进入。
func (p MemoryProbe) DependsOnMemory() bool {
	return p.WithMemory && !p.WithoutMemory
}

// MeasureMemoryDependency 检验一条事实是否真的经由记忆层进入 prompt。
// 这是判断「记忆有没有起作用」的最小实验：只观察注入，不评价模型回答质量。
func MeasureMemoryDependency(workspace, fact, question string, model message.Completer) (MemoryProbe, error) {
	probe := MemoryProbe{Fact: fact}

	store, err := memory.Open(workspace)
	if err != nil {
		return probe, err
	}
	store.AppendNote(fact)

	recorder := &promptRecorder{inner: model}
	run := func(features agent.Features) string {
		recorder.last = ""
		agent.Run(context.Background(), agent.RunArgs{
			Model:     recorder,
			Tools:     tools.NewRegistry(nil, tools.Metadata{}),
			Prefix:    "prefix",
			UserInput: question,
			Memory:    store,
			Features:  features,
		})
		return recorder.last
	}

	withPrompt := run(agent.DefaultFeatures())
	withoutPrompt := run(agent.DefaultFeatures().Apply(map[string]bool{"memory": false}))

	probe.WithMemory = strings.Contains(withPrompt, fact)
	probe.WithoutMemory = strings.Contains(withoutPrompt, fact)
	probe.PromptWithFact = len(withPrompt)
	probe.PromptWithout = len(withoutPrompt)
	return probe, nil
}

// promptRecorder 记录最后一次请求的 prompt。
type promptRecorder struct {
	inner message.Completer
	last  string
}

func (p *promptRecorder) Complete(ctx context.Context, prompt string) (string, error) {
	p.last = prompt
	if p.inner == nil {
		return "<final>ok</final>", nil
	}
	return p.inner.Complete(ctx, prompt)
}
