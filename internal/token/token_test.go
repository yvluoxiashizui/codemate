package token

import (
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
)

func TestEstimateText(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"空字符串", "", 0},
		{"四字符", "abcd", 1},
		{"五字符向上取整", "abcde", 2},
		{"中文按字符计", "代码智能体", 5},
		{"中英混排", "go 代码", 1 + 2},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			if got := EstimateText(item.text); got != item.want {
				t.Fatalf("EstimateText(%q) = %d, want %d", item.text, got, item.want)
			}
		})
	}
}

func TestEstimateMessagesCountsToolInput(t *testing.T) {
	plain := []message.Message{message.UserMessage("abcd")}
	withToolInput := []message.Message{{
		Role:     message.RoleAssistantToolCall,
		ToolName: "read_file",
		Input:    map[string]any{"path": "a.go"},
	}}

	plainTokens := EstimateMessages(plain)
	toolTokens := EstimateMessages(withToolInput)
	if toolTokens <= plainTokens {
		t.Fatalf("带工具入参的消息应更贵: plain=%d tool=%d", plainTokens, toolTokens)
	}
}

func TestEstimateMessagesGrowsWithLength(t *testing.T) {
	short := EstimateMessages([]message.Message{message.UserMessage("abcd")})
	long := EstimateMessages([]message.Message{
		message.UserMessage("abcd"),
		message.AssistantMessage("efgh"),
	})
	if long <= short {
		t.Fatalf("消息更多时应更大: short=%d long=%d", short, long)
	}
}

func TestNewBudgetDefaultsWindow(t *testing.T) {
	budget := NewBudget(100, 50, 0)
	if budget.Window != DefaultWindow {
		t.Fatalf("window = %d, want %d", budget.Window, DefaultWindow)
	}
	if budget.Total != 150 {
		t.Fatalf("total = %d, want 150", budget.Total)
	}
	if budget.Remaining != DefaultWindow-150 {
		t.Fatalf("remaining = %d", budget.Remaining)
	}
}

func TestNewBudgetClampsRemainingAtZero(t *testing.T) {
	budget := NewBudget(200, 100, 128)
	if budget.Remaining != 0 {
		t.Fatalf("remaining = %d, want 0", budget.Remaining)
	}
	if !budget.OverBudget() {
		t.Fatal("超出窗口应报告 OverBudget")
	}
}

func TestBudgetNotOverWhenFits(t *testing.T) {
	budget := NewBudget(10, 10, 100)
	if budget.OverBudget() {
		t.Fatal("未超出窗口不应报告 OverBudget")
	}
}
