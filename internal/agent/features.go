package agent

// Features 是运行时的能力开关，默认全开。
type Features struct {
	Memory           bool
	RelevantMemory   bool
	ContextReduction bool
	PromptCache      bool
}

// DefaultFeatures 返回全开的开关。
func DefaultFeatures() Features {
	return Features{
		Memory:           true,
		RelevantMemory:   true,
		ContextReduction: true,
		PromptCache:      true,
	}
}

// Apply 按配置覆盖开关，未出现的名字保持原值。
func (f Features) Apply(overrides map[string]bool) Features {
	for name, value := range overrides {
		switch name {
		case "memory":
			f.Memory = value
		case "relevant_memory":
			f.RelevantMemory = value
		case "context_reduction":
			f.ContextReduction = value
		case "prompt_cache":
			f.PromptCache = value
		}
	}
	return f
}

// Enabled 报告某个开关是否开启。
func (f Features) Enabled(name string) bool {
	switch name {
	case "memory":
		return f.Memory
	case "relevant_memory":
		return f.RelevantMemory
	case "context_reduction":
		return f.ContextReduction
	case "prompt_cache":
		return f.PromptCache
	default:
		return false
	}
}
