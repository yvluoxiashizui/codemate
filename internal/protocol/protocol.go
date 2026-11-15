package protocol

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

// Kind 是应答解析后的动作类型。
type Kind string

const (
	// KindTool 表示这一轮要执行一次工具调用。
	KindTool Kind = "tool"
	// KindFinal 表示模型给出了最终答案。
	KindFinal Kind = "final"
	// KindRetry 表示应答格式不合法，需要把提示写回历史后重新请求。
	KindRetry Kind = "retry"
)

// Action 是一次应答的解析结果。
type Action struct {
	Kind   Kind
	Tool   string
	Args   map[string]any
	Final  string
	Notice string
}

// Parse 把模型原始输出解析成工具调用、最终答案或格式重试。
func Parse(raw string) Action {
	text := raw

	if strings.Contains(text, "<tool>") {
		if idx, ok := firstIndex(text, "<final>"); !ok || strings.Index(text, "<tool>") < idx {
			return parseJSONTool(text)
		}
	}
	if strings.Contains(text, "<tool") {
		if idx, ok := firstIndex(text, "<final>"); !ok || strings.Index(text, "<tool") < idx {
			if action, ok := parseXMLTool(text); ok {
				return action
			}
			return retry("tool block is malformed")
		}
	}
	if strings.Contains(text, "<final>") {
		final := strings.TrimSpace(extract(text, "final"))
		if final == "" {
			return retry("final answer is empty")
		}
		return Action{Kind: KindFinal, Final: final}
	}

	trimmed := strings.TrimSpace(text)
	if trimmed != "" {
		return Action{Kind: KindFinal, Final: trimmed}
	}
	return retry("response is empty")
}

// NoticeText 把重试提示包装成写回历史的一行文本。
func NoticeText(action Action) string {
	notice := action.Notice
	if notice == "" {
		notice = "tool output is malformed"
	}
	return "Runtime notice: " + notice + ". " +
		"Reply with one valid <tool> call or a non-empty <final> answer. " +
		`For multi-line files prefer <tool name="write_file" path="a.go"><content>...</content></tool>.`
}

func parseJSONTool(text string) Action {
	body := extract(text, "tool")
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return retry("model returned malformed tool JSON")
	}
	name := strings.TrimSpace(asString(payload["name"]))
	if name == "" {
		return retry("tool payload is missing a tool name")
	}
	args := map[string]any{}
	switch value := payload["args"].(type) {
	case nil:
	case map[string]any:
		args = value
	default:
		return retry("tool args must be a JSON object")
	}
	return Action{Kind: KindTool, Tool: name, Args: args}
}

func parseXMLTool(text string) (Action, bool) {
	attrs := parseAttrs(attributeText(text))
	name := strings.TrimSpace(attrs["name"])
	if name == "" {
		return Action{}, false
	}
	args := map[string]any{}
	for key, value := range attrs {
		if key == "name" {
			continue
		}
		args[key] = value
	}

	body := xmlBody(text)
	for _, key := range []string{"content", "old_text", "new_text", "command", "task", "pattern", "path"} {
		if strings.Contains(body, "<"+key+">") {
			args[key] = extractRaw(body, key)
		}
	}
	if name == "write_file" {
		if _, ok := args["content"]; !ok {
			if trimmed := strings.Trim(body, "\n"); strings.TrimSpace(trimmed) != "" {
				args["content"] = trimmed
			}
		}
	}
	return Action{Kind: KindTool, Tool: name, Args: args}, true
}

func retry(notice string) Action {
	return Action{Kind: KindRetry, Notice: notice}
}

var attrPattern = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

func parseAttrs(text string) map[string]string {
	attrs := map[string]string{}
	for _, match := range attrPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 4 {
			continue
		}
		if match[2] != "" {
			attrs[match[1]] = match[2]
			continue
		}
		attrs[match[1]] = match[3]
	}
	return attrs
}

