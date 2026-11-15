package runs

import (
	"path/filepath"
	"testing"
)

func TestSaveCheckpointWritesSchemaAndKeyFiles(t *testing.T) {
	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	path, err := run.SaveCheckpoint(Checkpoint{
		RuntimeIdentity: map[string]any{"cwd": "/tmp/repo", "max_steps": 24},
		KeyFiles:        []KeyFile{{Path: "a.go", Freshness: "123:45"}},
	})
	if err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	if filepath.Base(path) != "checkpoint.json" {
		t.Fatalf("path = %s", path)
	}

	loaded, err := LoadCheckpoint(path)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if loaded.SchemaVersion != CheckpointSchemaVersion {
		t.Fatalf("schema = %s", loaded.SchemaVersion)
	}
	if len(loaded.KeyFiles) != 1 || loaded.KeyFiles[0].Path != "a.go" {
		t.Fatalf("key files = %#v", loaded.KeyFiles)
	}
	if loaded.RuntimeIdentity["cwd"] != "/tmp/repo" {
		t.Fatalf("identity = %#v", loaded.RuntimeIdentity)
	}
}

func TestLoadCheckpointMissingFile(t *testing.T) {
	if _, err := LoadCheckpoint(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("缺失文件应报错")
	}
}

func TestSaveCheckpointOnNilRunIsNoop(t *testing.T) {
	var run *Run
	path, err := run.SaveCheckpoint(Checkpoint{})
	if err != nil || path != "" {
		t.Fatalf("nil run 不应写文件: %q %v", path, err)
	}
}
