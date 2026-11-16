package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 长期记忆的主题，与 pico 的分类保持一致。
const (
	TopicConventions  = "project-conventions"
	TopicDecisions    = "key-decisions"
	TopicDependencies = "dependency-facts"
	TopicPreferences  = "user-preferences"
)

// Promotion 是一条待提升为长期记忆的内容。
type Promotion struct {
	Topic string
	Text  string
}

// DurableNote 是长期记忆里的一条记录。
type DurableNote struct {
	Topic     string `json:"topic"`
	Text      string `json:"text"`
	CreatedAt string `json:"created_at"`
}

// DurableStore 保存跨会话有效的长期记忆。
type DurableStore struct {
	path  string
	notes []DurableNote
}

// OpenDurable 加载工作区的长期记忆，不存在时返回空存储。
func OpenDurable(workspace string) (*DurableStore, error) {
	path := filepath.Join(workspace, ".codemate", "durable.json")
	store := &DurableStore{path: path}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(payload, &store.notes); err != nil {
		return nil, err
	}
	return store, nil
}

// Save 写回磁盘。
func (d *DurableStore) Save() error {
	if d == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(d.notes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.path, append(payload, '\n'), 0o600)
}

// Notes 返回全部长期记忆。
func (d *DurableStore) Notes() []DurableNote {
	if d == nil {
		return nil
	}
	return append([]DurableNote(nil), d.notes...)
}

// Promote 把几条内容提升为长期记忆。
// 完全相同的跳过；同一主题同一主语的新记录替换旧记录。
func (d *DurableStore) Promote(promotions []Promotion) (promoted, superseded []string) {
	if d == nil || len(promotions) == 0 {
		return nil, nil
	}
	for _, item := range promotions {
		topic := normalizeTopic(item.Topic)
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		duplicate := false
		for _, existing := range d.notes {
			if existing.Topic == topic && existing.Text == text {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		replaced := false
		subject := subjectKey(text)
		if subject != "" {
			for index, existing := range d.notes {
				if existing.Topic != topic {
					continue
				}
				if subjectKey(existing.Text) == subject {
					superseded = append(superseded, topic+": "+existing.Text+" -> "+text)
					d.notes[index].Text = text
					d.notes[index].CreatedAt = now()
					replaced = true
					break
				}
			}
		}
		if !replaced {
			d.notes = append(d.notes, DurableNote{Topic: topic, Text: text, CreatedAt: now()})
		}
		promoted = append(promoted, topic+": "+text)
	}
	return promoted, superseded
}

// ByTopic 返回某个主题下的记录。
func (d *DurableStore) ByTopic(topic string) []DurableNote {
	if d == nil {
		return nil
	}
	topic = normalizeTopic(topic)
	out := make([]DurableNote, 0, len(d.notes))
	for _, note := range d.notes {
		if note.Topic == topic {
			out = append(out, note)
		}
	}
	return out
}

func normalizeTopic(topic string) string {
	switch strings.TrimSpace(strings.ToLower(topic)) {
	case TopicConventions:
		return TopicConventions
	case TopicDecisions:
		return TopicDecisions
	case TopicDependencies:
		return TopicDependencies
	case TopicPreferences:
		return TopicPreferences
	default:
		return TopicConventions
	}
}

// subjectKey 取一条记录的主语，用于判断新旧记录是否在说同一件事。
func subjectKey(text string) string {
	fields := strings.Fields(strings.ToLower(text))
	if len(fields) == 0 {
		return ""
	}
	// 主语取第一个词：同一件事的新旧记录通常以同一个词开头。
	return strings.Trim(fields[0], "：:,，。.")
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
