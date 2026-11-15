package context

import (
	"strings"
	"testing"
)

func TestMiddleTextKeepsHeadAndTail(t *testing.T) {
	text := strings.Repeat("a", 100) + "TAIL"
	got := middleText(text, 40)

	if !strings.HasPrefix(got, "aaa") {
		t.Fatalf("应保留开头: %q", got)
	}
	if !strings.HasSuffix(got, "TAIL") {
		t.Fatalf("应保留结尾: %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("应有省略标记: %q", got)
	}
	if len(got) != 40 {
		t.Fatalf("长度 = %d, want 40", len(got))
	}
}

func TestMiddleTextFlattensNewlines(t *testing.T) {
	got := middleText("第一行\n第二行\n第三行", 80)
	if strings.Contains(got, "\n") {
		t.Fatalf("应压平换行: %q", got)
	}
	if !strings.Contains(got, "第一行 第二行") {
		t.Fatalf("压平后应保留空格: %q", got)
	}
}

func TestMiddleTextShortTextUnchanged(t *testing.T) {
	if got := middleText("short", 80); got != "short" {
		t.Fatalf("未超长不应改动: %q", got)
	}
}

func TestMiddleTextEdgeLimits(t *testing.T) {
	if got := middleText("abcdef", 3); got != "abc" {
		t.Fatalf("limit 3 时应直接截断: %q", got)
	}
	if got := middleText("abcdef", 1); got != "a" {
		t.Fatalf("limit 1 时应截断: %q", got)
	}
}

func TestDegradedHistoryUsesMiddleSummary(t *testing.T) {
	entries := make([]HistoryEntry, 0, 10)
	entries = append(entries, HistoryEntry{Role: RoleUser, Text: "开头"})
	for i := 0; i < 6; i++ {
		entries = append(entries, HistoryEntry{Role: RoleAssistant, Text: "中间步骤"})
	}
	long := strings.Repeat("x", 200) + "重要结尾"
	entries = append(entries, HistoryEntry{Role: RoleAssistant, Text: long})

	rendered := RenderHistory(entries, 6)
	if !strings.Contains(rendered, "重要结尾") {
		t.Fatalf("降级摘要应保留结尾: %q", rendered)
	}
}
