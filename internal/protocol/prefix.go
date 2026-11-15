package protocol

import "github.com/yvluoxiashizui/codemate/internal/tools"

// riskyTools 是需要审批的工具集合，用于在提示词里标注。
var riskyTools = map[string]bool{
	"write_file":  true,
	"edit_file":   true,
	"patch_file":  true,
	"run_command": true,
}

// ToolSpecs 把注册表里的工具转成提示词描述。
func ToolSpecs(registry *tools.Registry) []ToolSpec {
	definitions := registry.List()
	specs := make([]ToolSpec, 0, len(definitions))
	for _, definition := range definitions {
		specs = append(specs, ToolSpec{
			Name:        definition.Name,
			Description: definition.Description,
			Params:      paramsOf(definition.InputSchema),
			Risky:       riskyTools[definition.Name],
		})
	}
	return specs
}

func paramsOf(schema map[string]any) []Param {
	if schema == nil {
		return nil
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil
	}
	params := make([]Param, 0, len(properties))
	for name, raw := range properties {
		paramType := "string"
		if field, ok := raw.(map[string]any); ok {
			if value, ok := field["type"].(string); ok {
				paramType = value
			}
		}
		params = append(params, Param{Name: name, Type: paramType})
	}
	return params
}
