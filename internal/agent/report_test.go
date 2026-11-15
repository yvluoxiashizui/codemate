package agent

import (
	"context"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/tools"
)

type captureReport struct {
	reports []map[string]any
}

func (c *captureReport) SaveReport(report map[string]any) error {
	c.reports = append(c.reports, report)
	return nil
}

func TestRunWritesReport(t *testing.T) {
	report := &captureReport{}
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"echo","args":{}}</tool>`,
		"<final>完成</final>",
	}}

	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     echoRegistry(),
		Prefix:    "p",
		UserInput: "跑两步",
		Report:    report,
	})

	if len(report.reports) != 1 {
		t.Fatalf("报告数 = %d, want 1", len(report.reports))
	}
	got := report.reports[0]
	if got["status"] != string(StatusCompleted) {
		t.Fatalf("status = %v", got["status"])
	}
	if got["tool_steps"] != 1 {
		t.Fatalf("tool_steps = %v", got["tool_steps"])
	}
	if got["final_answer"] != "完成" {
		t.Fatalf("final = %v", got["final_answer"])
	}
	calls, ok := got["tool_calls"].([]map[string]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("tool_calls = %#v", got["tool_calls"])
	}
	if calls[0]["name"] != "echo" {
		t.Fatalf("记录的工具名 = %v", calls[0]["name"])
	}
	if _, exists := got["duration_ms"]; !exists {
		t.Fatal("报告应含耗时")
	}
}

func TestRunWritesReportOnModelError(t *testing.T) {
	report := &captureReport{}
	model := &scriptedCompleter{err: context.DeadlineExceeded}

	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "会失败",
		Report:    report,
	})

	if len(report.reports) != 1 {
		t.Fatalf("失败也应写报告: %d", len(report.reports))
	}
	if report.reports[0]["status"] != string(StatusFailed) {
		t.Fatalf("status = %v", report.reports[0]["status"])
	}
}

func TestRunCountsToolFailuresInReport(t *testing.T) {
	report := &captureReport{}
	model := &scriptedCompleter{responses: []string{
		`<tool>{"name":"missing_tool","args":{}}</tool>`,
		"<final>结束</final>",
	}}
	Run(context.Background(), RunArgs{
		Model:     model,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Prefix:    "p",
		UserInput: "调用不存在的工具",
		Report:    report,
	})

	if len(report.reports) != 1 {
		t.Fatal("应写一份报告")
	}
	if report.reports[0]["tool_failures"] != 1 {
		t.Fatalf("tool_failures = %v", report.reports[0]["tool_failures"])
	}
}
