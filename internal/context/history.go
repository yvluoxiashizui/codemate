package context

import (
	"strconv"
	"strings"
)

// DefaultRecentWindow 是历史里保留完整细节的最近条目数。
const DefaultRecentWindow = 6

// toolOutputLimit 是单条工具输出进入历史时的上限，超出的部分不进上下文。
const toolOutputLimit = 4000

// HistoryEntry 是一条历史记录的最小描述。
type HistoryEntry struct {
	// Role 取 user、assistant、tool 三者之一。
	Role string
	// Text 是正文；工具条目里是输出的首段。
	Text string
	// ToolName 仅在工具条目里有值。
	ToolName string
	// Path 是工具操作的目标路径，用于旧历史的按路径去重。
	Path string
}

// Roles。
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// RenderHistory 把历史渲染成近期完整、旧条目降级的两层形态。
// 最近 recentWindow 条保留正文，其余压成一行；同一路径的工具调用只留第一次。
func RenderHistory(entries []HistoryEntry, recentWindow int) string {
	if len(entries) == 0 {
		return ""
	}
	if recentWindow <= 0 {
		recentWindow = DefaultRecentWindow
	}
	if recentWindow > len(entries) {
		recentWindow = len(entries)
	}

	split := len(entries) - recentWindow
	seen := map[string]bool{}
	lines := make([]string, 0, len(entries))

	for _, entry := range entries[:split] {
		line, key := degrade(entry)
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		lines = append(lines, line)
	}
	for _, entry := range entries[split:] {
		lines = append(lines, inline(entry))
	}
	return strings.Join(lines, "\n")
}

// inline 渲染保留细节的近期条目。
func inline(entry HistoryEntry) string {
	text := strings.TrimSpace(entry.Text)
	switch entry.Role {
	case RoleTool:
		label := entry.ToolName
		if entry.Path != "" {
			label += "(" + entry.Path + ")"
		}
		if text == "" {
			return "[tool] " + label
		}
		return "[tool] " + label + ": " + clipText(text, toolOutputLimit)
	case RoleUser:
		return "[user] " + text
	case RoleAssistant:
		return "[assistant] " + text
	default:
		return "[" + entry.Role + "] " + text
	}
}

// degrade 把旧条目压成一行，并返回去重键。
func degrade(entry HistoryEntry) (string, string) {
	if entry.Role == RoleTool {
		label := entry.ToolName
		if entry.Path != "" {
			label += "(" + entry.Path + ")"
		}
		key := ""
		if entry.Path != "" {
			key = entry.ToolName + "|" + entry.Path
		}
		return "[tool] " + label, key
	}
	return "[" + entry.Role + "] " + firstLine(entry.Text), ""
}

// clipText 把单条文本压到上限内，保留开头并标注被截掉的字节数。
func clipText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + " ...[truncated " + strconv.Itoa(len(text)-limit) + " chars]"
}

// firstLine 取首个非空行，并压缩到一行能容纳的长度。
func firstLine(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	const limit = 80
	if len(text) <= limit {
		return text
	}
	return text[:limit-3] + "..."
}
