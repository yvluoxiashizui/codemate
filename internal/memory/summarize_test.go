package memory

import (
	"strings"
	"testing"
)

func TestSummarizeReadResultJoinsFirstThreeLines(t *testing.T) {
	got := SummarizeReadResult("package main\nimport \"fmt\"\nfunc main() {}\nvar x int\n", 180)

	if strings.Contains(got, "var x") {
		t.Fatalf("只应保留前三行: %q", got)
	}
	if !strings.Contains(got, "package main | import \"fmt\" | func main()") {
		t.Fatalf("摘要格式不符: %q", got)
	}
}

func TestSummarizeReadResultDropsTitle(t *testing.T) {
	got := SummarizeReadResult("# 设计说明\n第一段\n第二段\n", 180)
	if strings.HasPrefix(got, "# ") {
		t.Fatalf("应去掉 Markdown 标题: %q", got)
	}
	if !strings.Contains(got, "第一段") {
		t.Fatalf("摘要缺少正文: %q", got)
	}
}

func TestSummarizeReadResultEmpty(t *testing.T) {
	if got := SummarizeReadResult("\n\n  \n", 180); got != "(empty)" {
		t.Fatalf("空内容 = %q", got)
	}
}

func TestSummarizeReadResultRespectsLimit(t *testing.T) {
	long := strings.Repeat("很长的一行内容", 50)
	got := SummarizeReadResult(long, 20)
	if len([]rune(got)) > 40 {
		t.Fatalf("应受长度限制: %d", len([]rune(got)))
	}
}
