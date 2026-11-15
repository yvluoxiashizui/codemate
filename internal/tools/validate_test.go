package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsDirectoryAsFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := Builtins(root, nil, nil)

	result := registry.Execute(context.Background(), "read_file", map[string]any{"path": "sub"}, Context{CWD: root})
	if result.OK {
		t.Fatal("目录不能当文件读")
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s, output = %s", result.Meta.ErrorCode, result.Output)
	}
}

func TestValidateRejectsEmptyPattern(t *testing.T) {
	root := t.TempDir()
	registry := Builtins(root, nil, nil)

	result := registry.Execute(context.Background(), "grep_files", map[string]any{"pattern": "  "}, Context{CWD: root})
	if result.OK {
		t.Fatal("空检索词应被拒绝")
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}

func TestValidateRejectsOutOfRangeTimeout(t *testing.T) {
	root := t.TempDir()
	registry := Builtins(root, nil, nil)

	result := registry.Execute(context.Background(), "run_command", map[string]any{"command": "go", "timeout": 9999}, Context{CWD: root})
	if result.OK {
		t.Fatal("超时越界应被拒绝")
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}

func TestValidateRejectsDirectoryAsWriteTarget(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := Builtins(root, nil, nil)

	result := registry.Execute(context.Background(), "write_file", map[string]any{"path": "pkg", "content": "x"}, Context{CWD: root})
	if result.OK {
		t.Fatal("目录不能当文件写")
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}

func TestValidateRejectsEmptySearchInPatch(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "a.go")
	if err := os.WriteFile(target, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := Builtins(root, nil, nil)

	result := registry.Execute(context.Background(), "patch_file", map[string]any{
		"path": "a.go",
		"replacements": []any{
			map[string]any{"search": "", "replace": "x"},
		},
	}, Context{CWD: root})
	if result.OK {
		t.Fatal("空 search 应被拒绝")
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}

func TestValidateAllowsValidRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := Builtins(root, nil, nil)

	result := registry.Execute(context.Background(), "read_file", map[string]any{"path": "a.txt"}, Context{CWD: root})
	if !result.OK {
		t.Fatalf("合法读取应通过: %s", result.Output)
	}
	if result.Meta.Status != StatusOK {
		t.Fatalf("status = %s", result.Meta.Status)
	}
}
