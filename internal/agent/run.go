package agent

import (
	"context"
	"time"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/protocol"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

const (
	// DefaultMaxSteps 是一次任务允许的工具步数上限。
	DefaultMaxSteps = 24
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
	Options    ctxpkg.Options
	Recorder   Recorder
	OnTool     func(name string, output string, ok bool)
}

// RunResult 是一次运行的结果。
type RunResult struct {
	State   TaskState
	History []ctxpkg.HistoryEntry
}

// Run 推进一次任务，循环结构分四段：感知、决策、行动、记录。
func Run(ctx context.Context, args RunArgs) RunResult {
	maxSteps := args.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	maxAttempts := maxSteps * 3
	if maxSteps+4 > maxAttempts {
		maxAttempts = maxSteps + 4
	}

	state := TaskState{
		Status:      StatusRunning,
		UserRequest: args.UserInput,
	}
	history := append(append([]ctxpkg.HistoryEntry(nil), args.History...),
		ctxpkg.HistoryEntry{Role: ctxpkg.RoleUser, Text: args.UserInput})

	record(args.Recorder, "run_started", map[string]any{
		"user_request": args.UserInput,
	})

	toolSteps := 0
	attempts := 0

	for toolSteps < maxSteps && attempts < maxAttempts {
		attempts++
		state.Attempts = attempts

		prompt, meta := buildPrompt(args, history)
		record(args.Recorder, "prompt_built", map[string]any{
			"prompt_chars":  len(prompt),
			"history_chars": meta.HistoryChars,
			"reductions":    meta.Reductions,
			"attempt":       attempts,
		})

		raw, err := requestModel(ctx, args.Recorder, args.Model, prompt)
		if err != nil {
			state.Status = StatusFailed
			state.StopReason = StopModelError
			return RunResult{State: state, History: history}
		}

		action := protocol.Parse(raw)
		record(args.Recorder, "model_parsed", map[string]any{
			"kind": string(action.Kind),
			"tool": action.Tool,
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
			})
			history = append(history, ctxpkg.HistoryEntry{
				Role:     ctxpkg.RoleTool,
				ToolName: action.Tool,
				Path:     pathOf(action.Args),
				Text:     result.Output,
			})
			record(args.Recorder, "tool_executed", map[string]any{
				"name":         action.Tool,
				"ok":           result.OK,
				"duration_ms":  time.Since(started).Milliseconds(),
				"output_chars": len(result.Output),
			})
			if args.OnTool != nil {
				args.OnTool(action.Tool, result.Output, result.OK)
			}

		case protocol.KindRetry:
			history = append(history, ctxpkg.HistoryEntry{
				Role: ctxpkg.RoleAssistant,
				Text: protocol.NoticeText(action),
			})

		case protocol.KindFinal:
			return finish(state, history, action.Final, args, StatusCompleted, StopFinalAnswer)
		}
	}

	if toolSteps >= maxSteps {
		state.Attempts = attempts + 1
		prompt, meta := buildPrompt(args, history)
		prompt += finalizationNotice
		record(args.Recorder, "prompt_built", map[string]any{
			"prompt_chars":  len(prompt),
			"history_chars": meta.HistoryChars,
			"reductions":    meta.Reductions,
			"purpose":       "finalization",
		})
		raw, err := requestModel(ctx, args.Recorder, args.Model, prompt)
		if err == nil {
			if action := protocol.Parse(raw); action.Kind == protocol.KindFinal {
				return finish(state, history, action.Final, args, StatusCompleted, StopFinalAnswer)
			}
		} else {
			state.Status = StatusFailed
			state.StopReason = StopModelError
			return RunResult{State: state, History: history}
		}
	}

	if attempts >= maxAttempts && toolSteps < maxSteps {
		return finish(state, history, stopByRetry, args, StatusStopped, StopRetryLimit)
	}
	return finish(state, history, stopBySteps, args, StatusStopped, StopStepLimit)
}

// promptMeta 是一次拼装的附带信息。
type promptMeta struct {
	HistoryChars int
	Reductions   int
}

func buildPrompt(args RunArgs, history []ctxpkg.HistoryEntry) (string, promptMeta) {
	built := ctxpkg.Build(ctxpkg.Sections{
		Prefix:         args.Prefix,
		History:        ctxpkg.RenderHistory(history, 0),
		CurrentRequest: args.UserInput,
	}, args.Options)
	return built.Prompt, promptMeta{
		HistoryChars: built.Sections[ctxpkg.SectionHistory].Rendered,
		Reductions:   len(built.Reductions),
	}
}

func requestModel(ctx context.Context, recorder Recorder, model message.Completer, prompt string) (string, error) {
	record(recorder, "model_requested", map[string]any{
		"prompt_chars": len(prompt),
	})
	started := time.Now()
	raw, err := model.Complete(ctx, prompt)
	record(recorder, "model_parsed", map[string]any{
		"duration_ms": time.Since(started).Milliseconds(),
		"chars":       len(raw),
		"ok":          err == nil,
	})
	return raw, err
}

func finish(state TaskState, history []ctxpkg.HistoryEntry, final string, args RunArgs, status Status, reason StopReason) RunResult {
	state.Status = status
	state.StopReason = reason
	state.FinalAnswer = final
	history = append(history, ctxpkg.HistoryEntry{Role: ctxpkg.RoleAssistant, Text: final})
	record(args.Recorder, "run_finished", map[string]any{
		"status":      string(state.Status),
		"stop_reason": string(state.StopReason),
		"tool_steps":  state.ToolSteps,
		"attempts":    state.Attempts,
	})
	return RunResult{State: state, History: history}
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
