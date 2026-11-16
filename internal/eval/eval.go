package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yvluoxiashizui/codemate/internal/agent"
	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/protocol"
	"github.com/yvluoxiashizui/codemate/internal/tools"
	"github.com/yvluoxiashizui/codemate/internal/workspace"
)

// Case 是一个评测任务：给定初始仓库与请求，用判定函数检查结果。
type Case struct {
	Name   string
	Prompt string
	Files  map[string]string
	Check  func(root string) error
}

// Result 是一条任务的执行结果。
type Result struct {
	Case         string
	Passed       bool
	Reason       string
	Status       string
	StopReason   string
	Attempts     int
	ToolSteps    int
	PromptChars  int
	ToolFailures int
	DurationMS   int64
}

// Summary 是一批任务的聚合指标。
type Summary struct {
	Total           int
	Passed          int
	PassRate        float64
	AvgAttempts     float64
	AvgToolSteps    float64
	AvgPromptChars  float64
	ToolFailureRate float64
}

// Runner 用给定的模型与工具跑评测任务。
type Runner struct {
	Model    message.Completer
	MaxSteps int
	// Features 是本次评测使用的开关，零值表示全开。
	Features agent.Features
	// Options 是上下文预算，零值表示用默认预算。
	Options  ctxpkg.Options
	OnResult func(Result)
}

// RunCase 在临时工作区里跑一条任务并判定结果。
func (r Runner) RunCase(ctx context.Context, item Case) Result {
	started := time.Now()
	result := Result{Case: item.Name}

	root, err := os.MkdirTemp("", "codemate-eval-*")
	if err != nil {
		result.Reason = "创建临时工作区失败: " + err.Error()
		return result
	}
	defer os.RemoveAll(root)

	for name, content := range item.Files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			result.Reason = "准备文件失败: " + err.Error()
			return result
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			result.Reason = "准备文件失败: " + err.Error()
			return result
		}
	}

	registry := tools.Builtins(root, nil, nil)
	wsContext := workspace.BuildContext(root)
	recorder := &metricsRecorder{}
	run := agent.Run(ctx, agent.RunArgs{
		Model: r.Model,
		Tools: registry,
		Prefix: protocol.BuildPrefix(protocol.Workspace{
			Root:          wsContext.Root,
			Branch:        wsContext.Branch,
			Dirty:         wsContext.Dirty,
			RecentCommits: wsContext.RecentCommits,
			ProjectDocs:   wsContext.ProjectDocs,
		}, protocol.ToolSpecs(registry)),
		UserInput: item.Prompt,
		CWD:       root,
		MaxSteps:  r.MaxSteps,
		Features:  r.Features,
		Options:   r.Options,
		Recorder:  recorder,
	})

	result.Status = string(run.State.Status)
	result.StopReason = string(run.State.StopReason)
	result.Attempts = run.State.Attempts
	result.ToolSteps = run.State.ToolSteps
	result.PromptChars = recorder.promptChars
	result.ToolFailures = recorder.toolFailures
	result.DurationMS = time.Since(started).Milliseconds()

	if err := item.Check(root); err != nil {
		result.Reason = err.Error()
	} else {
		result.Passed = true
	}
	if r.OnResult != nil {
		r.OnResult(result)
	}
	return result
}

// RunCases 顺序跑完所有任务。
func (r Runner) RunCases(ctx context.Context, cases []Case) []Result {
	results := make([]Result, 0, len(cases))
	for _, item := range cases {
		results = append(results, r.RunCase(ctx, item))
	}
	return results
}

// Summarize 聚合一批结果。
func Summarize(results []Result) Summary {
	summary := Summary{Total: len(results)}
	if summary.Total == 0 {
		return summary
	}
	var attempts, steps, promptChars, failures int
	for _, item := range results {
		if item.Passed {
			summary.Passed++
		}
		attempts += item.Attempts
		steps += item.ToolSteps
		promptChars += item.PromptChars
		failures += item.ToolFailures
	}
	summary.PassRate = float64(summary.Passed) / float64(summary.Total)
	summary.AvgAttempts = float64(attempts) / float64(summary.Total)
	summary.AvgToolSteps = float64(steps) / float64(summary.Total)
	summary.AvgPromptChars = float64(promptChars) / float64(summary.Total)
	if steps > 0 {
		summary.ToolFailureRate = float64(failures) / float64(steps)
	}
	return summary
}

// metricsRecorder 从事件流里取评测需要的计数。
type metricsRecorder struct {
	promptChars  int
	toolFailures int
}

func (m *metricsRecorder) Event(typ string, data map[string]any) {
	switch typ {
	case "prompt_built":
		if value, ok := data["prompt_chars"].(int); ok {
			m.promptChars += value
		}
	case "tool_executed":
		if ok, _ := data["ok"].(bool); !ok {
			m.toolFailures++
		}
	}
}

// CheckFileEquals 判定文件内容等于期望值。
func CheckFileEquals(path, want string) func(root string) error {
	return func(root string) error {
		payload, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(payload)) != strings.TrimSpace(want) {
			return &checkError{message: "文件内容不符: " + path}
		}
		return nil
	}
}

// CheckFileContains 判定文件包含指定片段。
func CheckFileContains(path, want string) func(root string) error {
	return func(root string) error {
		payload, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if !strings.Contains(string(payload), want) {
			return &checkError{message: "文件缺少片段: " + path}
		}
		return nil
	}
}

// CheckFileMissing 判定文件不存在。
func CheckFileMissing(path string) func(root string) error {
	return func(root string) error {
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			return &checkError{message: "文件不应存在: " + path}
		}
		return nil
	}
}

type checkError struct{ message string }

func (e *checkError) Error() string { return e.message }
