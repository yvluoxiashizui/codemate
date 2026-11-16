package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindDotEnvWalksUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(root, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := FindDotEnv(nested); got != envPath {
		t.Fatalf("应向上找到 .env: %q", got)
	}
}

func TestFindDotEnvMissing(t *testing.T) {
	if got := FindDotEnv(t.TempDir()); got != "" {
		t.Fatalf("没有 .env 时应返回空: %q", got)
	}
}

func TestParseDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	body := "# 注释\n\nANTHROPIC_MODEL=claude-3\nMINI_CODE_TIMEOUT=\"30\"\nMINI_CODE_PROVIDER='openai'\nBAD_LINE\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	values, err := ParseDotEnv(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if values["ANTHROPIC_MODEL"] != "claude-3" {
		t.Fatalf("普通值 = %q", values["ANTHROPIC_MODEL"])
	}
	if values["MINI_CODE_TIMEOUT"] != "30" {
		t.Fatalf("双引号应去掉: %q", values["MINI_CODE_TIMEOUT"])
	}
	if values["MINI_CODE_PROVIDER"] != "openai" {
		t.Fatalf("单引号应去掉: %q", values["MINI_CODE_PROVIDER"])
	}
	if _, exists := values["BAD_LINE"]; exists {
		t.Fatal("没有等号的行应被忽略")
	}
}

func TestDotEnvValuesReturnsNilWhenAbsent(t *testing.T) {
	if values := DotEnvValues(t.TempDir()); values != nil {
		t.Fatalf("没有文件时应返回 nil: %#v", values)
	}
}
