package session

import (
	"encoding/json"

	"fmt"
	"github.com/yvluoxiashizui/codemate/internal/security"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
)

type Store struct {
	Dir string
}

// RecordVersion 是当前存档格式版本。
const RecordVersion = 2

type Record struct {
	Version        int                   `json:"version"`
	ID             string                `json:"id"`
	CWD            string                `json:"cwd,omitempty"`
	CreatedAt      time.Time             `json:"createdAt"`
	UpdatedAt      time.Time             `json:"updatedAt"`
	Fingerprint    string                `json:"fingerprint,omitempty"`
	CheckpointPath string                `json:"checkpoint_path,omitempty"`
	Raw            []ctxpkg.HistoryEntry `json:"raw"`
	Messages       []ctxpkg.HistoryEntry `json:"working"`
}

type Summary struct {
	ID           string
	CWD          string
	UpdatedAt    time.Time
	MessageCount int
}

func NewRecord(cwd string, entries []ctxpkg.HistoryEntry) Record {
	now := time.Now().UTC()
	return Record{
		Version:   RecordVersion,
		ID:        now.Format("20060102-150405"),
		CWD:       cwd,
		CreatedAt: now,
		UpdatedAt: now,
		Raw:       append([]ctxpkg.HistoryEntry(nil), entries...),
		Messages:  append([]ctxpkg.HistoryEntry(nil), entries...),
	}
}

func (s Store) Save(record Record) error {
	if strings.TrimSpace(record.ID) == "" {
		record.ID = time.Now().UTC().Format("20060102-150405")
	}
	path, err := s.path(record.ID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	bytes = []byte(security.Redact(string(bytes), nil))
	return os.WriteFile(path, append(bytes, '\n'), 0o644)
}

func (s Store) Load(id string) (Record, error) {
	path, err := s.path(id)
	if err != nil {
		return Record{}, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var payload recordPayload
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return Record{}, err
	}
	return payload.toRecord(), nil
}

// recordPayload 是存档的磁盘表示，保留 v1 的 messages 字段用于迁移。
type recordPayload struct {
	Version   int                   `json:"version"`
	ID        string                `json:"id"`
	CWD       string                `json:"cwd,omitempty"`
	CreatedAt time.Time             `json:"createdAt"`
	UpdatedAt time.Time             `json:"updatedAt"`
	Raw       []ctxpkg.HistoryEntry `json:"raw"`
	Messages  []ctxpkg.HistoryEntry `json:"working"`
	Legacy    []legacyMessage       `json:"messages"`
}

// legacyMessage 是 v1 存档里的消息形态，只用于迁移。
type legacyMessage struct {
	Role    string `json:"Role"`
	Content string `json:"Content"`
}

func (p recordPayload) toRecord() Record {
	record := Record{
		Version:   p.Version,
		ID:        p.ID,
		CWD:       p.CWD,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
		Raw:       p.Raw,
		Messages:  p.Messages,
	}
	if record.Version < RecordVersion {
		if len(record.Raw) == 0 {
			for _, item := range p.Legacy {
				record.Raw = append(record.Raw, ctxpkg.HistoryEntry{Role: item.Role, Text: item.Content})
			}
		}
		if len(record.Messages) == 0 {
			record.Messages = record.Raw
		}
		record.Version = RecordVersion
	}
	return record
}

func (s Store) Latest() (Record, error) {
	summaries, err := s.List()
	if err != nil {
		return Record{}, err
	}
	if len(summaries) == 0 {
		return Record{}, fmt.Errorf("no saved sessions")
	}
	return s.Load(summaries[0].ID)
}

func (s Store) List() ([]Summary, error) {
	entries, err := os.ReadDir(s.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Summary{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		record, err := s.Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, Summary{ID: record.ID, CWD: record.CWD, UpdatedAt: record.UpdatedAt, MessageCount: len(record.Messages)})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s Store) path(id string) (string, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return "", fmt.Errorf("session store directory is empty")
	}
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("session id is empty")
	}
	if id != filepath.Base(id) || strings.Contains(id, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid session id: %s", id)
	}
	return filepath.Join(s.Dir, id+".json"), nil
}

// Resumable 判断存档能否在当前工作区恢复。
// 没有记录指纹的旧存档按可恢复处理，指纹不一致时拒绝。
func Resumable(record Record, fingerprint string) bool {
	if record.Fingerprint == "" {
		return true
	}
	return record.Fingerprint == fingerprint
}
