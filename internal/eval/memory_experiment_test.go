package eval

import (
	"context"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
)

type fixedModel struct{}

func (fixedModel) Complete(context.Context, string) (string, error) {
	return "<final>ok</final>", nil
}

var _ message.Completer = fixedModel{}

func TestMeasureMemoryDependencyInjectsFact(t *testing.T) {
	probe, err := MeasureMemoryDependency(t.TempDir(), "token 过期时间是 30 分钟", "token 多久过期", fixedModel{})
	if err != nil {
		t.Fatalf("measure: %v", err)
	}

	if !probe.WithMemory {
		t.Fatal("记忆开启时事实应进入 prompt")
	}
	if probe.WithoutMemory {
		t.Fatal("记忆关闭时事实不应进入 prompt")
	}
	if !probe.DependsOnMemory() {
		t.Fatalf("应判定为依赖记忆: %#v", probe)
	}
	if probe.PromptWithFact <= probe.PromptWithout {
		t.Fatalf("带记忆的 prompt 应更长: %d vs %d", probe.PromptWithFact, probe.PromptWithout)
	}
}

func TestMemoryProbeDependsOnBothSides(t *testing.T) {
	only := MemoryProbe{WithMemory: true, WithoutMemory: true}
	if only.DependsOnMemory() {
		t.Fatal("两侧都出现时不算依赖")
	}
	none := MemoryProbe{}
	if none.DependsOnMemory() {
		t.Fatal("两侧都没有时不算依赖")
	}
}
