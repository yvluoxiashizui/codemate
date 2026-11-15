package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildContextWithoutGit(t *testing.T) {
	dir := t.TempDir()
	info := BuildContext(dir)

	if info.Branch != "" {
		t.Fatalf("非 git 仓库不应有分支: %q", info.Branch)
	}
	if !strings.Contains(info.Render(), "root:") {
		t.Fatalf("渲染结果应含根路径: %q", info.Render())
	}
}

func TestBuildContextDetectsProjectDocs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info := BuildContext(dir)
	if len(info.ProjectDocs) != 1 || info.ProjectDocs[0] != "README.md" {
		t.Fatalf("project docs = %#v", info.ProjectDocs)
	}
	if !strings.Contains(info.Render(), "README.md") {
		t.Fatalf("渲染结果应含说明文件: %q", info.Render())
	}
}

func TestBuildContextReadsGitState(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("环境里没有 git")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		command := exec.Command("git", args...)
		command.Dir = dir
		if err := command.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")

	info := BuildContext(dir)
	if info.Branch != "main" {
		t.Fatalf("branch = %q", info.Branch)
	}
	if info.Dirty {
		t.Fatal("刚提交后工作区应是干净的")
	}
	if len(info.RecentCommits) == 0 {
		t.Fatal("应读到最近提交")
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !BuildContext(dir).Dirty {
		t.Fatal("改动后应标记为脏")
	}
}
