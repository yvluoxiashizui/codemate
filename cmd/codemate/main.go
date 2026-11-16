package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yvluoxiashizui/codemate/internal/agent"
	"github.com/yvluoxiashizui/codemate/internal/config"
	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/manage"
	"github.com/yvluoxiashizui/codemate/internal/memory"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/model"
	"github.com/yvluoxiashizui/codemate/internal/permissions"
	"github.com/yvluoxiashizui/codemate/internal/prefix"
	"github.com/yvluoxiashizui/codemate/internal/prompt"
	"github.com/yvluoxiashizui/codemate/internal/protocol"
	"github.com/yvluoxiashizui/codemate/internal/runs"
	"github.com/yvluoxiashizui/codemate/internal/session"
	"github.com/yvluoxiashizui/codemate/internal/skills"
	"github.com/yvluoxiashizui/codemate/internal/tools"
	"github.com/yvluoxiashizui/codemate/internal/workspace"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, argv []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	startup, err := parseStartupArgs(argv)
	if err != nil {
		return err
	}
	if out, handled, err := manage.Handle(ctx, cwd, startup.ManagementArgs); handled || err != nil {
		if out != "" {
			fmt.Println(out)
		}
		return err
	}

	runtime, runtimeErr := config.LoadRuntime(cwd)
	skillStore := skills.NewStore(cwd, homeDir())
	discoveredSkills, _ := skillStore.Discover(ctx)
	toolRegistry := tools.Builtins(cwd, nil, skillStore)
	toolRegistry = tools.NewRegistry(toolRegistry.List(), tools.Metadata{Skills: discoveredSkills})

	if startup.MaxNewTokens > 0 {
		runtime.MaxOutputTokens = startup.MaxNewTokens
	}

	pm, _ := permissions.New(cwd, config.PermissionsPath(), nil)
	if startup.Approval != "" {
		pm.SetMode(permissions.Mode(startup.Approval))
	}
	systemPrompt := prompt.Build(ctx, prompt.Args{
		CWD:               cwd,
		PermissionSummary: pm.Summary(),
		Skills:            discoveredSkills,
		MCPServers:        toolRegistry.MCPServers(),
	})
	wsContext := workspace.BuildContext(cwd)
	prefixText := protocol.BuildPrefix(
		protocol.Workspace{
			Root:          wsContext.Root,
			Branch:        wsContext.Branch,
			Dirty:         wsContext.Dirty,
			TrackedFiles:  wsContext.TrackedFiles,
			RecentCommits: wsContext.RecentCommits,
			ProjectDocs:   wsContext.ProjectDocs,
		},
		protocol.ToolSpecs(toolRegistry),
	) + "\n\n" + systemPrompt
	prefixSource := prefix.New(func() string {
		info := workspace.BuildContext(cwd)
		return protocol.BuildPrefix(protocol.Workspace{
			Root:          info.Root,
			Branch:        info.Branch,
			Dirty:         info.Dirty,
			TrackedFiles:  info.TrackedFiles,
			RecentCommits: info.RecentCommits,
			ProjectDocs:   info.ProjectDocs,
		}, protocol.ToolSpecs(toolRegistry)) + "\n\n" + systemPrompt
	}, func() string { return workspace.Fingerprint(cwd) })
	_ = prefixText
	prefix := prefixSource.Get()

	var adapter message.Completer
	mockMode := os.Getenv("MINI_CODE_MODEL_MODE") == "mock"
	if mockMode {
		adapter = model.Mock{}
	} else if runtimeErr != nil {
		adapter = model.ErrorModel{Err: runtimeErr}
	} else {
		adapter, err = model.NewFromRuntime(runtime, toolRegistry)
		if err != nil {
			return err
		}
	}

	runStore := runs.NewStore(cwd)
	run, runErr := runStore.Start(map[string]any{"cwd": cwd})
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "运行工件目录不可用:", runErr)
	}
	defer func() {
		_ = run.Close("exit")
	}()

	mem, memErr := memory.Open(cwd)
	if memErr != nil {
		fmt.Fprintln(os.Stderr, "记忆文件不可用:", memErr)
	}

	store := session.Store{Dir: config.SessionsDir()}
	sessionID := startup.ResumeID
	fingerprint := workspace.Fingerprint(cwd)
	var entries []ctxpkg.HistoryEntry
	if sessionID != "" {
		record, err := loadSessionRecord(store, sessionID)
		if err != nil {
			return err
		}
		resumeHint := ""
		if record.CheckpointPath != "" {
			if checkpoint, err := runs.LoadCheckpoint(record.CheckpointPath); err == nil {
				verdict := runs.EvaluateResume(checkpoint, runtimeIdentity(cwd, fingerprint, runtime.Model, toolRegistry.Signature()), func(path string) string {
					return workspace.Freshness(cwd, path)
				})
				resumeHint = runs.ResumeHint(verdict)
			}
		}
		if !session.Resumable(record, fingerprint) {
			fmt.Fprintln(os.Stderr, "工作区已变化，未恢复会话，改为新建。")
			sessionID = ""
		} else {
			sessionID = record.ID
			entries = record.Messages
			if resumeHint != "" {
				prefix = resumeHint + "\n\n" + prefix
			}
		}
	}
	if sessionID == "" {
		record := session.NewRecord(cwd, entries)
		sessionID = record.ID
	}

	app := session.New(session.Args{
		CWD:         cwd,
		Tools:       toolRegistry,
		Model:       adapter,
		Runtime:     runtimePtr(runtime, runtimeErr),
		Entries:     entries,
		Prefix:      prefix,
		PrefixFn:    prefixSource.Get,
		Fingerprint: fingerprint,
		Permission:  pm,
		History:     session.History{Path: config.HistoryPath()},
		Store:       store,
		SessionID:   sessionID,
		Recorder:    run,
		ReadOnly:    startup.ReadOnly,
		Allowed:     startup.AllowedTools,
		State:       run,
		Report:      run,
		Memory:      mem,
		Freshness: func(path string) string {
			return workspace.Freshness(cwd, path)
		},
		Checkpoint: checkpointWriter{
			run:           run,
			cwd:           cwd,
			toolSignature: toolRegistry.Signature(),
			model:         runtime.Model,
			fingerprint:   fingerprint,
			maxSteps:      agent.DefaultMaxSteps,
		},
	})
	if isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		return app.RunTUI(ctx)
	}
	fmt.Println("MiniCode Go")
	if mockMode {
		fmt.Println("model: mock/offline")
	} else if runtimeErr != nil {
		fmt.Println("model: not-configured")
	} else {
		fmt.Println("provider:", runtime.Provider)
		fmt.Println("model:", runtime.Model)
	}
	return app.Run(ctx)
}

