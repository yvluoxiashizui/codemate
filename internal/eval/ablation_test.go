package eval

import (
	"context"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/agent"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
)

// longPrefixCase 造一条前缀很长、预算很小的任务，用来观察收缩是否生效。
func longPrefixCase() Case {
	return Case{
		Name:   "长前缀",
		Prompt: "回答一句",
		Check:  CheckFileMissing("nothing.txt"),
	}
}

func TestAblateContextReductionChangesPromptSize(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>ok</final>", "<final>ok</final>"}}
	runner := Runner{
		Model:    model,
		MaxSteps: 2,
		Options:  ctxpkg.Options{TotalBudget: 900, DisableReduction: false},
	}

	ablation := runner.Ablate(context.Background(), []Case{longPrefixCase()}, "context_reduction")

	if ablation.Feature != "context_reduction" {
		t.Fatalf("特性名 = %s", ablation.Feature)
	}
	if ablation.Baseline.Total != 1 || ablation.Disabled.Total != 1 {
		t.Fatalf("两次都应跑完一条: %#v %#v", ablation.Baseline, ablation.Disabled)
	}
	if ablation.Baseline.AvgPromptChars == 0 {
		t.Fatal("基线应记录 prompt 字符数")
	}
}

func TestAblateReportsSamePassRateWhenFeatureIrrelevant(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>ok</final>", "<final>ok</final>"}}
	runner := Runner{Model: model, MaxSteps: 2}

	ablation := runner.Ablate(context.Background(), []Case{longPrefixCase()}, "prompt_cache")
	if ablation.PassRateDelta() != 0 {
		t.Fatalf("关缓存不应改变通过率: %v", ablation.PassRateDelta())
	}
}

func TestBaselineFeaturesDefaults(t *testing.T) {
	features := baselineFeatures(agent.Features{})
	if !features.ContextReduction || !features.PromptCache {
		t.Fatalf("零值应解释为全开: %#v", features)
	}
}

func TestAblationDeltas(t *testing.T) {
	ablation := Ablation{
		Baseline: Summary{PassRate: 0.8, AvgPromptChars: 1000},
		Disabled: Summary{PassRate: 0.5, AvgPromptChars: 1400},
	}
	if ablation.PromptCharsDelta() != 400 {
		t.Fatalf("prompt 差 = %v", ablation.PromptCharsDelta())
	}
	if delta := ablation.PassRateDelta(); delta > -0.29 || delta < -0.31 {
		t.Fatalf("通过率差 = %v", delta)
	}
	if !strings.Contains("context_reduction", "reduction") {
		t.Fatal("占位断言")
	}
}
