package agent

import (
	"context"
	"strings"
	"time"

	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/protocol"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

const (
	// DefaultMaxSteps 是一次任务允许的工具步数上限。
	DefaultMaxSteps = 24
	// DefaultMaxRetries 是连续格式错误的重试上限。
	DefaultMaxRetries = 3
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
	MaxRetries int
	Options    ctxpkg.Options
	Recorder   Recorder
	OnTool     func(name string, output string, ok bool)
}

// RunResult 是一次运行的结果。
type RunResult struct {
	State   TaskState
	History []ctxpkg.HistoryEntry
}

// Run 按文本协议推进一次任务：拼 prompt、请求模型、解析、执行工具、回灌历史。
func Run(ctx context.Context, args RunArgs) RunResult {
	maxSteps := args.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	maxRetries := args.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	state := TaskState{
		Status:      StatusRunning,
		UserRequest: args.UserInput,
	}
	history := append([]ctxpkg.HistoryEntry(nil), args.History...)
	turn := make([]ctxpkg.HistoryEntry, 0, 8)
	retries := 0

	for step := 0; step < maxSteps; step++ {
		sections := ctxpkg.Sections{
			Prefix:         args.Prefix,
			History:        ctxpkg.RenderHistory(append(append([]ctxpkg.HistoryEntry(nil), history...), turn...), 0),
			CurrentRequest: args.UserInput,
		}
		built := ctxpkg.Build(sections, args.Options)
		record(args.Recorder, "model.request", map[string]any{
			"step":          step,
			"prompt_chars":  len(built.Prompt),
			"history_chars": built.Sections[ctxpkg.SectionHistory].Rendered,
			"reductions":    len(built.Reductions),
		})

		started := time.Now()
		raw, err := args.Model.Complete(ctx, built.Prompt)
		state.Attempts++
		record(args.Recorder, "model.response", map[string]any{
			"duration_ms": time.Since(started).Milliseconds(),
			"chars":       len(raw),
			"ok":          err == nil,
		})
		if err != nil {
			state.Status = StatusFailed
			state.StopReason = StopModelError
			return RunResult{State: state, History: finishHistory(history, args.UserInput, "")}
		}

		action := protocol.Parse(raw)
		record(args.Recorder, "model.parsed", map[string]any{
			"kind": string(action.Kind),
			"tool": action.Tool,
		})

		switch action.Kind {
		case protocol.KindFinal:
			state.Status = StatusCompleted
			state.StopReason = StopFinalAnswer
			state.FinalAnswer = action.Final
			return RunResult{State: state, History: finishHistory(history, args.UserInput, action.Final)}

		case protocol.KindRetry:
			turn = append(turn, ctxpkg.HistoryEntry{Role: ctxpkg.RoleUser, Text: protocol.NoticeText(action)})
			retries++
			if retries >= maxRetries {
				state.Status = StatusStopped
				state.StopReason = StopRetryLimit
				return RunResult{State: state, History: finishHistory(history, args.UserInput, "")}
			}

		case protocol.KindTool:
			result := executeTool(ctx, args, action)
			state.ToolSteps++
			state.LastTool = action.Tool
			entry := ctxpkg.HistoryEntry{
				Role:     ctxpkg.RoleTool,
				ToolName: action.Tool,
				Path:     pathOf(action.Args),
				Text:     result.Output,
			}
			turn = append(turn, ctxpkg.HistoryEntry{Role: ctxpkg.RoleAssistant, Text: "调用 " + action.Tool}, entry)
			if args.OnTool != nil {
				args.OnTool(action.Tool, result.Output, result.OK)
			}
		}
	}

	state.Status = StatusStopped
	state.StopReason = StopStepLimit
	return RunResult{State: state, History: finishHistory(history, args.UserInput, "")}
}

func executeTool(ctx context.Context, args RunArgs, action protocol.Action) tools.Result {
	record(args.Recorder, "tool.call", map[string]any{
		"name": action.Tool,
	})
	started := time.Now()
	result := args.Tools.Execute(ctx, action.Tool, action.Args, tools.Context{
		CWD:        args.CWD,
		Permission: args.Permission,
	})
	record(args.Recorder, "tool.result", map[string]any{
		"name":         action.Tool,
		"ok":           result.OK,
		"duration_ms":  time.Since(started).Milliseconds(),
		"output_chars": len(result.Output),
	})
	return result
}

// finishHistory 把本轮的用户请求与最终回答并入历史；没有最终回答时只记请求。
func finishHistory(history []ctxpkg.HistoryEntry, userInput, final string) []ctxpkg.HistoryEntry {
	out := append([]ctxpkg.HistoryEntry(nil), history...)
	out = append(out, ctxpkg.HistoryEntry{Role: ctxpkg.RoleUser, Text: userInput})
	if strings.TrimSpace(final) != "" {
		out = append(out, ctxpkg.HistoryEntry{Role: ctxpkg.RoleAssistant, Text: final})
	}
	return out
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
