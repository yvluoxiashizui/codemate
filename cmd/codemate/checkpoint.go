package main

import (
	"time"

	"github.com/yvluoxiashizui/codemate/internal/agent"
	"github.com/yvluoxiashizui/codemate/internal/runs"
	"github.com/yvluoxiashizui/codemate/internal/workspace"
)

// checkpointWriter 在每轮结束后把检查点写进运行目录。
type checkpointWriter struct {
	run         *runs.Run
	cwd         string
	model       string
	fingerprint string
	maxSteps    int
}

// WriteCheckpoint 写入检查点并返回文件路径。
func (w checkpointWriter) WriteCheckpoint(result agent.RunResult) string {
	if w.run == nil {
		return ""
	}
	keyFiles := make([]runs.KeyFile, 0, len(result.TouchedPaths))
	for _, path := range result.TouchedPaths {
		keyFiles = append(keyFiles, runs.KeyFile{
			Path:      path,
			Freshness: workspace.Freshness(w.cwd, path),
		})
	}
	checkpoint := runs.Checkpoint{
		SchemaVersion: runs.CheckpointSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		RuntimeIdentity: map[string]any{
			"cwd":                   w.cwd,
			"model":                 w.model,
			"max_steps":             w.maxSteps,
			"workspace_fingerprint": w.fingerprint,
			"status":                string(result.State.Status),
			"stop_reason":           string(result.State.StopReason),
		},
		KeyFiles: keyFiles,
	}
	path, err := w.run.SaveCheckpoint(checkpoint)
	if err != nil {
		return ""
	}
	w.run.Event("checkpoint_created", map[string]any{
		"path":        path,
		"key_files":   len(keyFiles),
		"status":      string(result.State.Status),
		"stop_reason": string(result.State.StopReason),
	})
	return path
}
