package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/memory"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/protocol"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

const (
	// DefaultMaxSteps 是一次任务允许的工具步数上限。
	DefaultMaxSteps = 24

	// DefaultMaxDepth 是子 agent 允许的最大嵌套深度。
	DefaultMaxDepth = 2
)

const (
	// finalizationNotice 在工具预算用尽后追加，要求模型只给最终答案。
	finalizationNotice = "\n\nRuntime notice: the tool budget is exhausted. Do not call another tool. " +
		"Use the evidence already present in the tool history and return exactly one non-empty <final>...</final> answer."

	// stopByRetry 是格式错误次数用尽时的收尾文案。
	stopByRetry = "模型连续返回无法解析的格式，已停止本次任务。"

	// stopBySteps 是工具步数用尽且没有拿到最终答案时的收尾文案。
	stopBySteps = "达到工具步数上限，且没有拿到最终答案，已停止本次任务。"
)

// StateSink 把任务状态快照写进运行目录，允许为 nil。
type StateSink interface {
	SaveState(snapshot map[string]any) error
}

// ReportSink 接收一次运行的汇总报告，允许为 nil。
type ReportSink interface {
	SaveReport(report map[string]any) error
}

// Recorder 接收运行过程中产生的事件，允许为 nil。
type Recorder interface {
	Event(typ string, data map[string]any)
}

func record(recorder Recorder, typ string, data map[string]any) {
	if recorder == nil {
		return
	}
	recorder.Event(typ, data)
}

// RunArgs 是文本协议下的一次运行入参。
type RunArgs struct {
	Model      message.Completer
	Tools      *tools.Registry
	Prefix     string
	History    []ctxpkg.HistoryEntry
	UserInput  string
	CWD        string
	Permission tools.PermissionManager
	MaxSteps   int
	Depth      int
	MaxDepth   int
	ReadOnly   bool
	Allowed    []string
	Options    ctxpkg.Options
	Recorder   Recorder
	Features   Features
	Memory     *memory.Store
	Durable    *memory.DurableStore
	Freshness  func(path string) string
	State      StateSink
	Report     ReportSink
	OnTool     func(name string, output string, ok bool)
}

// RunResult 是一次运行的结果。
// TouchedPaths 是本次运行中工具访问过的文件路径，按出现顺序去重。
type RunResult struct {
	State        TaskState
	History      []ctxpkg.HistoryEntry
	TouchedPaths []string
}

