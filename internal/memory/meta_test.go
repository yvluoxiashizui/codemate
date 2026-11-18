package memory

import (
	"strings"
	"testing"
)

func TestTagMatchOutranksKeywordOverlap(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNoteWithMeta(Note{Text: "缓存穿透的三种处理方式", Tags: []string{"redis"}})
	store.AppendNote("redis 连接超时排查记录")

	notes := store.Retrieval("redis", 2)
	if len(notes) != 2 {
		t.Fatalf("两条都应命中: %d", len(notes))
	}
	if !strings.Contains(notes[0].Tags[0], "redis") {
		t.Fatalf("标签命中应排在前: %#v", notes[0])
	}
}

func TestNewerNotesWinWhenScoresTie(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNote("token 过期问题")
	store.AppendNote("token 过期问题的修复")

	notes := store.Retrieval("token", 1)
	if len(notes) != 1 {
		t.Fatalf("应返回一条: %#v", notes)
	}
	if !strings.Contains(notes[0].Text, "修复") {
		t.Fatalf("同分时新的优先: %q", notes[0].Text)
	}
}

func TestSourceParticipatesInMatching(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNoteWithMeta(Note{Text: "连接池参数", Source: "db/config.go"})

	if notes := store.Retrieval("config", 1); len(notes) != 1 {
		t.Fatalf("来源应参与匹配: %#v", notes)
	}
}

func TestNotesCarryIndex(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNote("第一条")
	store.AppendNote("第二条")

	state := store.State()
	if state.Notes[0].Index != 0 || state.Notes[1].Index != 1 {
		t.Fatalf("序号不连续: %#v", state.Notes)
	}
}

func TestRenderRespectsBudget(t *testing.T) {
	store, _ := Open(t.TempDir())
	for i := 0; i < 40; i++ {
		store.RememberFile(strings.Repeat("a", 1)+string(rune('a'+i%26))+"_"+string(rune('a'+i/26))+".go", strings.Repeat("长摘要内容", 20), "1:1")
	}
	text := store.Render()
	if len(text) > MemoryRenderBudget+8 {
		t.Fatalf("渲染结果应受预算限制: %d", len(text))
	}
}

func TestRetrievalWithoutMatchesReturnsNone(t *testing.T) {
	store, _ := Open(t.TempDir())
	store.AppendNote("完全无关的一条记录")
	if notes := store.Retrieval("zzz", 3); len(notes) != 0 {
		t.Fatalf("无命中应返回空: %#v", notes)
	}
}
