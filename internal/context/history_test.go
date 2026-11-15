package context

import (
	"strings"
	"testing"
)

func historySample() []HistoryEntry {
	return []HistoryEntry{
		{Role: RoleUser, Text: "先看看仓库结构"},
		{Role: RoleAssistant, Text: "我列一下目录"},
		{Role: RoleTool, ToolName: "list_files", Path: "."},
		{Role: RoleUser, Text: "读一下 a.go"},
		{Role: RoleTool, ToolName: "read_file", Path: "a.go", Text: "package a\n\nfunc A() {}"},
		{Role: RoleUser, Text: "再读一遍 a.go"},
		{Role: RoleTool, ToolName: "read_file", Path: "a.go", Text: "package a\n\nfunc A() {}"},
		{Role: RoleUser, Text: "修一下 B"},
		{Role: RoleAssistant, Text: "改好了"},
	}
}

func TestRenderHistoryKeepsRecentDetail(t *testing.T) {
	out := RenderHistory(historySample(), 3)
	lines := strings.Split(out, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "改好了") {
		t.Fatalf("最近的条目应保留正文: %q", last)
	}
}

func TestRenderHistoryDegradesOldEntries(t *testing.T) {
	out := RenderHistory(historySample(), 2)
	if !strings.Contains(out, "[user] 先看看仓库结构") {
		t.Fatalf("旧条目应压成一行: %q", out)
	}
	if strings.Contains(out, "func A() {}") && strings.Count(out, "func A() {}") > 1 {
		t.Fatal("旧工具输出不应重复出现")
	}
}

func TestRenderHistoryDedupesToolCallsByPath(t *testing.T) {
	entries := []HistoryEntry{
		{Role: RoleTool, ToolName: "read_file", Path: "a.go", Text: "first"},
		{Role: RoleTool, ToolName: "read_file", Path: "a.go", Text: "second"},
		{Role: RoleUser, Text: "继续"},
		{Role: RoleAssistant, Text: "好"},
	}
	out := RenderHistory(entries, 1)
	if strings.Count(out, "read_file(a.go)") != 1 {
		t.Fatalf("同一路径的旧工具调用应去重: %q", out)
	}
}

func TestRenderHistoryTruncatesLongLines(t *testing.T) {
	entries := []HistoryEntry{
		{Role: RoleUser, Text: strings.Repeat("长", 200)},
		{Role: RoleAssistant, Text: "最近一条"},
	}
	out := RenderHistory(entries, 1)
	if !strings.Contains(out, "...") {
		t.Fatalf("超长行应截断: %q", out)
	}
	if len([]rune(out)) > 120 {
		t.Fatalf("截断后仍过长: %d", len([]rune(out)))
	}
}

func TestRenderHistoryIsDeterministic(t *testing.T) {
	first := RenderHistory(historySample(), 3)
	second := RenderHistory(historySample(), 3)
	if first != second {
		t.Fatal("同一输入两次渲染结果应一致")
	}
}

func TestRenderHistoryEmpty(t *testing.T) {
	if RenderHistory(nil, 6) != "" {
		t.Fatal("空历史应返回空串")
	}
}

func TestRenderHistoryWindowLargerThanHistory(t *testing.T) {
	entries := []HistoryEntry{
		{Role: RoleUser, Text: "a"},
		{Role: RoleAssistant, Text: "b"},
	}
	out := RenderHistory(entries, 10)
	if !strings.Contains(out, "[user] a") || !strings.Contains(out, "[assistant] b") {
		t.Fatalf("窗口大于历史时应全部保留: %q", out)
	}
}

func TestBuildUsesRenderedHistory(t *testing.T) {
	history := RenderHistory(historySample(), 3)
	result := Build(Sections{History: history, CurrentRequest: "继续"}, Options{TotalBudget: 100000})
	if !strings.Contains(result.Prompt, "read_file(a.go)") {
		t.Fatalf("渲染后的历史应进入 prompt: %q", result.Prompt)
	}
}
