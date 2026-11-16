package eval

import (
	"context"

	"github.com/yvluoxiashizui/codemate/internal/agent"
)

// Ablation 是关闭某一项能力前后的对照。
type Ablation struct {
	Feature  string
	Baseline Summary
	Disabled Summary
}

// PromptCharsDelta 返回关闭后相对基线的 prompt 字符数变化，负值表示变短。
func (a Ablation) PromptCharsDelta() float64 {
	return a.Disabled.AvgPromptChars - a.Baseline.AvgPromptChars
}

// PassRateDelta 返回关闭后通过率的变化，负值表示变差。
func (a Ablation) PassRateDelta() float64 {
	return a.Disabled.PassRate - a.Baseline.PassRate
}

// Ablate 用同一批任务在开、关某项能力两种配置下各跑一遍。
// 用于量化「这个能力到底值不值」，也是判断某项机制有没有实际效果的依据。
func (r Runner) Ablate(ctx context.Context, cases []Case, feature string) Ablation {
	baseline := r
	baseline.Features = baselineFeatures(r.Features)
	baseResults := baseline.RunCases(ctx, cases)

	disabled := r
	disabled.Features = baseline.Features.Apply(map[string]bool{feature: false})
	disabledResults := disabled.RunCases(ctx, cases)

	return Ablation{
		Feature:  feature,
		Baseline: Summarize(baseResults),
		Disabled: Summarize(disabledResults),
	}
}

// baselineFeatures 在调用方没有指定开关时给出默认值。
func baselineFeatures(features agent.Features) agent.Features {
	if !features.Memory && !features.RelevantMemory && !features.ContextReduction && !features.PromptCache {
		return agent.DefaultFeatures()
	}
	return features
}
