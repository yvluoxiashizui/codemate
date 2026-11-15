package memory

import (
	"strings"
	"testing"
)

func TestOpenMissingFileGivesEmptyState(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if store.Render() != "" {
		t.Fatalf("空记忆不应渲染内容: %q", store.Render())
	}
}

func TestSaveAndReloadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	store.SetTaskSummary("修复登录失败")
	store.RememberFile("auth.go", "负责登录校验", "1:1")
	store.AppendNote("auth.go 的 token 过期时间写错")
	if err := store.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	state := reloaded.State()
	if state.TaskSummary != "修复登录失败" {
		t.Fatalf("任务摘要丢失: %q", state.TaskSummary)
	}
	if state.FileSummaries["auth.go"].Summary != "负责登录校验" {
		t.Fatalf("文件摘要丢失: %#v", state.FileSummaries)
	}
	if len(state.Notes) != 1 {
		t.Fatalf("笔记丢失: %#v", state.Notes)
	}
}

func TestRenderIncludesTaskAndFiles(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.SetTaskSummary("重构配置加载")
	store.RememberFile("config.go", "配置解析入口", "2:2")

	text := store.Render()
	if !strings.Contains(text, "任务摘要") || !strings.Contains(text, "config.go") {
		t.Fatalf("渲染内容不完整: %q", text)
	}
}

func TestInvalidateStaleDropsChangedFiles(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.RememberFile("a.go", "旧摘要", "1:1")
	store.RememberFile("b.go", "未变", "2:2")

	stale := store.InvalidateStale(func(path string) string {
		if path == "a.go" {
			return "9:9"
		}
		return "2:2"
	})
	if len(stale) != 1 || stale[0] != "a.go" {
		t.Fatalf("应只失效 a.go: %#v", stale)
	}
	if _, ok := store.State().FileSummaries["a.go"]; ok {
		t.Fatal("失效的文件摘要应被删除")
	}
	if _, ok := store.State().FileSummaries["b.go"]; !ok {
		t.Fatal("未变的文件摘要应保留")
	}
}

func TestRetrievalRanksByKeywordHit(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNote("登录接口返回 401，原因是 token 过期")
	store.AppendNote("数据库连接池配置不合理")
	store.AppendNote("token 刷新逻辑缺失")

	notes := store.Retrieval("token 过期", 2)
	if len(notes) == 0 {
		t.Fatal("应召回相关笔记")
	}
	if !strings.Contains(notes[0].Text, "token") {
		t.Fatalf("首条应命中 token: %#v", notes)
	}
	if len(notes) > 2 {
		t.Fatalf("应受 limit 限制: %d", len(notes))
	}
}

func TestRetrievalWithoutQueryReturnsNothing(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNote("随便记一条")
	if notes := store.Retrieval("", 3); len(notes) != 0 {
		t.Fatalf("空查询不应召回: %#v", notes)
	}
}

func TestFilesAreTrimmedToLimit(t *testing.T) {
	store, _ := Open(t.TempDir())
	for i := 0; i < MaxFileSummaries+8; i++ {
		store.RememberFile(strings.Repeat("a", 1)+string(rune('a'+i%26))+".go", "摘要", "1:1")
	}
	if got := len(store.State().FileSummaries); got > MaxFileSummaries {
		t.Fatalf("文件摘要数 = %d, 上限 %d", got, MaxFileSummaries)
	}
}

func TestNotesAreTrimmedToLimit(t *testing.T) {
	store, _ := Open(t.TempDir())
	for i := 0; i < MaxNotes+5; i++ {
		store.AppendNote("笔记")
	}
	if got := len(store.State().Notes); got != MaxNotes {
		t.Fatalf("笔记数 = %d, want %d", got, MaxNotes)
	}
}

func TestRenderNotes(t *testing.T) {
	text := RenderNotes([]Note{{Text: "第一条"}, {Text: "第二条"}})
	if !strings.Contains(text, "第一条") || !strings.Contains(text, "第二条") {
		t.Fatalf("渲染结果不符: %q", text)
	}
}
