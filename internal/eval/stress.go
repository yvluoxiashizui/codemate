package eval

import (
	"strings"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
)

// StressPoint 是一个历史长度下的上下文装配结果。
type StressPoint struct {
	Entries     int
	PromptChars int
	Reductions  int
	OverBudget  bool
}

// MeasureStress 用递增的历史长度观察上下文装配。
// 关注三点：prompt 是否随历史线性膨胀、收缩有没有发生、超预算后还剩多少。
func MeasureStress(prefix string, sizes []int, budget int) []StressPoint {
	points := make([]StressPoint, 0, len(sizes))
	for _, size := range sizes {
		history := syntheticHistory(size)
		built := ctxpkg.Build(ctxpkg.Sections{
			Prefix:         prefix,
			History:        ctxpkg.RenderHistory(history, 0),
			CurrentRequest: "继续",
		}, ctxpkg.Options{TotalBudget: budget})

		points = append(points, StressPoint{
			Entries:     size,
			PromptChars: len(built.Prompt),
			Reductions:  len(built.Reductions),
			OverBudget:  built.Prompt != "" && len(built.Prompt) > budget,
		})
	}
	return points
}

// syntheticHistory 造出指定条数的历史，内容长度接近真实工具输出。
func syntheticHistory(size int) []ctxpkg.HistoryEntry {
	entries := make([]ctxpkg.HistoryEntry, 0, size)
	body := strings.Repeat("读取到的内容片段 ", 20)
	for i := 0; i < size; i++ {
		role := ctxpkg.RoleAssistant
		if i%3 == 0 {
			role = ctxpkg.RoleUser
		}
		entries = append(entries, ctxpkg.HistoryEntry{
			Role: role,
			Text: body,
		})
	}
	return entries
}
