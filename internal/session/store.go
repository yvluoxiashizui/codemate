package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yvluoxiashizui/codemate/internal/message"
)

type Store struct {
	Dir string
}

// RecordVersion 是当前存档格式版本。
const RecordVersion = 2

type Record struct {
	Version   int               `json:"version"`
	ID        string            `json:"id"`
	CWD       string            `json:"cwd,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Raw       []message.Message `json:"raw"`
	Messages  []message.Message `json:"working"`
}

type Summary struct {
	ID           string
	CWD          string
	UpdatedAt    time.Time
	MessageCount int
}

func NewRecord(cwd string, messages []message.Message) Record {
	now := time.Now().UTC()
	return Record{
		Version:   RecordVersion,
		ID:        now.Format("20060102-150405"),
		CWD:       cwd,
		CreatedAt: now,
		UpdatedAt: now,
		Raw:       append([]message.Message(nil), messages...),
		Messages:  append([]message.Message(nil), messages...),
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
	Version   int               `json:"version"`
	ID        string            `json:"id"`
	CWD       string            `json:"cwd,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Raw       []message.Message `json:"raw"`
	Messages  []message.Message `json:"working"`
	Legacy    []message.Message `json:"messages"`
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
			record.Raw = p.Legacy
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
