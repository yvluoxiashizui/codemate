package message

import "context"

// Completer 是文本协议下的模型接口：输入一段 prompt，输出一段文本。
type Completer interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

type Role string

const (
	RoleSystem            Role = "system"
	RoleUser              Role = "user"
	RoleAssistant         Role = "assistant"
	RoleAssistantProgress Role = "assistant_progress"
	RoleAssistantToolCall Role = "assistant_tool_call"
	RoleToolResult        Role = "tool_result"
)

type Message struct {
	Role      Role
	Content   string
	ToolUseID string
	ToolName  string
	Input     any
	IsError   bool
}

func SystemMessage(content string) Message {
	return Message{Role: RoleSystem, Content: content}
}

func UserMessage(content string) Message {
	return Message{Role: RoleUser, Content: content}
}

func AssistantMessage(content string) Message {
	return Message{Role: RoleAssistant, Content: content}
}

func ProgressMessage(content string) Message {
	return Message{Role: RoleAssistantProgress, Content: content}
}

func AssistantToolCallMessage(call ToolCall) Message {
	return Message{
		Role:      RoleAssistantToolCall,
		ToolUseID: call.ID,
		ToolName:  call.ToolName,
		Input:     call.Input,
	}
}

func ToolResultMessage(toolUseID, toolName, content string, isError bool) Message {
	return Message{
		Role:      RoleToolResult,
		ToolUseID: toolUseID,
		ToolName:  toolName,
		Content:   content,
		IsError:   isError,
	}
}

type ToolCall struct {
	ID       string
	ToolName string
	Input    any
}

type Diagnostics struct {
	StopReason        string
	BlockTypes        []string
	IgnoredBlockTypes []string
}

type StepType string

const (
	StepAssistant StepType = "assistant"
	StepToolCalls StepType = "tool_calls"
)

type ContentKind string

const (
	ContentNone     ContentKind = ""
	ContentFinal    ContentKind = "final"
	ContentProgress ContentKind = "progress"
)

// Usage 记录一次模型调用的 token 用量。
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// IsZero 报告用量是否为空。
func (u Usage) IsZero() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 && u.TotalTokens == 0
}
