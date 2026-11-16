package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// DefaultDelegateSteps 是子 agent 默认的步数上限。
const DefaultDelegateSteps = 3

// delegateTool 把一个调查任务交给只读的子 agent。
func delegateTool() Definition {
	return Definition{
		Name:        "delegate",
		Description: "Hand a read-only investigation to a child agent and return its conclusion.",
		Risk:        RiskLow,
		ReadOnly:    true,
		InputSchema: objectSchema(map[string]any{
			"task":      map[string]any{"type": "string"},
			"max_steps": map[string]any{"type": "number"},
		}, []string{"task"}),
		Validate: validateDelegate,
		Run: func(ctx context.Context, raw json.RawMessage, tc Context) Result {
			var input struct {
				Task     string `json:"task"`
				MaxSteps int    `json:"max_steps"`
			}
			if err := json.Unmarshal(raw, &input); err != nil {
				return Error(err.Error())
			}
			if tc.Spawn == nil {
				return Error("delegate is unavailable in this run")
			}
			steps := input.MaxSteps
			if steps <= 0 {
				steps = DefaultDelegateSteps
			}
			output, err := tc.Spawn(ctx, strings.TrimSpace(input.Task), steps)
			if err != nil {
				return Error(err.Error())
			}
			return Success(output)
		},
	}
}

// validateDelegate 校验任务非空且没有超过嵌套深度。
func validateDelegate(raw json.RawMessage, tc Context) error {
	var input struct {
		Task string `json:"task"`
	}
	if err := json.Unmarshal(raw, &input); err != nil {
		return err
	}
	if strings.TrimSpace(input.Task) == "" {
		return errors.New("task must not be empty")
	}
	return nil
}
