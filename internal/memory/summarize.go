package memory

import "strings"

// ReadSummaryLimit 是读文件结果摘要的默认长度上限。
const ReadSummaryLimit = 180

// SummarizeReadResult 把读文件的结果压成短摘要。
// 不保留文件正文，只保留足够提醒下一轮“读到了什么”的信息。
func SummarizeReadResult(result string, limit int) string {
	if limit <= 0 {
		limit = ReadSummaryLimit
	}
	lines := make([]string, 0, 8)
	for _, line := range strings.Split(result, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		lines = lines[1:]
	}
	if len(lines) == 0 {
		return "(empty)"
	}
	if len(lines) > 3 {
		lines = lines[:3]
	}
	return clip(strings.Join(lines, " | "), limit)
}
