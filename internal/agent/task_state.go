package agent

// Status 是一次运行的状态。
type Status string

const (
	// StatusRunning 表示任务仍在推进。
	StatusRunning Status = "running"
	// StatusCompleted 表示模型给出了最终答案。
	StatusCompleted Status = "completed"
	// StatusStopped 表示因步数或重试上限主动停下。
	StatusStopped Status = "stopped"
	// StatusFailed 表示模型请求失败。
	StatusFailed Status = "failed"
)

// StopReason 说明这次运行为什么结束。
type StopReason string

const (
	// StopFinalAnswer 表示正常收尾。
	StopFinalAnswer StopReason = "final_answer_returned"
	// StopStepLimit 表示达到工具步数上限。
	StopStepLimit StopReason = "step_limit_reached"
	// StopRetryLimit 表示连续格式错误达到上限。
	StopRetryLimit StopReason = "retry_limit_reached"
	// StopModelError 表示模型请求失败。
	StopModelError StopReason = "model_error"
)

// TaskState 是一次运行的快照，用于观测与复盘。
type TaskState struct {
	RunID       string
	UserRequest string
	Status      Status
	ToolSteps   int
	Attempts    int
	LastTool    string
	StopReason  StopReason
	FinalAnswer string
}

// Done 报告状态是否为终止态。
func (s TaskState) Done() bool {
	return s.Status != StatusRunning
}
