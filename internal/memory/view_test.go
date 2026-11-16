package memory

import (
	"strings"
	"testing"
)

func TestRenderViewWithNotes(t *testing.T) {
	view := RenderView([]Note{{Text: "token 过期时间写错"}}, nil)
	if !strings.HasPrefix(view, "Relevant memory:") {
		t.Fatalf("应带标题: %q", view)
	}
	if !strings.Contains(view, "- token 过期时间写错") {
		t.Fatalf("缺少笔记内容: %q", view)
	}
}

func TestRenderViewEmptySaysNone(t *testing.T) {
	view := RenderView(nil, nil)
	if !strings.Contains(view, "- none") {
		t.Fatalf("空结果应显示 none: %q", view)
	}
}

func TestRenderViewGroupsDurableByTopic(t *testing.T) {
	view := RenderView(nil, []DurableNote{
		{Topic: TopicDecisions, Text: "数据库用 mysql"},
		{Topic: TopicDecisions, Text: "缓存用 redis"},
		{Topic: TopicPreferences, Text: "回答简洁"},
	})

	if !strings.Contains(view, "- [key-decisions]") {
		t.Fatalf("应按主题分组: %q", view)
	}
	if strings.Count(view, "[key-decisions]") != 1 {
		t.Fatalf("同一主题只应出现一次标题: %q", view)
	}
	if !strings.Contains(view, "  - 缓存用 redis") {
		t.Fatalf("分组内应有缩进条目: %q", view)
	}
	if !strings.Contains(view, "- [user-preferences]") {
		t.Fatalf("第二个主题缺失: %q", view)
	}
}

func TestRenderViewSkipsBlankEntries(t *testing.T) {
	view := RenderView([]Note{{Text: "  "}}, []DurableNote{{Text: ""}})
	if !strings.Contains(view, "- none") {
		t.Fatalf("空内容应等同于没有内容: %q", view)
	}
}
