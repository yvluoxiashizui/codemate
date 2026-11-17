package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/yvluoxiashizui/codemate/internal/runs"
)

// RunMetrics 是一次运行从工件里聚合出来的指标。
type RunMetrics struct {
	RunID        string
	Status       string
	StopReason   string
	Attempts     int
	ToolSteps    int
	ToolFailures int
	PromptChars  int
	DurationMS   int64
}

// reportFile 是 report.json 的读取结构。
type reportFile struct {
	Status       string `json:"status"`
	StopReason   string `json:"stop_reason"`
	Attempts     int    `json:"attempts"`
	ToolSteps    int    `json:"tool_steps"`
	ToolFailures int    `json:"tool_failures"`
	DurationMS   int64  `json:"duration_ms"`
}

// AggregateRunDir 读取一个运行目录，汇总报告与事件里的指标。
func AggregateRunDir(dir string) (RunMetrics, error) {
	metrics := RunMetrics{RunID: filepath.Base(dir)}

	payload, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		return metrics, err
	}
	var report reportFile
	if err := json.Unmarshal(payload, &report); err != nil {
		return metrics, err
	}
	metrics.Status = report.Status
	metrics.StopReason = report.StopReason
	metrics.Attempts = report.Attempts
	metrics.ToolSteps = report.ToolSteps
	metrics.ToolFailures = report.ToolFailures
	metrics.DurationMS = report.DurationMS

	events, err := runs.ReadEvents(filepath.Join(dir, "events.jsonl"))
	if err == nil {
		for _, event := range events {
			if event.Type != "prompt_built" {
				continue
			}
			if value, ok := event.Data["prompt_chars"].(float64); ok {
				metrics.PromptChars += int(value)
			}
		}
	}
	return metrics, nil
}

// AggregateRuns 扫描运行根目录，返回每条运行的指标与整体汇总。
func AggregateRuns(runsRoot string) ([]RunMetrics, Summary, error) {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return nil, Summary{}, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	all := make([]RunMetrics, 0, len(names))
	for _, name := range names {
		metrics, err := AggregateRunDir(filepath.Join(runsRoot, name))
		if err != nil {
			continue
		}
		all = append(all, metrics)
	}
	return all, SummarizeRuns(all), nil
}

// SummarizeRuns 把运行指标聚合成与任务集一致的汇总口径。
// 已完成的运行计入通过数，与任务集的「判定通过」对应。
func SummarizeRuns(all []RunMetrics) Summary {
	summary := Summary{Total: len(all)}
	if summary.Total == 0 {
		return summary
	}
	var attempts, steps, promptChars, failures int
	for _, item := range all {
		if item.Status == "completed" {
			summary.Passed++
		}
		attempts += item.Attempts
		steps += item.ToolSteps
		promptChars += item.PromptChars
		failures += item.ToolFailures
	}
	summary.PassRate = float64(summary.Passed) / float64(summary.Total)
	summary.AvgAttempts = float64(attempts) / float64(summary.Total)
	summary.AvgToolSteps = float64(steps) / float64(summary.Total)
	summary.AvgPromptChars = float64(promptChars) / float64(summary.Total)
	if attempts > 0 {
		summary.AvgPromptPerAttempt = float64(promptChars) / float64(attempts)
	}
	if steps > 0 {
		summary.ToolFailureRate = float64(failures) / float64(steps)
	}
	return summary
}
