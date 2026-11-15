package eval

import (
	"context"
	"testing"
)

// scriptedModel 按顺序返回预设应答。
type scriptedModel struct {
	responses []string
	index     int
}

func (s *scriptedModel) Complete(_ context.Context, _ string) (string, error) {
	if s.index >= len(s.responses) {
		return "<final>结束</final>", nil
	}
	out := s.responses[s.index]
	s.index++
	return out, nil
}

func TestRunCasePassesWhenFileMatches(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`<tool name="write_file" path="hello.txt"><content>hi</content></tool>`,
		"<final>写好了</final>",
	}}
	runner := Runner{Model: model, MaxSteps: 4}

	result := runner.RunCase(context.Background(), Case{
		Name:   "写文件",
		Prompt: "把 hi 写进 hello.txt",
		Check:  CheckFileContains("hello.txt", "hi"),
	})

	if !result.Passed {
		t.Fatalf("应通过: %s", result.Reason)
	}
	if result.ToolSteps != 1 {
		t.Fatalf("工具步数 = %d", result.ToolSteps)
	}
	if result.Attempts != 2 {
		t.Fatalf("请求次数 = %d", result.Attempts)
	}
	if result.PromptChars == 0 {
		t.Fatal("应记录 prompt 字符数")
	}
}

func TestRunCaseFailsWhenFileMissing(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>没写</final>"}}
	runner := Runner{Model: model, MaxSteps: 2}

	result := runner.RunCase(context.Background(), Case{
		Name:   "不该通过",
		Prompt: "随便",
		Check:  CheckFileContains("hello.txt", "hi"),
	})

	if result.Passed {
		t.Fatal("文件不存在时不应通过")
	}
	if result.Reason == "" {
		t.Fatal("失败应带原因")
	}
}

func TestRunCaseSeedsInitialFiles(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>看到了</final>"}}
	runner := Runner{Model: model, MaxSteps: 2}

	result := runner.RunCase(context.Background(), Case{
		Name:   "初始文件",
		Prompt: "读一下",
		Files:  map[string]string{"a.go": "package a"},
		Check:  CheckFileEquals("a.go", "package a"),
	})

	if !result.Passed {
		t.Fatalf("初始文件应写入并被判定: %s", result.Reason)
	}
}

func TestRunCaseCountsToolFailures(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`<tool>{"name":"read_file","args":{"path":"missing.txt"}}</tool>`,
		"<final>失败也结束</final>",
	}}
	runner := Runner{Model: model, MaxSteps: 3}

	result := runner.RunCase(context.Background(), Case{
		Name:   "工具失败计数",
		Prompt: "读一个不存在的文件",
		Check:  CheckFileMissing("never.txt"),
	})

	if result.ToolFailures != 1 {
		t.Fatalf("工具失败数 = %d", result.ToolFailures)
	}
}

func TestSummarizeAggregates(t *testing.T) {
	results := []Result{
		{Passed: true, Attempts: 2, ToolSteps: 1, PromptChars: 100, ToolFailures: 0},
		{Passed: false, Attempts: 4, ToolSteps: 3, PromptChars: 200, ToolFailures: 1},
	}
	summary := Summarize(results)

	if summary.Total != 2 || summary.Passed != 1 {
		t.Fatalf("计数错误: %#v", summary)
	}
	if summary.PassRate != 0.5 {
		t.Fatalf("通过率 = %v", summary.PassRate)
	}
	if summary.AvgAttempts != 3 {
		t.Fatalf("平均请求次数 = %v", summary.AvgAttempts)
	}
	if summary.AvgToolSteps != 2 {
		t.Fatalf("平均工具步数 = %v", summary.AvgToolSteps)
	}
	if summary.AvgPromptChars != 150 {
		t.Fatalf("平均 prompt 字符 = %v", summary.AvgPromptChars)
	}
	if summary.ToolFailureRate != 0.25 {
		t.Fatalf("工具失败率 = %v", summary.ToolFailureRate)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	summary := Summarize(nil)
	if summary.Total != 0 || summary.PassRate != 0 {
		t.Fatalf("空结果应返回零值: %#v", summary)
	}
}
