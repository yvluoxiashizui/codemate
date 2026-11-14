package runs

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestStartWritesRunArtifactUnderWorkspace(t *testing.T) {
	workspace := t.TempDir()
	store := NewStore(workspace)

	run, err := store.Start(map[string]any{"cwd": workspace})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	if want := filepath.Join(workspace, ".codemate", "runs"); filepath.Dir(run.Dir) != want {
		t.Fatalf("run dir = %s, want under %s", run.Dir, want)
	}
	if filepath.Base(run.Path) != "events.jsonl" {
		t.Fatalf("artifact path = %s", run.Path)
	}
	if err := run.Close("done"); err != nil {
		t.Fatalf("close run: %v", err)
	}

	events, err := ReadEvents(run.Path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Type != "run.start" || events[1].Type != "run.end" {
		t.Fatalf("unexpected event types: %s, %s", events[0].Type, events[1].Type)
	}
	if events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("seq not monotonic: %d, %d", events[0].Seq, events[1].Seq)
	}
	if events[1].Data["reason"] != "done" {
		t.Fatalf("run.end reason = %v", events[1].Data["reason"])
	}
}

func TestEventKeepsInsertionOrder(t *testing.T) {
	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	run.Event("model.call", map[string]any{"duration_ms": 12})
	run.Event("tool.call", map[string]any{"name": "read_file"})
	if err := run.Close("done"); err != nil {
		t.Fatalf("close run: %v", err)
	}

	events, err := ReadEvents(run.Path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	want := []string{"run.start", "model.call", "tool.call", "run.end"}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d", len(events), len(want))
	}
	for i, typ := range want {
		if events[i].Type != typ {
			t.Fatalf("event %d = %s, want %s", i, events[i].Type, typ)
		}
	}
}

func TestEventIsConcurrencySafe(t *testing.T) {
	run, err := NewStore(t.TempDir()).Start(nil)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run.Event("tool.result", map[string]any{"ok": true})
		}()
	}
	wg.Wait()
	if err := run.Close("done"); err != nil {
		t.Fatalf("close run: %v", err)
	}

	events, err := ReadEvents(run.Path)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if len(events) != 10 {
		t.Fatalf("got %d events, want 10", len(events))
	}
	for i, event := range events {
		if event.Seq != i+1 {
			t.Fatalf("event %d has seq %d", i, event.Seq)
		}
	}
}

func TestNilRunIsNoop(t *testing.T) {
	var run *Run
	run.Event("model.call", nil)
	if err := run.Close("done"); err != nil {
		t.Fatalf("close nil run: %v", err)
	}
}
