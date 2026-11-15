package runs

import "testing"

func TestEvaluateResumeWithoutCheckpoint(t *testing.T) {
	result := EvaluateResume(Checkpoint{}, map[string]any{}, nil)
	if result.Status != ResumeNoCheckpoint {
		t.Fatalf("status = %s", result.Status)
	}
}

func TestEvaluateResumeSchemaMismatch(t *testing.T) {
	checkpoint := Checkpoint{SchemaVersion: "old-v0", RuntimeIdentity: map[string]any{"cwd": "/a"}}
	if got := EvaluateResume(checkpoint, map[string]any{"cwd": "/a"}, nil); got.Status != ResumeSchemaMismatch {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestEvaluateResumeWorkspaceMismatch(t *testing.T) {
	checkpoint := Checkpoint{
		SchemaVersion:   CheckpointSchemaVersion,
		RuntimeIdentity: map[string]any{"cwd": "/a", "workspace_fingerprint": "x"},
	}
	result := EvaluateResume(checkpoint, map[string]any{"cwd": "/b", "workspace_fingerprint": "x"}, nil)
	if result.Status != ResumeWorkspaceMismatch {
		t.Fatalf("status = %s", result.Status)
	}
	if len(result.MismatchFields) != 1 || result.MismatchFields[0] != "cwd" {
		t.Fatalf("mismatch fields = %#v", result.MismatchFields)
	}
}

func TestEvaluateResumePartialStale(t *testing.T) {
	checkpoint := Checkpoint{
		SchemaVersion:   CheckpointSchemaVersion,
		RuntimeIdentity: map[string]any{"cwd": "/a", "workspace_fingerprint": "x"},
		KeyFiles:        []KeyFile{{Path: "a.go", Freshness: "1:1"}, {Path: "b.go", Freshness: "2:2"}},
	}
	freshness := func(path string) string {
		if path == "a.go" {
			return "9:9"
		}
		return "2:2"
	}
	result := EvaluateResume(checkpoint, map[string]any{"cwd": "/a", "workspace_fingerprint": "x"}, freshness)
	if result.Status != ResumePartialStale {
		t.Fatalf("status = %s", result.Status)
	}
	if len(result.StaleFiles) != 1 || result.StaleFiles[0] != "a.go" {
		t.Fatalf("stale files = %#v", result.StaleFiles)
	}
}

func TestEvaluateResumeFullValid(t *testing.T) {
	checkpoint := Checkpoint{
		SchemaVersion:   CheckpointSchemaVersion,
		RuntimeIdentity: map[string]any{"cwd": "/a", "workspace_fingerprint": "x", "max_steps": 24},
		KeyFiles:        []KeyFile{{Path: "a.go", Freshness: "1:1"}},
	}
	identity := map[string]any{"cwd": "/a", "workspace_fingerprint": "x", "max_steps": 24}
	result := EvaluateResume(checkpoint, identity, func(string) string { return "1:1" })
	if result.Status != ResumeFullValid {
		t.Fatalf("status = %s", result.Status)
	}
}
