package eval

import (
	"context"
	"fmt"
)

// Regression 是一次回归对比的结果。
type Regression struct {
	Total     int
	Baseline  Summary
	Current   Summary
	Tolerance float64
	Passed    bool
	Reason    string
}

// PassRateDelta 返回本次相对基线的通过率变化。
func (r Regression) PassRateDelta() float64 {
	return r.Current.PassRate - r.Baseline.PassRate
}

// Regress 跑一遍任务集并与基线对比。
// 通过率低于基线超过容忍度时判为回归；基线为空时只记录本次结果。
func (r Runner) Regress(ctx context.Context, cases []Case, baseline Summary, tolerance float64) Regression {
	results := r.RunCases(ctx, cases)
	current := Summarize(results)

	regression := Regression{
		Total:     current.Total,
		Baseline:  baseline,
		Current:   current,
		Tolerance: tolerance,
	}
	if baseline.Total == 0 {
		regression.Passed = true
		regression.Reason = "无基线，仅记录本次结果"
		return regression
	}

	delta := regression.PassRateDelta()
	if delta < -tolerance {
		regression.Passed = false
		regression.Reason = fmt.Sprintf("通过率下降 %.2f，超过容忍度 %.2f", -delta, tolerance)
		return regression
	}
	regression.Passed = true
	regression.Reason = fmt.Sprintf("通过率变化 %+.2f，在容忍度内", delta)
	return regression
}

// Digest 生成一份可存档的基线摘要，用于后续回归对比。
func Digest(summary Summary) map[string]any {
	return map[string]any{
		"total":             summary.Total,
		"passed":            summary.Passed,
		"pass_rate":         summary.PassRate,
		"avg_attempts":      summary.AvgAttempts,
		"avg_tool_steps":    summary.AvgToolSteps,
		"avg_prompt_chars":  summary.AvgPromptChars,
		"tool_failure_rate": summary.ToolFailureRate,
	}
}
