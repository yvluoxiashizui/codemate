package session

import (
	"os"
	"path/filepath"
	"testing"
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
	if len(record.Raw) != 1 || record.Raw[0].Text != "hi" {
		t.Fatalf("原始历史未迁移: %#v", record.Raw)
	}
	if len(record.Messages) != 1 {
		t.Fatalf("工作上下文未迁移: %#v", record.Messages)
	}
}
