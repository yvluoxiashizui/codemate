package token

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/yvluoxiashizui/codemate/internal/message"
)

const (
	// DefaultWindow 是默认上下文窗口，单位为 token。
	DefaultWindow = 131072

	// asciiPerToken 是 ASCII 文本的估算比例，取四字符一个 token。
	asciiPerToken = 4

	// messageOverhead 是每条消息的角色与分隔符开销。
	messageOverhead = 4
)

// EstimateText 估算一段文本的 token 数。
// ASCII 按四字符一个 token，非 ASCII 按每个字符一个 token；两者都向上取整，
// 结果偏高，用于预算时留出余量。
func EstimateText(text string) int {
	if text == "" {
		return 0
	}
	ascii := 0
	other := 0
	for _, r := range text {
		if r < utf8.RuneSelf {
			ascii++
			continue
		}
		other++
	}
	tokens := ascii / asciiPerToken
	if ascii%asciiPerToken != 0 {
		tokens++
	}
	return tokens + other
}

// EstimateJSON 估算任意值序列化后的 token 数。
func EstimateJSON(value any) int {
	payload, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return EstimateText(string(payload))
}

// EstimateMessages 估算一组消息占用的 token 数。
func EstimateMessages(messages []message.Message) int {
	total := 0
	for _, item := range messages {
		total += messageOverhead
		total += EstimateText(item.Content)
		total += EstimateText(item.ToolName)
		if item.Input != nil {
			total += EstimateJSON(item.Input)
		}
	}
	return total
}

// Budget 描述一次请求的上下文占用。
type Budget struct {
	Messages  int `json:"messages_tokens"`
	Tools     int `json:"tools_tokens"`
	Total     int `json:"total_tokens"`
	Window    int `json:"window_tokens"`
	Remaining int `json:"remaining_tokens"`
}

// NewBudget 按消息与工具声明的估算值计算上下文占用。
func NewBudget(messages, tools, window int) Budget {
	if window <= 0 {
		window = DefaultWindow
	}
	remaining := window - messages - tools
	if remaining < 0 {
		remaining = 0
	}
	return Budget{
		Messages:  messages,
		Tools:     tools,
		Total:     messages + tools,
		Window:    window,
		Remaining: remaining,
	}
}

// OverBudget 报告上下文是否已超出窗口。
func (b Budget) OverBudget() bool {
	return b.Remaining == 0 && b.Total > b.Window
}
