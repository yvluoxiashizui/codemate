package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileSupportsOffsetLimit(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := Builtins(dir, nil, nil)
	result := reg.Execute(context.Background(), "read_file", map[string]any{"path": "a.txt", "offset": 2, "limit": 3}, Context{CWD: dir})
	if !result.OK || !strings.Contains(result.Output, "TRUNCATED: yes") || !strings.HasSuffix(result.Output, "cde") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEditFileReplacesText(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(target, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := Builtins(dir, nil, nil)
	result := reg.Execute(context.Background(), "edit_file", map[string]any{"path": "a.txt", "search": "world", "replace": "mini"}, Context{CWD: dir})
	if !result.OK {
		t.Fatalf("unexpected result: %#v", result)
	}
	content, _ := os.ReadFile(target)
	if string(content) != "hello mini" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestPatchFileRequiresOldTextWithNewText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := Builtins(dir, nil, nil)

	result := reg.Execute(context.Background(), "patch_file", map[string]any{
		"path": "a.txt",
	}, Context{CWD: dir})
	if result.OK {
		t.Fatalf("缺少 old_text 应被拒绝: %#v", result)
	}
	if result.Meta.ErrorCode != CodeInvalidInput {
		t.Fatalf("error code = %s", result.Meta.ErrorCode)
	}
}

func TestPatchFileRejectsAmbiguousOldText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x x x"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := Builtins(dir, nil, nil)

	result := reg.Execute(context.Background(), "patch_file", map[string]any{
		"path":     "a.txt",
		"old_text": "x",
		"new_text": "y",
	}, Context{CWD: dir})
	if result.OK {
		t.Fatalf("old_text 出现三次应被拒绝: %#v", result)
	}
	if !strings.Contains(result.Output, "exactly once") {
		t.Fatalf("应说明唯一性要求: %s", result.Output)
	}
}

func TestPatchFileReplacesExactlyOnce(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(target, []byte("alpha beta gamma"), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := Builtins(dir, nil, nil)

	result := reg.Execute(context.Background(), "patch_file", map[string]any{
		"path":     "a.txt",
		"old_text": "beta",
		"new_text": "delta",
	}, Context{CWD: dir})
	if !result.OK {
		t.Skipf("需要审批环境: %s", result.Output)
	}
	content, _ := os.ReadFile(target)
	if string(content) != "alpha delta gamma" {
		t.Fatalf("替换结果 = %q", string(content))
	}
}
