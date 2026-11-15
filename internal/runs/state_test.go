package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveStateWritesSnapshot(t *testing.T) {
	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := run.SaveState(map[string]any{"status": "running", "tool_steps": 2}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	path := filepath.Join(run.Dir, "task_state.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if snapshot["status"] != "running" {
		t.Fatalf("status = %v", snapshot["status"])
	}
	if snapshot["tool_steps"] != float64(2) {
		t.Fatalf("tool_steps = %v", snapshot["tool_steps"])
	}
}

func TestSaveStateOverwritesPreviousSnapshot(t *testing.T) {
	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if err := run.SaveState(map[string]any{"status": "running"}); err != nil {
		t.Fatal(err)
	}
	if err := run.SaveState(map[string]any{"status": "completed"}); err != nil {
		t.Fatal(err)
	}

	payload, err := os.ReadFile(filepath.Join(run.Dir, "task_state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["status"] != "completed" {
		t.Fatalf("应保留最新快照: %v", snapshot["status"])
	}
}

func TestSaveStateOnNilRunIsNoop(t *testing.T) {
	var run *Run
	if err := run.SaveState(map[string]any{"status": "x"}); err != nil {
		t.Fatalf("nil run 不应报错: %v", err)
	}
}
