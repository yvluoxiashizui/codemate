package protocol

import (
	"strings"
	"testing"
)

func TestParseJSONTool(t *testing.T) {
	action := Parse(`<tool>{"name":"read_file","args":{"path":"a.go"}}</tool>`)
	if action.Kind != KindTool {
		t.Fatalf("kind = %s, want tool", action.Kind)
	}
	if action.Tool != "read_file" {
		t.Fatalf("tool = %s", action.Tool)
	}
	if action.Args["path"] != "a.go" {
		t.Fatalf("args = %#v", action.Args)
	}
}

func TestParseJSONToolWithoutArgs(t *testing.T) {
	action := Parse(`<tool>{"name":"list_files"}</tool>`)
	if action.Kind != KindTool || action.Tool != "list_files" {
		t.Fatalf("unexpected action: %#v", action)
	}
	if len(action.Args) != 0 {
		t.Fatalf("args should be empty: %#v", action.Args)
	}
}

func TestParseXMLToolWithContent(t *testing.T) {
	raw := `<tool name="write_file" path="a.go"><content>package main

func main() {}
</content></tool>`
	action := Parse(raw)
	if action.Kind != KindTool || action.Tool != "write_file" {
		t.Fatalf("unexpected action: %#v", action)
	}
	if action.Args["path"] != "a.go" {
		t.Fatalf("path = %v", action.Args["path"])
	}
	content, _ := action.Args["content"].(string)
	if !strings.Contains(content, "func main()") {
		t.Fatalf("content = %q", content)
	}
}

func TestParseXMLToolKeepsMultiLineBody(t *testing.T) {
	raw := "<tool name=\"write_file\" path=\"b.go\">\npackage b\n</tool>"
	action := Parse(raw)
	content, _ := action.Args["content"].(string)
	if !strings.Contains(content, "package b") {
		t.Fatalf("body should become content: %#v", action.Args)
	}
}

func TestParseFinal(t *testing.T) {
	action := Parse("<final>已经修好了。</final>")
	if action.Kind != KindFinal || action.Final != "已经修好了。" {
		t.Fatalf("unexpected action: %#v", action)
	}
}

func TestParsePlainTextAsFinal(t *testing.T) {
	action := Parse("  直接给答案  ")
	if action.Kind != KindFinal || action.Final != "直接给答案" {
		t.Fatalf("unexpected action: %#v", action)
	}
}

func TestParseMalformedJSONRetries(t *testing.T) {
	action := Parse("<tool>{not json}</tool>")
	if action.Kind != KindRetry {
		t.Fatalf("kind = %s, want retry", action.Kind)
	}
	if action.Notice == "" {
		t.Fatal("retry 应带提示")
	}
}

func TestParseMissingToolNameRetries(t *testing.T) {
	action := Parse(`<tool>{"args":{"path":"a.go"}}</tool>`)
	if action.Kind != KindRetry {
		t.Fatalf("kind = %s, want retry", action.Kind)
	}
}

func TestParseNonObjectArgsRetries(t *testing.T) {
	action := Parse(`<tool>{"name":"read_file","args":[1,2]}</tool>`)
	if action.Kind != KindRetry {
		t.Fatalf("kind = %s, want retry", action.Kind)
	}
}

func TestParseEmptyRetries(t *testing.T) {
	if action := Parse("   "); action.Kind != KindRetry {
		t.Fatalf("kind = %s, want retry", action.Kind)
	}
}

func TestParsePrefersEarlierBlock(t *testing.T) {
	raw := `<final>先给答案</final><tool>{"name":"read_file","args":{}}</tool>`
	action := Parse(raw)
	if action.Kind != KindFinal {
		t.Fatalf("final 在前时应取 final, got %s", action.Kind)
	}

	raw = `<tool>{"name":"read_file","args":{}}</tool><final>后面补一句</final>`
	action = Parse(raw)
	if action.Kind != KindTool {
		t.Fatalf("tool 在前时应取 tool, got %s", action.Kind)
	}
}

func TestNoticeTextIsActionable(t *testing.T) {
	text := NoticeText(Parse("<tool>{bad}</tool>"))
	if !strings.Contains(text, "<tool>") || !strings.Contains(text, "<final>") {
		t.Fatalf("提示应说明正确格式: %s", text)
	}
}

func TestBuildPrefixIsStable(t *testing.T) {
	tools := []ToolSpec{
		{Name: "read_file", Description: "读文件", Params: []Param{{Name: "path", Type: "string"}}, Risky: false},
		{Name: "write_file", Description: "写文件", Params: []Param{{Name: "path", Type: "string"}}, Risky: true},
	}
	workspace := Workspace{Root: "/tmp/repo", Files: []string{"a.go", "b.go"}}

	first := BuildPrefix(workspace, tools)
	second := BuildPrefix(workspace, tools)
	if first != second {
		t.Fatal("同一输入两次构建结果应完全一致")
	}

	reordered := BuildPrefix(workspace, []ToolSpec{tools[1], tools[0]})
	if first != reordered {
		t.Fatal("工具顺序不应影响前缀")
	}
}

func TestBuildPrefixDescribesTools(t *testing.T) {
	prefix := BuildPrefix(Workspace{Root: "/tmp/repo"}, []ToolSpec{
		{Name: "write_file", Description: "写文件", Params: []Param{{Name: "path", Type: "string"}}, Risky: true},
	})
	if !strings.Contains(prefix, "write_file") || !strings.Contains(prefix, "approval required") {
		t.Fatalf("前缀应描述工具与审批要求: %s", prefix)
	}
	if !strings.Contains(prefix, "<final>") {
		t.Fatal("前缀应给出应答格式示例")
	}
}
