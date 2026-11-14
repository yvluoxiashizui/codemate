package session

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
)

func TestLoadMigratesLegacyMessages(t *testing.T) {
	dir := t.TempDir()
	store := Store{Dir: dir}
	legacy := `{"id":"legacy","cwd":"/tmp/p","messages":[{"Role":"user","Content":"hi"}]}`
	if err := os.WriteFile(filepath.Join(dir, "legacy.json"), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	record, err := store.Load("legacy")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if record.Version != RecordVersion {
		t.Fatalf("version = %d, want %d", record.Version, RecordVersion)
	}
	if len(record.Raw) != 1 || record.Raw[0].Content != "hi" {
		t.Fatalf("原始历史未迁移: %#v", record.Raw)
	}
	if len(record.Messages) != 1 {
		t.Fatalf("工作上下文未迁移: %#v", record.Messages)
	}
}

func TestPersistKeepsRawHistorySeparateFromWorkingContext(t *testing.T) {
	dir := t.TempDir()
	s := New(Args{
		Store:     Store{Dir: dir},
		SessionID: "s1",
		CWD:       "/tmp/p",
		Messages:  []message.Message{message.SystemMessage("sys")},
	})
	s.raw = []message.Message{
		message.SystemMessage("sys"),
		message.UserMessage("old-1"),
		message.UserMessage("old-2"),
	}
	s.args.Messages = []message.Message{
		message.SystemMessage("sys"),
		message.UserMessage("以下是此前会话的摘要：old"),
	}
	if err := s.persistMessages(); err != nil {
		t.Fatalf("persist: %v", err)
	}

	loaded, err := s.args.Store.Load("s1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Raw) != 3 {
		t.Fatalf("原始历史条数 = %d, want 3", len(loaded.Raw))
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("工作上下文条数 = %d, want 2", len(loaded.Messages))
	}
}
