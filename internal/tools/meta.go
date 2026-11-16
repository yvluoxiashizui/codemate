package tools

// RiskLevel 描述一次工具调用的风险等级。
type RiskLevel string

const (
	// RiskLow 是不改变工作区的操作。
	RiskLow RiskLevel = "low"
	// RiskHigh 是可能改变工作区的操作。
	RiskHigh RiskLevel = "high"
)

// 工具状态。
const (
	StatusOK     = "ok"
	StatusError  = "error"
	StatusPanic  = "panic"
	StatusReject = "rejected"
)

// 错误码，用于把失败归类。
const (
	CodeUnknownTool    = "unknown_tool"
	CodeInvalidInput   = "invalid_input"
	CodeToolError      = "tool_error"
	CodeToolPanic      = "tool_panic"
	CodeReadOnly       = "read_only_blocked"
	CodeToolNotAllowed = "tool_not_allowed"
)

// Meta 是一次工具执行的元数据。
type Meta struct {
	Status        string    `json:"tool_status"`
	ErrorCode     string    `json:"tool_error_code,omitempty"`
	RiskLevel     RiskLevel `json:"risk_level"`
	ReadOnly      bool      `json:"read_only"`
	AffectedPaths []string  `json:"affected_paths,omitempty"`
	// SecurityEvent 标记这次调用触发了哪类安全拦截。
	SecurityEvent    string `json:"security_event_type,omitempty"`
	WorkspaceChanged bool   `json:"workspace_changed"`
}

// 安全事件类型，用于在事件流里区分拦截原因。
const (
	SecurityReadOnlyBlocked = "read_only_blocked"
	SecurityToolNotAllowed  = "tool_not_allowed"
)