// attributeText 取 <tool ...> 里除标签名之外的部分。
func attributeText(text string) string {
	start := strings.Index(text, "<tool")
	if start < 0 {
		return ""
	}
	rest := text[start+len("<tool"):]
	end := strings.Index(rest, ">")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

// xmlBody 取 XML 形式 <tool ...> 与 </tool> 之间的内容。
func xmlBody(text string) string {
	start := strings.Index(text, "<tool")
	if start < 0 {
		return ""
	}
	rest := text[start:]
	open := strings.Index(rest, ">")
	if open < 0 {
		return ""
	}
	body := rest[open+1:]
	if end := strings.Index(body, "</tool>"); end >= 0 {
		return body[:end]
	}
	return body
}

func extract(text, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	start := strings.Index(text, startTag)
	if start < 0 {
		return text
	}
	start += len(startTag)
	end := strings.Index(text[start:], endTag)
	if end < 0 {
		return strings.TrimSpace(text[start:])
	}
	return strings.TrimSpace(text[start : start+end])
}

func extractRaw(text, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	start := strings.Index(text, startTag)
	if start < 0 {
		return text
	}
	start += len(startTag)
	end := strings.Index(text[start:], endTag)
	if end < 0 {
		return text[start:]
	}
	return text[start : start+end]
}

func firstIndex(text, needle string) (int, bool) {
	idx := strings.Index(text, needle)
	return idx, idx >= 0
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

// ToolSpec 是提示词里描述一个工具所需的信息。
type ToolSpec struct {
	Name        string
	Description string
	Params      []Param
	Risky       bool
}

// Param 描述工具的一个参数。
type Param struct {
	Name string
	Type string
}

// Workspace 是拼进前缀的仓库信息。
type Workspace struct {
	Root          string
	Files         []string
	Branch        string
	Dirty         bool
	TrackedFiles  int
	RecentCommits []string
	ProjectDocs   []string
}

// SortTools 按名称排序，保证提示词前缀稳定。
func SortTools(tools []ToolSpec) []ToolSpec {
	out := append([]ToolSpec(nil), tools...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// BuildPrefix 构建提示词前缀：角色、规则、工具清单、示例与仓库信息。
func BuildPrefix(workspace Workspace, tools []ToolSpec) string {
	var builder strings.Builder
	builder.WriteString("You are CodeMate, a small local coding agent working inside a local repository.\n\n")
	builder.WriteString("Rules:\n")
	builder.WriteString("- Use tools instead of guessing about the workspace.\n")
	builder.WriteString("- Return exactly one <tool>...</tool> or one <final>...</final>.\n")
	builder.WriteString(`- Tool calls must look like: <tool>{"name":"tool_name","args":{...}}</tool>` + "\n")
	builder.WriteString(`- For multi-line file content prefer XML style: <tool name="write_file" path="a.go"><content>...</content></tool>` + "\n")
	builder.WriteString("- Never invent tool results.\n")
	builder.WriteString("- Do not repeat a tool call with the same arguments if it did not help.\n")
	builder.WriteString("- Required tool arguments must not be empty.\n\n")

	builder.WriteString("Tools:\n")
	for _, tool := range SortTools(tools) {
		builder.WriteString("- " + tool.Name + "(")
		parts := make([]string, 0, len(tool.Params))
		for _, param := range tool.Params {
			parts = append(parts, param.Name+": "+param.Type)
		}
		builder.WriteString(strings.Join(parts, ", "))
		builder.WriteString(")")
		if tool.Risky {
			builder.WriteString(" [approval required]")
		} else {
			builder.WriteString(" [safe]")
		}
		builder.WriteString(" " + tool.Description + "\n")
	}

	builder.WriteString("\nValid response examples:\n")
	builder.WriteString(`<tool>{"name":"list_files","args":{"path":"."}}</tool>` + "\n")
	builder.WriteString(`<tool>{"name":"read_file","args":{"path":"README.md"}}</tool>` + "\n")
	builder.WriteString(`<tool name="write_file" path="a.go"><content>package main</content></tool>` + "\n")
	builder.WriteString("<final>Done.</final>\n")

	builder.WriteString("\nWorkspace:\n")
	builder.WriteString("root: " + workspace.Root + "\n")
	if workspace.Branch != "" {
		state := "clean"
		if workspace.Dirty {
			state = "has uncommitted changes"
		}
		builder.WriteString("git branch: " + workspace.Branch + " (" + state + ")\n")
	}
	for _, commit := range workspace.RecentCommits {
		builder.WriteString("recent commit: " + commit + "\n")
	}
	if len(workspace.ProjectDocs) > 0 {
		builder.WriteString("project docs: " + strings.Join(workspace.ProjectDocs, ", ") + "\n")
	}
	if len(workspace.Files) > 0 {
		builder.WriteString("top-level entries: " + strings.Join(workspace.Files, ", ") + "\n")
	}
	return strings.TrimSpace(builder.String())
}
