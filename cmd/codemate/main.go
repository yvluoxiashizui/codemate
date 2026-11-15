package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yvluoxiashizui/codemate/internal/config"
	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/manage"
	"github.com/yvluoxiashizui/codemate/internal/mcp"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/model"
	"github.com/yvluoxiashizui/codemate/internal/permissions"
	"github.com/yvluoxiashizui/codemate/internal/prompt"
	"github.com/yvluoxiashizui/codemate/internal/protocol"
	"github.com/yvluoxiashizui/codemate/internal/runs"
	"github.com/yvluoxiashizui/codemate/internal/session"
	"github.com/yvluoxiashizui/codemate/internal/skills"
	"github.com/yvluoxiashizui/codemate/internal/tools"
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
	mcpResult := mcp.CreateBackedTools(ctx, cwd, runtime.MCPServers)
	definitions := append(toolRegistry.List(), mcpResult.Tools...)
	toolRegistry = tools.NewRegistry(definitions, tools.Metadata{Skills: discoveredSkills, MCPServers: mcpResult.Servers}).WithDisposer(mcpResult.Dispose)
	defer func() {
		_ = toolRegistry.Dispose(ctx)
	}()

	pm, _ := permissions.New(cwd, config.PermissionsPath(), nil)
	systemPrompt := prompt.Build(ctx, prompt.Args{
		CWD:               cwd,
		PermissionSummary: pm.Summary(),
		Skills:            discoveredSkills,
		MCPServers:        toolRegistry.MCPServers(),
	})
	prefix := protocol.BuildPrefix(
		protocol.Workspace{Root: cwd},
		toolSpecs(toolRegistry),
	) + "\n\n" + systemPrompt

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

	store := session.Store{Dir: config.SessionsDir()}
	sessionID := startup.ResumeID
	var entries []ctxpkg.HistoryEntry
	if sessionID != "" {
		record, err := loadSessionRecord(store, sessionID)
		if err != nil {
			return err
		}
		sessionID = record.ID
		entries = record.Messages
	} else {
		record := session.NewRecord(cwd, entries)
		sessionID = record.ID
	}

	app := session.New(session.Args{
		CWD:        cwd,
		Tools:      toolRegistry,
		Model:      adapter,
		Runtime:    runtimePtr(runtime, runtimeErr),
		Entries:    entries,
		Prefix:     prefix,
		Permission: pm,
		History:    session.History{Path: config.HistoryPath()},
		Store:      store,
		SessionID:  sessionID,
		Recorder:   run,
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
	ManagementArgs []string
}

func parseStartupArgs(argv []string) (startupArgs, error) {
	out := startupArgs{ManagementArgs: []string{}}
	for i := 0; i < len(argv); i++ {
		switch argv[i] {
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

// toolSpecs 把工具定义转成提示词里需要描述的信息。
func toolSpecs(registry *tools.Registry) []protocol.ToolSpec {
	risky := map[string]bool{
		"write_file":  true,
		"edit_file":   true,
		"patch_file":  true,
		"run_command": true,
	}
	definitions := registry.List()
	specs := make([]protocol.ToolSpec, 0, len(definitions))
	for _, definition := range definitions {
		specs = append(specs, protocol.ToolSpec{
			Name:        definition.Name,
			Description: definition.Description,
			Params:      paramsOf(definition.InputSchema),
			Risky:       risky[definition.Name],
		})
	}
	return specs
}

// paramsOf 从 JSON Schema 里取出参数名与类型。
func paramsOf(schema map[string]any) []protocol.Param {
	if schema == nil {
		return nil
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}
	params := make([]protocol.Param, 0, len(properties))
	for name, raw := range properties {
		paramType := "string"
		if field, ok := raw.(map[string]any); ok {
			if value, ok := field["type"].(string); ok {
				paramType = value
			}
		}
		params = append(params, protocol.Param{Name: name, Type: paramType})
	}
	return params
}
