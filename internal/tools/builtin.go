package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yvluoxiashizui/codemate/internal/filereview"
	"github.com/yvluoxiashizui/codemate/internal/security"
	"github.com/yvluoxiashizui/codemate/internal/workspace"
)

type SkillLoader interface {
	LoadSkill(ctx context.Context, name string) (string, error)
}

func Builtins(cwd string, permission PermissionManager, skillLoader SkillLoader) *Registry {
	definitions := []Definition{
		listFilesTool(),
		searchTool(),
		readFileTool(),
		writeFileTool(),
		patchFileTool(),
		runShellTool(),
		loadSkillTool(skillLoader),
		delegateTool(),
	}
	return NewRegistry(definitions, Metadata{})
}

func listFilesTool() Definition {
	return Definition{
		Name:        "list_files",
		Validate:    validateListFiles,
		Risk:        RiskLow,
		ReadOnly:    true,
		Description: "List files in a directory relative to the workspace root.",
		InputSchema: objectSchema(map[string]any{"path": map[string]any{"type": "string"}}, nil),
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Path string `json:"path"`
			}
			_ = json.Unmarshal(raw, &input)
			target, err := workspace.Resolve(ctx, tc.CWD, defaultString(input.Path, "."), "list", tc.Permission)
			if err != nil {
				return Error(err.Error())
			}
			entries, err := os.ReadDir(target)
			if err != nil {
				return Error(err.Error())
			}
			lines := []string{}
			for i, entry := range entries {
				if i >= 200 {
					break
				}
				prefix := "file"
				if entry.IsDir() {
					prefix = "dir "
				}
				lines = append(lines, prefix+" "+entry.Name())
			}
			if len(lines) == 0 {
				return Success("(empty)")
			}
			return Success(strings.Join(lines, "\n"))
		},
	}
}

func searchTool() Definition {
	return Definition{
		Name:        "search",
		Validate:    validateGrepFiles,
		Risk:        RiskLow,
		ReadOnly:    true,
		Description: "Search for text in files using ripgrep.",
		InputSchema: objectSchema(map[string]any{"pattern": map[string]any{"type": "string"}, "path": map[string]any{"type": "string"}}, []string{"pattern"}),
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			args := []string{"-n", "--no-heading", input.Pattern}
			if input.Path != "" {
				target, err := workspace.Resolve(ctx, tc.CWD, input.Path, "search", tc.Permission)
				if err != nil {
					return Error(err.Error())
				}
				args = append(args, target)
			} else {
				args = append(args, ".")
			}
			cmd := exec.CommandContext(ctx, "rg", args...)
			cmd.Dir = tc.CWD
			output, _ := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))
			if text == "" {
				text = "(no matches)"
			}
			return Success(text)
		},
	}
}

func readFileTool() Definition {
	return Definition{
		Name:        "read_file",
		Validate:    validateReadFile,
		Risk:        RiskLow,
		ReadOnly:    true,
		Description: "Read a UTF-8 text file relative to the workspace root. Large files can be read in chunks via offset and limit.",
		InputSchema: objectSchema(map[string]any{"path": map[string]any{"type": "string"}, "offset": map[string]any{"type": "number"}, "limit": map[string]any{"type": "number"}}, []string{"path"}),
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			target, err := workspace.Resolve(ctx, tc.CWD, input.Path, "read", tc.Permission)
			if err != nil {
				return Error(err.Error())
			}
			bytes, err := os.ReadFile(target)
			if err != nil {
				return Error(err.Error())
			}
			content := string(bytes)
			limit := input.Limit
			if limit <= 0 {
				limit = 8000
			}
			if limit > 20000 {
				limit = 20000
			}
			offset := input.Offset
			if offset < 0 {
				offset = 0
			}
			if offset > len(content) {
				offset = len(content)
			}
			end := offset + limit
			if end > len(content) {
				end = len(content)
			}
			truncated := "no"
			if end < len(content) {
				truncated = "yes - call read_file again with offset " + strconv.Itoa(end)
			}
			header := strings.Join([]string{
				"FILE: " + input.Path,
				"OFFSET: " + strconv.Itoa(offset),
				"END: " + strconv.Itoa(end),
				"TOTAL_CHARS: " + strconv.Itoa(len(content)),
				"TRUNCATED: " + truncated,
				"",
			}, "\n")
			return Success(header + content[offset:end])
		},
	}
}

func writeFileTool() Definition {
	return reviewedWriteTool("write_file", "Write a UTF-8 text file relative to the workspace root.")
}

func reviewedWriteTool(name, description string) Definition {
	return Definition{
		Name:        name,
		Description: description,
		InputSchema: objectSchema(map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, []string{"path", "content"}),
		Risk:        RiskHigh,
		ReadOnly:    false,
		Validate:    validateWriteFile,
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			tc.ReportAffected(input.Path)
			target, err := workspace.Resolve(ctx, tc.CWD, input.Path, "write", tc.Permission)
			if err != nil {
				return Error(err.Error())
			}
			return fromReview(filereview.ApplyReviewedChange(ctx, tc.Permission, input.Path, target, input.Content))
		},
	}
}

