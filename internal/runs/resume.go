package runs

// ResumeStatus 描述一份检查点能否用于恢复。
type ResumeStatus string

const (
	// ResumeNoCheckpoint 表示没有可用的检查点。
	ResumeNoCheckpoint ResumeStatus = "no-checkpoint"
	// ResumeFullValid 表示检查点与当前环境完全一致。
	ResumeFullValid ResumeStatus = "full-valid"
	// ResumePartialStale 表示关键文件在检查点之后被改过。
	ResumePartialStale ResumeStatus = "partial-stale"
	// ResumeWorkspaceMismatch 表示工作区身份与检查点不一致。
	ResumeWorkspaceMismatch ResumeStatus = "workspace-mismatch"
	// ResumeSchemaMismatch 表示检查点格式版本不认识。
	ResumeSchemaMismatch ResumeStatus = "schema-mismatch"
)

// ResumeResult 是一次恢复判定的结果。
type ResumeResult struct {
	Status         ResumeStatus
	StaleFiles     []string
	MismatchFields []string
}

// EvaluateResume 比对检查点与当前环境。
// identity 是当前运行时身份，freshness 用于取文件当前指纹。
func EvaluateResume(checkpoint Checkpoint, identity map[string]any, freshness func(path string) string) ResumeResult {
	if checkpoint.SchemaVersion == "" && len(checkpoint.RuntimeIdentity) == 0 {
		return ResumeResult{Status: ResumeNoCheckpoint}
	}
	if checkpoint.SchemaVersion != CheckpointSchemaVersion {
		return ResumeResult{Status: ResumeSchemaMismatch}
	}

	mismatch := make([]string, 0, 4)
	for _, key := range []string{"cwd", "workspace_fingerprint", "model", "max_steps"} {
		expected, ok := checkpoint.RuntimeIdentity[key]
		if !ok {
			continue
		}
		if !sameValue(expected, identity[key]) {
			mismatch = append(mismatch, key)
		}
	}
	if len(mismatch) > 0 {
		return ResumeResult{Status: ResumeWorkspaceMismatch, MismatchFields: mismatch}
	}

	stale := make([]string, 0, len(checkpoint.KeyFiles))
	for _, file := range checkpoint.KeyFiles {
		if file.Path == "" || freshness == nil {
			continue
		}
		if freshness(file.Path) != file.Freshness {
			stale = append(stale, file.Path)
		}
	}
	if len(stale) > 0 {
		return ResumeResult{Status: ResumePartialStale, StaleFiles: stale}
	}
	return ResumeResult{Status: ResumeFullValid}
}

func sameValue(expected, actual any) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}
	return formatValue(expected) == formatValue(actual)
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case int:
		return itoa(int64(typed))
	case int64:
		return itoa(typed)
	case float64:
		return itoa(int64(typed))
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func itoa(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	digits := make([]byte, 0, 20)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// ResumeHint 把恢复判定结果渲染成写进提示词前缀的一句话。
// 完全有效或没有检查点时返回空串。
func ResumeHint(result ResumeResult) string {
	switch result.Status {
	case ResumePartialStale:
		return "恢复提示：以下文件在上次检查点之后被修改过，此前的结论可能已过时：" + joinList(result.StaleFiles) + "。"
	case ResumeWorkspaceMismatch:
		return "恢复提示：当前工作区与上次检查点不一致（" + joinList(result.MismatchFields) + " 变化），此前的结论可能已过时。"
	case ResumeSchemaMismatch:
		return "恢复提示：上次检查点格式版本不受支持，已忽略。"
	default:
		return ""
	}
}

func joinList(items []string) string {
	if len(items) == 0 {
		return "无"
	}
	out := ""
	for i, item := range items {
		if i > 0 {
			out += "、"
		}
		out += item
	}
	return out
}
