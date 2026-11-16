package memory

import (
	"strings"
	"testing"
)

func TestPromoteAppendsNewNotes(t *testing.T) {
	store, err := OpenDurable(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	promoted, superseded := store.Promote([]Promotion{
		{Topic: TopicConventions, Text: "所有错误都用 fmt.Errorf 包装"},
	})

	if len(promoted) != 1 || len(superseded) != 0 {
		t.Fatalf("promoted=%v superseded=%v", promoted, superseded)
	}
	if len(store.ByTopic(TopicConventions)) != 1 {
		t.Fatalf("主题记录数 = %d", len(store.ByTopic(TopicConventions)))
	}
}

func TestPromoteSkipsDuplicates(t *testing.T) {
	store, _ := OpenDurable(t.TempDir())
	note := Promotion{Topic: TopicConventions, Text: "统一用 gofmt"}

	store.Promote([]Promotion{note})
	promoted, _ := store.Promote([]Promotion{note})

	if len(promoted) != 0 {
		t.Fatalf("重复内容不应再次写入: %v", promoted)
	}
	if len(store.Notes()) != 1 {
		t.Fatalf("记录数 = %d", len(store.Notes()))
	}
}

func TestPromoteSupersedesSameSubject(t *testing.T) {
	store, _ := OpenDurable(t.TempDir())
	store.Promote([]Promotion{{Topic: TopicDecisions, Text: "database mysql 8 因为运维熟悉"}})

	_, superseded := store.Promote([]Promotion{{Topic: TopicDecisions, Text: "database postgres 15 之后统一迁移"}})
	if len(superseded) != 1 {
		t.Fatalf("同主语应被替换: %v", superseded)
	}

	notes := store.ByTopic(TopicDecisions)
	if len(notes) != 1 {
		t.Fatalf("替换后应只有一条: %#v", notes)
	}
	if !strings.Contains(notes[0].Text, "postgres") {
		t.Fatalf("保留的应是新记录: %q", notes[0].Text)
	}
}

func TestPromoteKeepsDifferentSubjects(t *testing.T) {
	store, _ := OpenDurable(t.TempDir())
	store.Promote([]Promotion{{Topic: TopicDecisions, Text: "database 用 mysql"}})
	store.Promote([]Promotion{{Topic: TopicDecisions, Text: "缓存 用 redis 集群"}})

	if len(store.ByTopic(TopicDecisions)) != 2 {
		t.Fatalf("不同主语应各自保留: %#v", store.ByTopic(TopicDecisions))
	}
}

func TestDurableRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, _ := OpenDurable(dir)
	store.Promote([]Promotion{{Topic: TopicPreferences, Text: "回答简洁"}})
	if err := store.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded, err := OpenDurable(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	notes := reloaded.ByTopic(TopicPreferences)
	if len(notes) != 1 || !strings.Contains(notes[0].Text, "简洁") {
		t.Fatalf("长期记忆未能持久化: %#v", notes)
	}
}

func TestUnknownTopicFallsBack(t *testing.T) {
	store, _ := OpenDurable(t.TempDir())
	store.Promote([]Promotion{{Topic: "whatever", Text: "随便一条"}})
	if len(store.ByTopic(TopicConventions)) != 1 {
		t.Fatalf("未知主题应归入默认主题: %#v", store.Notes())
	}
}
