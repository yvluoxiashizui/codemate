package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFingerprintIsStableForSamePath(t *testing.T) {
	dir := t.TempDir()
	first := Fingerprint(dir)
	second := Fingerprint(dir)
	if first != second {
		t.Fatal("同一路径两次计算应一致")
	}
	if len(first) != 16 {
		t.Fatalf("指纹长度 = %d, want 16", len(first))
	}
}

func TestFingerprintDiffersBetweenPaths(t *testing.T) {
	first := Fingerprint(t.TempDir())
	second := Fingerprint(t.TempDir())
	if first == second {
		t.Fatal("不同路径应得到不同指纹")
	}
}

func TestFingerprintFollowsGitHead(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refPath := filepath.Join(gitDir, "refs", "heads", "main")
	if err := os.WriteFile(refPath, []byte("aaaa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := Fingerprint(dir)

	if err := os.WriteFile(refPath, []byte("bbbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := Fingerprint(dir)
	if before == after {
		t.Fatal("提交变化后指纹应改变")
	}
}

func TestFingerprintWithoutGit(t *testing.T) {
	dir := t.TempDir()
	if got := Fingerprint(dir); got == "" {
		t.Fatal("没有 git 的目录也应给出指纹")
	}
}
