package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeRunDir(t *testing.T, root, name string, report map[string]any, events []string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := mustJSON(t, report)
	if err := os.WriteFile(filepath.Join(dir, "report.json"), payload, 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	body := ""
	for _, line := range events {
		body += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return payload
}

func TestAggregateRunDirReadsReportAndEvents(t *testing.T) {
	root := t.TempDir()
	writeRunDir(t, root, "run-1", map[string]any{
		"status":        "completed",
		"stop_reason":   "final_answer_returned",
		"attempts":      3,
		"tool_steps":    2,
		"tool_failures": 1,
		"duration_ms":   1500,
	}, []string{
		`{"seq":1,"type":"prompt_built","data":{"prompt_chars":1000}}`,
		`{"seq":2,"type":"prompt_built","data":{"prompt_chars":1200}}`,
	})

	metrics, err := AggregateRunDir(filepath.Join(root, "run-1"))
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if metrics.Status != "completed" || metrics.Attempts != 3 {
		t.Fatalf("报告字段读取错误: %#v", metrics)
	}
	if metrics.PromptChars != 2200 {
		t.Fatalf("prompt 字符应累加: %d", metrics.PromptChars)
	}
	if metrics.ToolFailures != 1 {
		t.Fatalf("工具失败数 = %d", metrics.ToolFailures)
	}
}

func TestAggregateRunsSummarizesAllDirectories(t *testing.T) {
	root := t.TempDir()
	writeRunDir(t, root, "run-1", map[string]any{
		"status": "completed", "attempts": 2, "tool_steps": 1, "tool_failures": 0,
	}, []string{`{"seq":1,"type":"prompt_built","data":{"prompt_chars":500}}`})
	writeRunDir(t, root, "run-2", map[string]any{
		"status": "stopped", "attempts": 4, "tool_steps": 3, "tool_failures": 1,
	}, []string{`{"seq":1,"type":"prompt_built","data":{"prompt_chars":1500}}`})

	all, summary, err := AggregateRuns(root)
	if err != nil {
		t.Fatalf("aggregate runs: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("运行数 = %d", len(all))
	}
	if summary.Total != 2 || summary.Passed != 1 {
		t.Fatalf("汇总计数错误: %#v", summary)
	}
	if summary.PassRate != 0.5 {
		t.Fatalf("完成率 = %v", summary.PassRate)
	}
	if summary.AvgAttempts != 3 {
		t.Fatalf("平均请求 = %v", summary.AvgAttempts)
	}
	if summary.AvgPromptChars != 1000 {
		t.Fatalf("平均 prompt = %v", summary.AvgPromptChars)
	}
	if summary.ToolFailureRate != 0.25 {
		t.Fatalf("工具失败率 = %v", summary.ToolFailureRate)
	}
}

func TestAggregateRunsSkipsBrokenDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeRunDir(t, root, "ok", map[string]any{"status": "completed"}, nil)

	all, summary, err := AggregateRuns(root)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(all) != 1 || summary.Total != 1 {
		t.Fatalf("坏目录应被跳过: %#v", all)
	}
}

func TestSummarizeRunsEmpty(t *testing.T) {
	summary := SummarizeRuns(nil)
	if summary.Total != 0 || summary.PassRate != 0 {
		t.Fatalf("空输入应返回零值: %#v", summary)
	}
}
