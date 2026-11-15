package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

type Result struct {
	OK     bool
	Output string
	Meta   Meta
}

func Success(output string) Result {
	return Result{OK: true, Output: output}
}

func Error(output string) Result {
	return Result{OK: false, Output: output}
}

type PermissionManager interface {
	EnsurePathAccess(ctx context.Context, targetPath, intent string) error
	EnsureCommand(ctx context.Context, command string, args []string, cwd string) error
	EnsureEdit(ctx context.Context, targetPath, diffPreview string) error
}

// Policy 是一次运行的工具策略。
type Policy struct {
	// ReadOnly 为真时只允许执行只读工具。
	ReadOnly bool
	// Allowed 非空时只允许执行列出的工具。
	Allowed []string
}

// Permits 判断工具名是否在本次运行允许的范围内。
func (p Policy) Permits(name string) bool {
	if p.Allowed == nil {
		return true
	}
	for _, item := range p.Allowed {
		if item == name {
			return true
		}
	}
	return false
}

type Context struct {
	CWD        string
	Permission PermissionManager
	Policy     Policy
	affected   *[]string
}

// ReportAffected 记录本次调用改动或读取的文件路径。
func (c Context) ReportAffected(path string) {
	if c.affected == nil || path == "" {
		return
	}
	for _, existing := range *c.affected {
		if existing == path {
			return
		}
	}
	*c.affected = append(*c.affected, path)
}

type Definition struct {
	Name        string
	Description string
	InputSchema map[string]any
	Risk        RiskLevel
	ReadOnly    bool
	// Validate 在执行前做语义校验，参数语义不成立时返回错误。
	Validate func(json.RawMessage, Context) error
	Run      func(context.Context, json.RawMessage, Context) Result
}

type SkillSummary struct {
	Name        string
	Description string
	Path        string
	Source      string
}

type MCPServerSummary struct {
	Name          string
	Command       string
	Status        string
	ToolCount     int
	Error         string
	Protocol      string
	ResourceCount int
	PromptCount   int
}

type Metadata struct {
	Skills     []SkillSummary
	MCPServers []MCPServerSummary
}

type Registry struct {
	tools    []Definition
	byName   map[string]Definition
	metadata Metadata
	dispose  func(context.Context) error
}

func NewRegistry(definitions []Definition, metadata Metadata) *Registry {
	byName := map[string]Definition{}
	for _, definition := range definitions {
		byName[definition.Name] = definition
	}
	return &Registry{tools: definitions, byName: byName, metadata: metadata}
}

func (r *Registry) WithDisposer(dispose func(context.Context) error) *Registry {
	r.dispose = dispose
	return r
}

func (r *Registry) List() []Definition {
	out := make([]Definition, len(r.tools))
	copy(out, r.tools)
	return out
}

func (r *Registry) Skills() []SkillSummary {
	out := make([]SkillSummary, len(r.metadata.Skills))
	copy(out, r.metadata.Skills)
	return out
}

func (r *Registry) MCPServers() []MCPServerSummary {
	out := make([]MCPServerSummary, len(r.metadata.MCPServers))
	copy(out, r.metadata.MCPServers)
	return out
}

func (r *Registry) Find(name string) (Definition, bool) {
	definition, ok := r.byName[name]
	return definition, ok
}

func (r *Registry) Execute(ctx context.Context, toolName string, input any, toolContext Context) (result Result) {
	definition, ok := r.Find(toolName)
	if !ok {
		return Result{
			OK:     false,
			Output: "Unknown tool: " + toolName,
			Meta:   Meta{Status: StatusReject, ErrorCode: CodeUnknownTool, RiskLevel: RiskHigh},
		}
	}

	if !toolContext.Policy.Permits(toolName) {
		return Result{
			OK:     false,
			Output: "Tool " + toolName + " is not allowed in this run",
			Meta:   Meta{Status: StatusReject, ErrorCode: CodeToolNotAllowed, RiskLevel: RiskHigh},
		}
	}
	if toolContext.Policy.ReadOnly && !definition.ReadOnly {
		return Result{
			OK:     false,
			Output: "Tool " + toolName + " is blocked: this run is read-only",
			Meta:   Meta{Status: StatusReject, ErrorCode: CodeReadOnly, RiskLevel: definition.Risk, ReadOnly: true},
		}
	}

	affected := make([]string, 0, 2)
	toolContext.affected = &affected

	meta := Meta{
		Status:    StatusOK,
		RiskLevel: definition.Risk,
		ReadOnly:  definition.ReadOnly,
	}
	if meta.RiskLevel == "" {
		meta.RiskLevel = RiskLow
	}

	raw, err := normalizeInput(input)
	if err != nil {
		return finishWithError(toolName, meta, affected, CodeInvalidInput, err.Error())
	}
	if err := validateInputSchema(definition.InputSchema, raw); err != nil {
		return finishWithError(toolName, meta, affected, CodeInvalidInput, "Invalid input for "+toolName+": "+err.Error())
	}
	if definition.Validate != nil {
		if err := definition.Validate(raw, toolContext); err != nil {
			return finishWithError(toolName, meta, affected, CodeInvalidInput, "Invalid input for "+toolName+": "+err.Error())
		}
	}

	if definition.Run == nil {
		return finishWithError(toolName, meta, affected, CodeToolError, "Tool has no runner: "+toolName)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			result = finishWithError(toolName, meta, affected, CodeToolPanic, fmt.Sprintf("Tool %s panicked: %v", toolName, recovered))
		}
	}()

	result = definition.Run(ctx, raw, toolContext)
	result.Meta = meta
	result.Meta.AffectedPaths = affected
	result.Meta.WorkspaceChanged = !definition.ReadOnly && len(affected) > 0
	if !result.OK {
		result.Meta.Status = StatusError
		result.Meta.ErrorCode = CodeToolError
	}
	return result
}

// finishWithError 统一构造失败结果。
func finishWithError(toolName string, meta Meta, affected []string, code, message string) Result {
	meta.Status = StatusError
	meta.ErrorCode = code
	meta.AffectedPaths = affected
	return Result{OK: false, Output: message, Meta: meta}
}

func (r *Registry) Dispose(ctx context.Context) error {
	if r.dispose == nil {
		return nil
	}
	return r.dispose(ctx)
}

func normalizeInput(input any) (json.RawMessage, error) {
	if input == nil {
		return json.RawMessage(`{}`), nil
	}
	if raw, ok := input.(json.RawMessage); ok {
		return raw, nil
	}
	bytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("invalid tool input: %w", err)
	}
	return bytes, nil
}

func (r *Registry) Schema() []map[string]any {
	out := make([]map[string]any, 0, len(r.tools))
	for _, definition := range r.tools {
		out = append(out, map[string]any{
			"name":         definition.Name,
			"description":  definition.Description,
			"input_schema": definition.InputSchema,
		})
	}
	return out
}