type startupArgs struct {
	ResumeID       string
	ReadOnly       bool
	AllowedTools   []string
	Approval       string
	MaxNewTokens   int
	ManagementArgs []string
}

func parseStartupArgs(argv []string) (startupArgs, error) {
	out := startupArgs{ManagementArgs: []string{}}
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
		case "--max-new-tokens":
			if i+1 >= len(argv) {
				return startupArgs{}, fmt.Errorf("missing value for --max-new-tokens")
			}
			value, err := strconv.Atoi(argv[i+1])
			if err != nil || value <= 0 {
				return startupArgs{}, fmt.Errorf("invalid value for --max-new-tokens: %s", argv[i+1])
			}
			out.MaxNewTokens = value
			i++
		case "--approval":
			if i+1 >= len(argv) {
				return startupArgs{}, fmt.Errorf("missing value for --approval")
			}
			switch argv[i+1] {
			case "ask", "auto", "never":
				out.Approval = argv[i+1]
				i++
			default:
				return startupArgs{}, fmt.Errorf("invalid value for --approval: %s", argv[i+1])
			}
		case "--tool":
			if i+1 >= len(argv) || strings.TrimSpace(argv[i+1]) == "" {
				return startupArgs{}, fmt.Errorf("missing value for --tool")
			}
			out.AllowedTools = append(out.AllowedTools, argv[i+1])
			i++
		case "--read-only":
			out.ReadOnly = true
		case "--resume":
			if i+1 >= len(argv) || strings.TrimSpace(argv[i+1]) == "" {
				return startupArgs{}, fmt.Errorf("missing value for --resume")
			}
			out.ResumeID = argv[i+1]
			i++
		default:
			out.ManagementArgs = append(out.ManagementArgs, argv[i])
		}
	}
	return out, nil
}

func loadSessionRecord(store session.Store, id string) (session.Record, error) {
	if id == "latest" {
		return store.Latest()
	}
	return store.Load(id)
}

func usage() string {
	return "minicode"
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}

func runtimePtr(runtime config.Runtime, err error) *config.Runtime {
	if err != nil {
		return nil
	}
	return &runtime
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}

// runtimeIdentity 是一次运行的运行时身份，用于检查点比对。
func runtimeIdentity(cwd, fingerprint, model, toolSignature string) map[string]any {
	return map[string]any{
		"cwd":                   cwd,
		"workspace_fingerprint": fingerprint,
		"model":                 model,
		"max_steps":             agent.DefaultMaxSteps,
		"tool_signature":        toolSignature,
	}
}