// Run 推进一次任务，循环结构分四段：感知、决策、行动、记录。
func Run(ctx context.Context, args RunArgs) RunResult {
	maxSteps := args.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	features := args.Features
	if !features.Memory && !features.RelevantMemory && !features.ContextReduction && !features.PromptCache {
		features = DefaultFeatures()
	}
	cacheKey := ""
	if features.PromptCache {
		cacheKey = prefixCacheKey(args.Prefix)
	}
	maxDepth := args.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultMaxDepth
	}
	maxAttempts := maxSteps * 3
	if maxSteps+4 > maxAttempts {
		maxAttempts = maxSteps + 4
	}

	state := TaskState{
		Status:      StatusRunning,
		UserRequest: args.UserInput,
	}
	if args.Memory != nil && features.Memory {
		args.Memory.SetTaskSummary(args.UserInput)
	}
	if args.Durable != nil && features.Memory && memory.HasMemoryIntent(args.UserInput) {
		args.Durable.Promote(memory.DetectPromotions(args.UserInput))
		_ = args.Durable.Save()
	}

	history := append(append([]ctxpkg.HistoryEntry(nil), args.History...),
		ctxpkg.HistoryEntry{Role: ctxpkg.RoleUser, Text: args.UserInput})

	record(args.Recorder, "run_started", map[string]any{
		"user_request": args.UserInput,
	})

	spawn := func(ctx context.Context, task string, childSteps int) (string, error) {
		if args.Depth >= maxDepth {
			return "", errors.New("delegate depth exceeded")
		}
		child := args
		child.Depth = args.Depth + 1
		child.MaxSteps = childSteps
		child.ReadOnly = true
		child.Allowed = nil
		child.UserInput = task
		child.History = []ctxpkg.HistoryEntry{{
			Role: ctxpkg.RoleAssistant,
			Text: clipText(ctxpkg.RenderHistory(history, 0), 300),
		}}
		result := Run(ctx, child)
		return "delegate_result:\n" + result.State.FinalAnswer, nil
	}

	toolSteps := 0
	attempts := 0
	touched := make([]string, 0, 8)
	toolCalls := make([]map[string]any, 0, 8)
	runStarted := time.Now()
	seen := map[string]bool{}
	flush := func() {
		if args.State == nil {
			return
		}
		_ = args.State.SaveState(map[string]any{
			"user_request": state.UserRequest,
			"status":       string(state.Status),
			"attempts":     state.Attempts,
			"tool_steps":   state.ToolSteps,
			"last_tool":    state.LastTool,
			"stop_reason":  string(state.StopReason),
			"final_answer": state.FinalAnswer,
		})
	}
	flush()

	emitReport := func() {
		if args.Report == nil {
			return
		}
		failures := 0
		for _, call := range toolCalls {
			if ok, _ := call["ok"].(bool); !ok {
				failures++
			}
		}
		_ = args.Report.SaveReport(map[string]any{
			"status":        string(state.Status),
			"stop_reason":   string(state.StopReason),
			"attempts":      state.Attempts,
			"tool_steps":    state.ToolSteps,
			"tool_failures": failures,
			"tool_calls":    toolCalls,
			"touched_paths": touched,
			"duration_ms":   time.Since(runStarted).Milliseconds(),
			"final_answer":  state.FinalAnswer,
		})
	}

	for toolSteps < maxSteps && attempts < maxAttempts {
		attempts++
		state.Attempts = attempts
		flush()

		prompt, meta := buildPrompt(args, history, features)
		record(args.Recorder, "prompt_built", map[string]any{
			"prompt_chars":  len(prompt),
			"history_chars": meta.HistoryChars,
			"reductions":    meta.Reductions,
			"attempt":       attempts,
		})

		raw, elapsed, cachedTokens, err := requestModel(ctx, args.Recorder, args.Model, prompt, cacheKey)
		if err != nil {
			record(args.Recorder, "model_parsed", map[string]any{
				"duration_ms": elapsed.Milliseconds(),
				"ok":          false,
			})
			state.Status = StatusFailed
			state.StopReason = StopModelError
			emitReport()
			return RunResult{State: state, History: history, TouchedPaths: touched}
		}

		action := protocol.Parse(raw)
		record(args.Recorder, "model_parsed", map[string]any{
			"kind":          string(action.Kind),
			"tool":          action.Tool,
			"chars":         len(raw),
			"duration_ms":   elapsed.Milliseconds(),
			"cached_tokens": cachedTokens,
			"cache_hit":     cachedTokens > 0,
			"ok":            true,
		})

		switch action.Kind {
		case protocol.KindTool:
			toolSteps++
			state.ToolSteps = toolSteps
			state.LastTool = action.Tool

			started := time.Now()
			result := args.Tools.Execute(ctx, action.Tool, action.Args, tools.Context{
				CWD:        args.CWD,
				Permission: args.Permission,
				Policy:     tools.Policy{ReadOnly: args.ReadOnly, Allowed: args.Allowed},
				Depth:      args.Depth,
				MaxDepth:   maxDepth,
				Spawn:      spawn,
			})
			history = append(history, ctxpkg.HistoryEntry{
				Role:     ctxpkg.RoleTool,
				ToolName: action.Tool,
				Path:     pathOf(action.Args),
				Text:     result.Output,
			})
			toolCalls = append(toolCalls, map[string]any{
				"name":           action.Tool,
				"ok":             result.OK,
				"tool_status":    result.Meta.Status,
				"error_code":     result.Meta.ErrorCode,
				"risk_level":     string(result.Meta.RiskLevel),
				"affected_paths": result.Meta.AffectedPaths,
			})
			record(args.Recorder, "tool_executed", map[string]any{
				"name":              action.Tool,
				"ok":                result.OK,
				"duration_ms":       time.Since(started).Milliseconds(),
				"output_chars":      len(result.Output),
				"tool_status":       result.Meta.Status,
				"tool_error_code":   result.Meta.ErrorCode,
				"security_event":    result.Meta.SecurityEvent,
				"risk_level":        string(result.Meta.RiskLevel),
				"read_only":         result.Meta.ReadOnly,
				"workspace_changed": result.Meta.WorkspaceChanged,
				"affected_paths":    result.Meta.AffectedPaths,
			})
			flush()
			if path := pathOf(action.Args); path != "" {
				if !seen[path] {
					seen[path] = true
					touched = append(touched, path)
				}
				if args.Memory != nil && features.Memory {
					freshness := ""
					if args.Freshness != nil {
						freshness = args.Freshness(path)
					}
					args.Memory.RememberFile(path, summarizeToolOutput(action.Tool, result.Output), freshness)
				}
			}
			if args.OnTool != nil {
				args.OnTool(action.Tool, result.Output, result.OK)
			}

		case protocol.KindRetry:
			history = append(history, ctxpkg.HistoryEntry{
				Role: ctxpkg.RoleAssistant,
				Text: protocol.NoticeText(action),
			})

		case protocol.KindFinal:
			result := finish(&state, history, action.Final, args, StatusCompleted, StopFinalAnswer)
			result.TouchedPaths = touched
			emitReport()
			return result
		}
	}

	if toolSteps >= maxSteps {
		state.Attempts = attempts + 1
		prompt, meta := buildPrompt(args, history, features)
		prompt += finalizationNotice
		record(args.Recorder, "prompt_built", map[string]any{
			"prompt_chars":  len(prompt),
			"history_chars": meta.HistoryChars,
			"reductions":    meta.Reductions,
			"purpose":       "finalization",
		})
		raw, _, _, err := requestModel(ctx, args.Recorder, args.Model, prompt, cacheKey)
		if err == nil {
			if action := protocol.Parse(raw); action.Kind == protocol.KindFinal {
				result := finish(&state, history, action.Final, args, StatusCompleted, StopFinalAnswer)
				result.TouchedPaths = touched
				emitReport()
				return result
			}
		} else {
			state.Status = StatusFailed
			state.StopReason = StopModelError
			emitReport()
			return RunResult{State: state, History: history, TouchedPaths: touched}
		}
	}

	if attempts >= maxAttempts && toolSteps < maxSteps {
		result := finish(&state, history, stopByRetry, args, StatusStopped, StopRetryLimit)
		result.TouchedPaths = touched
		emitReport()
		return result
	}
	result := finish(&state, history, stopBySteps, args, StatusStopped, StopStepLimit)
	result.TouchedPaths = touched
	emitReport()
	return result
}

