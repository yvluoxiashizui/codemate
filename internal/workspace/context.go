package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Context 是提示词前缀里描述仓库状态的信息。
type Context struct {
	Root          string
	Branch        string
	Dirty         bool
	TrackedFiles  int
	RecentCommits []string
	ProjectDocs   []string
}

// projectDocs 是会被当成项目说明的文件名。
var projectDocs = []string{"README.md", "AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md"}

// BuildContext 收集仓库状态。不是 git 仓库时只返回根路径与顶层说明文件。
func BuildContext(root string) Context {
	info := Context{Root: root}

	abs, err := filepath.Abs(root)
	if err == nil {
		info.Root = filepath.Clean(abs)
	}

	if branch := gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD"); branch != "" {
		info.Branch = branch
		status := gitOutput(root, "status", "--porcelain")
		info.Dirty = strings.TrimSpace(status) != ""
		if count := gitOutput(root, "ls-files"); count != "" {
			info.TrackedFiles = len(strings.Split(strings.TrimSpace(count), "\n"))
		}
		for _, line := range strings.Split(gitOutput(root, "log", "--oneline", "-3"), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				info.RecentCommits = append(info.RecentCommits, line)
			}
		}
	}

	for _, name := range projectDocs {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			info.ProjectDocs = append(info.ProjectDocs, name)
		}
	}
	return info
}

// Render 把仓库状态渲染成前缀里的一段。
func (c Context) Render() string {
	var builder strings.Builder
	builder.WriteString("root: " + c.Root + "\n")
	if c.Branch != "" {
		state := "clean"
		if c.Dirty {
			state = "has uncommitted changes"
		}
		builder.WriteString("git branch: " + c.Branch + " (" + state + ")\n")
		if c.TrackedFiles > 0 {
			builder.WriteString("tracked files: " + itoa(c.TrackedFiles) + "\n")
		}
	}
	for _, commit := range c.RecentCommits {
		builder.WriteString("recent commit: " + commit + "\n")
	}
	if len(c.ProjectDocs) > 0 {
		builder.WriteString("project docs: " + strings.Join(c.ProjectDocs, ", ") + "\n")
	}
	return builder.String()
}

func gitOutput(root string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(output), "\n")
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 8)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
