package runs

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// CheckpointSchemaVersion 是检查点格式版本，格式变更时递增。
const CheckpointSchemaVersion = "phase1-v1"

// KeyFile 记录一个被访问过的文件及其新鲜度指纹。
type KeyFile struct {
	Path      string `json:"path"`
	Freshness string `json:"freshness"`
}

// Checkpoint 是一次运行留下的恢复依据。
type Checkpoint struct {
	SchemaVersion   string         `json:"schema_version"`
	CreatedAt       string         `json:"created_at"`
	RuntimeIdentity map[string]any `json:"runtime_identity"`
	KeyFiles        []KeyFile      `json:"key_files"`
}

// SaveCheckpoint 把检查点写进运行目录，返回文件路径。
func (r *Run) SaveCheckpoint(checkpoint Checkpoint) (string, error) {
	if r == nil {
		return "", nil
	}
	if checkpoint.SchemaVersion == "" {
		checkpoint.SchemaVersion = CheckpointSchemaVersion
	}
	payload, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(r.Dir, "checkpoint.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(payload, '\n'), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

// LoadCheckpoint 读取运行目录里的检查点。
func LoadCheckpoint(path string) (Checkpoint, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Checkpoint{}, err
	}
	var checkpoint Checkpoint
	if err := json.Unmarshal(payload, &checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}