func patchFileTool() Definition {
	return Definition{
		Name:        "patch_file",
		Validate:    validatePatchFile,
		Risk:        RiskHigh,
		ReadOnly:    false,
		Description: "Replace one exact piece of text in a file. old_text must occur exactly once.",
		InputSchema: objectSchema(map[string]any{
			"path":     map[string]any{"type": "string"},
			"old_text": map[string]any{"type": "string"},
			"new_text": map[string]any{"type": "string"},
		}, []string{"path", "old_text", "new_text"}),
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Path    string `json:"path"`
				OldText string `json:"old_text"`
				NewText string `json:"new_text"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			tc.ReportAffected(input.Path)
			target, err := workspace.Resolve(ctx, tc.CWD, input.Path, "write", tc.Permission)
			if err != nil {
				return Error(err.Error())
			}
			bytes, err := os.ReadFile(target)
			if err != nil {
				return Error(err.Error())
			}
			content := string(bytes)
			// 校验阶段已经确认只出现一次，这里再确认，防止并发改动。
			if count := strings.Count(content, input.OldText); count != 1 {
				return Error("old_text must occur exactly once, found " + strconv.Itoa(count))
			}
			content = strings.Replace(content, input.OldText, input.NewText, 1)
			result := fromReview(filereview.ApplyReviewedChange(ctx, tc.Permission, input.Path, target, content))
			if !result.OK {
				return result
			}
			return Success("Patched " + input.Path)
		},
	}
}

func runShellTool() Definition {
	return Definition{
		Name:        "run_shell",
		Validate:    validateRunCommand,
		Risk:        RiskHigh,
		ReadOnly:    false,
		Description: "Run a common development command from an allowlist.",
		InputSchema: objectSchema(map[string]any{
			"command": map[string]any{"type": "string"},
			"args":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"cwd":     map[string]any{"type": "string"},
		}, []string{"command"}),
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Command string   `json:"command"`
				Args    []string `json:"args"`
				CWD     string   `json:"cwd"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			effectiveCWD := tc.CWD
			if input.CWD != "" {
				target, err := workspace.Resolve(ctx, tc.CWD, input.CWD, "list", tc.Permission)
				if err != nil {
					return Error(err.Error())
				}
				effectiveCWD = target
			}
			command := input.Command
			args := input.Args
			if looksLikeShellSnippet(command, args) {
				args = []string{"-lc", command}
				command = "bash"
			} else if !allowedCommand(command) {
				return Error("Command not allowed: " + command)
			}
			if tc.Permission != nil {
				if err := tc.Permission.EnsureCommand(ctx, command, args, effectiveCWD); err != nil {
					return Error(err.Error())
				}
			}
			cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(cmdCtx, command, args...)
			cmd.Dir = effectiveCWD
			cmd.Env = security.ShellEnv(nil, nil)
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			err := cmd.Run()
			text := strings.TrimSpace(output.String())
			if err != nil {
				if text == "" {
					text = err.Error()
				}
				return Error(text)
			}
			return Success(text)
		},
	}
}

func loadSkillTool(loader SkillLoader) Definition {
	return Definition{
		Name:        "load_skill",
		Risk:        RiskLow,
		ReadOnly:    true,
		Description: "Load the full contents of a named SKILL.md file so you can follow that workflow accurately.",
		InputSchema: objectSchema(map[string]any{"name": map[string]any{"type": "string"}}, []string{"name"}),
		Run: func(ctx context.Context, raw json.RawMessage, _ Context) Result {
			var input struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			if loader == nil {
				return Error("Unknown skill: " + input.Name)
			}
			content, err := loader.LoadSkill(ctx, input.Name)
			if err != nil {
				return Error(err.Error())
			}
			return Success(content)
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{"type": "object", "properties": properties}
	if required != nil {
		schema["required"] = required
	}
	return schema
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func allowedCommand(command string) bool {
	allowed := map[string]bool{"pwd": true, "ls": true, "find": true, "rg": true, "cat": true, "echo": true, "env": true, "grep": true, "git": true, "npm": true, "node": true, "python3": true, "pytest": true, "bash": true, "sh": true, "bun": true, "sed": true, "head": true, "tail": true, "wc": true, "go": true}
	return allowed[command]
}

func looksLikeShellSnippet(command string, args []string) bool {
	if len(args) > 0 {
		return false
	}
	return regexp.MustCompile(`[|&;<>()$` + "`" + `]`).MatchString(command)
}

func absPath(path string) string {
	out, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return out
}

func fromReview(result filereview.Result) Result {
	return Result{OK: result.OK, Output: result.Output}
}
