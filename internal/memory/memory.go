package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 默认上限，防止记忆本身长成第二份历史。
const (
	MaxFileSummaries = 32
	MaxNotes         = 64
	MaxSummaryChars  = 300
	MaxNoteChars     = 500
)

// FileSummary 记录一个文件的一句话摘要及其新鲜度。
type FileSummary struct {
	Summary   string `json:"summary"`
	Freshness string `json:"freshness"`
	UpdatedAt string `json:"updated_at"`
}

// Note 是一条过程笔记。
type Note struct {
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// State 是被持久化的记忆内容。
type State struct {
	TaskSummary   string                 `json:"task_summary"`
	FileSummaries map[string]FileSummary `json:"file_summaries"`
	Notes         []Note                 `json:"notes"`
}

// Store 负责记忆的读写与召回。
type Store struct {
	path  string
	state State
}

// NewState 返回一份空记忆。
func NewState() State {
	return State{FileSummaries: map[string]FileSummary{}}
}

// Open 从工作区加载记忆，不存在时返回空记忆。
func Open(workspace string) (*Store, error) {
	path := filepath.Join(workspace, ".codemate", "memory.json")
	store := &Store{path: path, state: NewState()}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	var state State
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, err
	}
	if state.FileSummaries == nil {
		state.FileSummaries = map[string]FileSummary{}
	}
	store.state = state
	return store, nil
}

// Save 把记忆写回磁盘。
func (s *Store) Save() error {
	if s == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(payload, '\n'), 0o600)
}

// State 返回记忆快照。
func (s *Store) State() State {
	if s == nil {
		return NewState()
	}
	return s.state
}

// SetTaskSummary 记录本次任务的意图，超出上限时截断。
func (s *Store) SetTaskSummary(text string) {
	if s == nil {
		return
	}
	s.state.TaskSummary = clip(strings.TrimSpace(text), MaxSummaryChars)
}

// RememberFile 记录一个文件的一句话摘要与其新鲜度。
func (s *Store) RememberFile(path, summary, freshness string) {
	if s == nil || strings.TrimSpace(path) == "" {
		return
	}
	if s.state.FileSummaries == nil {
		s.state.FileSummaries = map[string]FileSummary{}
	}
	s.state.FileSummaries[path] = FileSummary{
		Summary:   clip(strings.TrimSpace(summary), MaxSummaryChars),
		Freshness: freshness,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.trimFiles()
}

// AppendNote 追加一条过程笔记。
func (s *Store) AppendNote(text string) {
	if s == nil {
		return
	}
	text = clip(strings.TrimSpace(text), MaxNoteChars)
	if text == "" {
		return
	}
	s.state.Notes = append(s.state.Notes, Note{
		Text:      text,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	if len(s.state.Notes) > MaxNotes {
		s.state.Notes = s.state.Notes[len(s.state.Notes)-MaxNotes:]
	}
}

// InvalidateStale 删除新鲜度已经变化的文件摘要，返回被删掉的路径。
func (s *Store) InvalidateStale(freshness func(path string) string) []string {
	if s == nil || freshness == nil {
		return nil
	}
	stale := make([]string, 0, 4)
	for path, entry := range s.state.FileSummaries {
		if freshness(path) != entry.Freshness {
			stale = append(stale, path)
			delete(s.state.FileSummaries, path)
		}
	}
	sort.Strings(stale)
	return stale
}

// Retrieval 按关键词命中数召回笔记，返回前 limit 条。
func (s *Store) Retrieval(query string, limit int) []Note {
	if s == nil {
		return nil
	}
	tokens := tokenize(query)
	if len(tokens) == 0 || len(s.state.Notes) == 0 {
		return nil
	}
	type scored struct {
		note  Note
		score int
	}
	ranked := make([]scored, 0, len(s.state.Notes))
	for _, note := range s.state.Notes {
		score := 0
		lower := strings.ToLower(note.Text)
		for _, token := range tokens {
			if strings.Contains(lower, token) {
				score++
			}
		}
		if score > 0 {
			ranked = append(ranked, scored{note: note, score: score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if limit > 0 && len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]Note, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.note)
	}
	return out
}

// Render 把记忆渲染成上下文的「记忆」一节。
func (s *Store) Render() string {
	if s == nil {
		return ""
	}
	lines := make([]string, 0, len(s.state.FileSummaries)+2)
	if s.state.TaskSummary != "" {
		lines = append(lines, "任务摘要: "+s.state.TaskSummary)
	}
	paths := make([]string, 0, len(s.state.FileSummaries))
	for path := range s.state.FileSummaries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		lines = append(lines, "- "+path+": "+s.state.FileSummaries[path].Summary)
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// RenderNotes 把召回的笔记渲染成「相关笔记」一节。
func RenderNotes(notes []Note) string {
	lines := make([]string, 0, len(notes))
	for _, note := range notes {
		lines = append(lines, "- "+note.Text)
	}
	return strings.Join(lines, "\n")
}

func (s *Store) trimFiles() {
	if len(s.state.FileSummaries) <= MaxFileSummaries {
		return
	}
	type entry struct {
		path string
		at   string
	}
	entries := make([]entry, 0, len(s.state.FileSummaries))
	for path, item := range s.state.FileSummaries {
		entries = append(entries, entry{path: path, at: item.UpdatedAt})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].at < entries[j].at })
	for i := 0; i < len(entries)-MaxFileSummaries; i++ {
		delete(s.state.FileSummaries, entries[i].path)
	}
}

func clip(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

func tokenize(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		case r > 127:
			return false
		default:
			return true
		}
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) >= 2 {
			out = append(out, field)
		}
	}
	return out
}
