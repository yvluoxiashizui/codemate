package runs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEventRedactsSecretValues(t *testing.T) {
	t.Setenv("CODEMATE_TEST_TOKEN", "eventsecretvalue123")

	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	run.Event("tool_executed", map[string]any{
		"output": "token=eventsecretvalue123",
	})
	if err := run.Close("done"); err != nil {
		t.Fatalf("close: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(run.Dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "eventsecretvalue123") {
		t.Fatalf("事件里出现明文密钥: %s", payload)
	}
	if !strings.Contains(string(payload), "[redacted:CODEMATE_TEST_TOKEN]") {
		t.Fatalf("应带占位符: %s", payload)
	}
}

func TestReportRedactsSecretValues(t *testing.T) {
	t.Setenv("CODEMATE_TEST_KEY", "reportsecretvalue999")

	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := run.SaveReport(map[string]any{"final_answer": "used reportsecretvalue999"}); err != nil {
		t.Fatalf("save report: %v", err)
	}

	payload, err := os.ReadFile(filepath.Join(run.Dir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "reportsecretvalue999") {
		t.Fatalf("报告里出现明文密钥: %s", payload)
	}
}

func TestCheckpointRedactsSecretValues(t *testing.T) {
	t.Setenv("CODEMATE_TEST_SECRET", "checkpointsecret777")

	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	path, err := run.SaveCheckpoint(Checkpoint{
		RuntimeIdentity: map[string]any{"cwd": "checkpointsecret777"},
	})
	if err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "checkpointsecret777") {
		t.Fatalf("检查点里出现明文密钥: %s", payload)
	}
}
