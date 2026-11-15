package runs

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event 是一次运行中的一条结构化事件。
type Event struct {
	Seq  int            `json:"seq"`
	TS   string         `json:"ts"`
	Type string         `json:"type"`
	Data map[string]any `json:"data,omitempty"`
}

// Store 管理工作区下的运行工件目录。
type Store struct {
	Root string
}

// NewStore 返回工作区内的运行工件存储。
func NewStore(workspace string) Store {
	return Store{Root: filepath.Join(workspace, ".codemate", "runs")}
}

// Start 创建一个新的运行目录并写入起始事件。
func (s Store) Start(meta map[string]any) (*Run, error) {
	id := newRunID()
	dir := filepath.Join(s.Root, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "events.jsonl")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	run := &Run{ID: id, Dir: dir, Path: path, file: file}
	run.Event("run.start", meta)
	return run, nil
}

// Run 是一次运行的工件句柄，事件按追加顺序落盘。
type Run struct {
	ID   string
	Dir  string
	Path string

	mu   sync.Mutex
	seq  int
	file *os.File
	err  error
}

// Event 追加一条事件。写入失败不会打断调用方，只记录首个错误。
func (r *Run) Event(typ string, data map[string]any) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file == nil {
		return
	}
	r.seq++
	payload, err := json.Marshal(Event{
		Seq:  r.seq,
		TS:   time.Now().UTC().Format(time.RFC3339Nano),
		Type: typ,
		Data: data,
	})
	if err != nil {
		if r.err == nil {
			r.err = err
		}
		return
	}
	if _, err := r.file.Write(append(payload, byte(10))); err != nil && r.err == nil {
		r.err = err
	}
}

// Close 写入结束事件并关闭文件。
func (r *Run) Close(reason string) error {
	if r == nil {
		return nil
	}
	r.Event("run.end", map[string]any{"reason": reason})
	r.mu.Lock()
	defer r.mu.Unlock()
	err := r.err
	if r.file != nil {
		if closeErr := r.file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		r.file = nil
	}
	return err
}

// SaveState 把任务状态快照写进运行目录。每次都覆盖，保留最新一份。
func (r *Run) SaveState(snapshot map[string]any) error {
	if r == nil {
		return nil
	}
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(r.Dir, "task_state.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(payload, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadEvents 读取一个工件文件里的全部事件。
func ReadEvents(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]Event, 0, 64)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		event := Event{}
		if err := json.Unmarshal(line, &event); err != nil {
			return events, fmt.Errorf("解析事件失败: %w", err)
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

func newRunID() string {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return time.Now().UTC().Format("20060102T150405") + "-" + hex.EncodeToString(suffix)
}