// promptMeta 是一次拼装的附带信息。
type promptMeta struct {
	HistoryChars int
	Reductions   int
}

func buildPrompt(args RunArgs, history []ctxpkg.HistoryEntry, features Features) (string, promptMeta) {
	memoryText := ""
	relevantText := ""
	if args.Memory != nil && features.Memory {
		memoryText = args.Memory.Render()
		if features.RelevantMemory {
			var durable []memory.DurableNote
			if args.Durable != nil {
				durable = args.Durable.Notes()
			}
			relevantText = memory.RenderView(args.Memory.Retrieval(args.UserInput, 3), durable)
		}
	}
	options := args.Options
	if !features.ContextReduction {
		options.DisableReduction = true
	}
	built := ctxpkg.Build(ctxpkg.Sections{
		Prefix:         args.Prefix,
		Memory:         memoryText,
		Relevant:       relevantText,
		History:        ctxpkg.RenderHistory(history, 0),
		CurrentRequest: args.UserInput,
	}, options)
	return built.Prompt, promptMeta{
		HistoryChars: built.Sections[ctxpkg.SectionHistory].Rendered,
		Reductions:   len(built.Reductions),
	}
}

// prefixCacheKey 用提示词前缀的哈希作为缓存键，前缀不变时键也不变。
func prefixCacheKey(prefix string) string {
	if prefix == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(prefix))
	return hex.EncodeToString(sum[:8])
}

func requestModel(ctx context.Context, recorder Recorder, model message.Completer, prompt, cacheKey string) (string, time.Duration, int, error) {
	record(recorder, "model_requested", map[string]any{
		"prompt_chars": len(prompt),
		"cache_key":    cacheKey,
	})
	started := time.Now()
	if cacheKey != "" {
		if caching, ok := model.(message.CachingCompleter); ok {
			raw, cached, err := caching.CompleteWithCache(ctx, prompt, cacheKey, "")
			return raw, time.Since(started), cached, err
		}
	}
	raw, err := model.Complete(ctx, prompt)
	return raw, time.Since(started), 0, err
}

func finish(state *TaskState, history []ctxpkg.HistoryEntry, final string, args RunArgs, status Status, reason StopReason) RunResult {
	state.Status = status
	state.StopReason = reason
	state.FinalAnswer = final
	if args.State != nil {
		_ = args.State.SaveState(map[string]any{
			"user_request": state.UserRequest,
			"status":       string(state.Status),
			"attempts":     state.Attempts,
			"tool_steps":   state.ToolSteps,
			"last_tool":    state.LastTool,
			"stop_reason":  string(state.StopReason),
			"final_answer": state.FinalAnswer,
		})
	}
	history = append(history, ctxpkg.HistoryEntry{Role: ctxpkg.RoleAssistant, Text: final})
	record(args.Recorder, "run_finished", map[string]any{
		"status":      string(state.Status),
		"stop_reason": string(state.StopReason),
		"tool_steps":  state.ToolSteps,
		"attempts":    state.Attempts,
	})
	return RunResult{State: *state, History: history}
}

func pathOf(args map[string]any) string {
	if args == nil {
		return ""
	}
	value, ok := args["path"].(string)
	if !ok {
		return ""
	}
	return value
}

// summarizeToolOutput 按工具类型压缩输出：读文件用专门的摘要逻辑，其余取首行。
func summarizeToolOutput(tool string, output string) string {
	if tool == "read_file" {
		return memory.SummarizeReadResult(output, 0)
	}
	return summarizeOutput(output)
}

// summarizeOutput 把工具输出压成一句话摘要，供记忆层保存。
func summarizeOutput(output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return ""
	}
	if idx := strings.IndexByte(text, 10); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	const limit = 120
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "…"
}

// clipText 截断文本，用于给子 agent 的上下文摘要。
func clipText(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + "..."
}
