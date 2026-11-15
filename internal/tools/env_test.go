package tools

import (
	"context"
	"strings"
	"testing"
)

func TestRunCommandDoesNotInheritSecrets(t *testing.T) {
	t.Setenv("CODEMATE_TEST_TOKEN", "runcommandsecret123")

	root := t.TempDir()
	registry := Builtins(root, nil, nil)
	result := registry.Execute(context.Background(), "run_command", map[string]any{
		"command": "env",
	}, Context{CWD: root})

	if !result.OK {
		t.Skipf("环境不允许执行命令: %s", result.Output)
	}
	if strings.Contains(result.Output, "runcommandsecret123") {
		t.Fatalf("子进程不应看到密钥: %s", result.Output)
	}
}

func TestRunCommandKeepsPathInEnv(t *testing.T) {
	root := t.TempDir()
	registry := Builtins(root, nil, nil)
	result := registry.Execute(context.Background(), "run_command", map[string]any{
		"command": "env",
	}, Context{CWD: root})

	if !result.OK {
		t.Skipf("环境不允许执行命令: %s", result.Output)
	}
	if !strings.Contains(result.Output, "PATH=") {
		t.Fatalf("白名单应保留 PATH: %s", result.Output)
	}
}

func TestRunCommandDropsUnlistedVariables(t *testing.T) {
	t.Setenv("CODEMATE_UNLISTED_VAR", "shouldnotappear")

	root := t.TempDir()
	registry := Builtins(root, nil, nil)
	result := registry.Execute(context.Background(), "run_command", map[string]any{
		"command": "env",
	}, Context{CWD: root})

	if !result.OK {
		t.Skipf("环境不允许执行命令: %s", result.Output)
	}
	if strings.Contains(result.Output, "CODEMATE_UNLISTED_VAR") {
		t.Fatalf("白名单外的变量不应传入: %s", result.Output)
	}
}
