package compact

import (
	"strings"

	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/token"
)

const (
	// TriggerMargin 是触发压缩的剩余预算，低于此值时认为上下文接近窗口。
	TriggerMargin = 16384

	// KeepTokens 是压缩后保留的近期原文预算。
	KeepTokens = 20000

	// MaxPerTurn 是单个回合内允许的连续压缩次数，防止压缩不收敛。
	MaxPerTurn = 3

	summaryInstruction = `下面是一次编码会话中较早的消息记录。请把它压缩成一段摘要，供后续继续任务使用。

要求：
1. 保留已确认的目标、约束、修改过的文件路径与关键结论。
2. 保留仍未完成的事项与已知失败原因。
3. 不要臆测未出现的信息，不要复述原文。
4. 只输出摘要正文，不要加标题、前缀或代码块。`
)

// KeepTokensFor 返回给定窗口下应保留的近期原文预算。
// 保留量不超过窗口的四分之一，避免小窗口下永远达不到触发条件。
func KeepTokensFor(window int) int {
	if window <= 0 {
		window = token.DefaultWindow
	}
	keep := KeepTokens
	if limit := window / 4; limit < keep {
		keep = limit
	}
	if keep < 1 {
		keep = 1
	}
	return keep
}

// Should 判断当前上下文是否需要压缩。
func Should(budget token.Budget, keep int) bool {
	if keep <= 0 {
		keep = KeepTokens
	}
	return budget.Remaining < TriggerMargin && budget.Messages > keep
}

// Split 把上下文切成可压缩的前缀与需要保留的尾部。
// 系统消息始终保留在尾部之前，不会被压缩；工具调用与其结果不会被切开。
func Split(messages []message.Message, keep int) (head []message.Message, tail []message.Message) {
	if len(messages) == 0 {
		return nil, nil
	}

	start := 0
	if messages[0].Role == message.RoleSystem {
		start = 1
	}
	body := messages[start:]
	if len(body) == 0 {
		return nil, nil
	}

	boundary := len(body)
	accumulated := 0
	for boundary > 0 {
		accumulated += token.EstimateMessages(body[boundary-1 : boundary])
		if accumulated >= keep {
			break
		}
		boundary--
	}
	boundary = alignBoundary(body, boundary)
	if boundary <= 0 {
		return nil, nil
	}
	return body[:boundary], body[boundary:]
}

// alignBoundary 调整切分位置，避免把工具调用与对应的结果分开。
func alignBoundary(body []message.Message, boundary int) int {
	for boundary > 0 && boundary < len(body) && body[boundary].Role == message.RoleToolResult {
		boundary--
	}
	return boundary
}

// BuildRequest 构造一次摘要请求。
func BuildRequest(system message.Message, head []message.Message, window int) []message.Message {
	if window <= 0 {
		window = token.DefaultWindow
	}
	var builder strings.Builder
	for _, item := range head {
		builder.WriteString("[" + string(item.Role) + "] ")
		if item.ToolName != "" {
			builder.WriteString(item.ToolName + " ")
		}
		builder.WriteString(item.Content)
		builder.WriteString("\n")
	}
	request := make([]message.Message, 0, 2)
	if system.Content != "" {
		request = append(request, message.SystemMessage(system.Content))
	}
	request = append(request, message.UserMessage(summaryInstruction+"\n\n---\n"+builder.String()))
	return request
}

// Apply 用摘要替换前缀，返回压缩后的上下文。
func Apply(system message.Message, summary string, tail []message.Message) []message.Message {
	out := make([]message.Message, 0, len(tail)+2)
	if system.Content != "" {
		out = append(out, system)
	}
	out = append(out, message.UserMessage("以下是此前会话的摘要：\n"+summary))
	out = append(out, tail...)
	return out
}
