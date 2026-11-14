package compact

import (
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/token"
)

func userTurn(text string) []message.Message {
	return []message.Message{
		message.UserMessage(text),
		message.AssistantMessage("ok"),
	}
}

func TestShouldTriggersNearWindow(t *testing.T) {
	nearWindow := token.NewBudget(9000, 1000, 12000)
	if !Should(nearWindow, 1000) {
		t.Fatal("剩余预算低于阈值时应触发压缩")
	}
	roomy := token.NewBudget(1000, 100, 100000)
	if Should(roomy, 1000) {
		t.Fatal("预算充足时不应触发压缩")
	}
}

func TestShouldSkipsWhenHistoryShorterThanKeep(t *testing.T) {
	tight := token.NewBudget(900, 100, 1000)
	if Should(tight, 5000) {
		t.Fatal("历史本身短于保留预算时不应压缩")
	}
}

func TestSplitKeepsSystemOutOfHead(t *testing.T) {
	messages := append([]message.Message{message.SystemMessage("system")}, userTurn("a")...)
	messages = append(messages, userTurn("b")...)

	head, tail := Split(messages, 6)
	if len(head) == 0 {
		t.Fatal("应有一部分消息进入可压缩前缀")
	}
	for _, item := range head {
		if item.Role == message.RoleSystem {
			t.Fatal("系统消息不应进入可压缩前缀")
		}
	}
	if len(tail) == 0 {
		t.Fatal("应保留近期消息")
	}
	if got := len(head) + len(tail); got != len(messages)-1 {
		t.Fatalf("切分后消息数 = %d, want %d", got, len(messages)-1)
	}
}

func TestSplitDoesNotSeparateToolCallFromResult(t *testing.T) {
	messages := []message.Message{
		message.SystemMessage("system"),
		message.UserMessage("a"),
		message.AssistantMessage("ok"),
		message.AssistantToolCallMessage(message.ToolCall{ID: "1", ToolName: "read_file"}),
		message.ToolResultMessage("1", "read_file", "content", false),
		message.UserMessage("b"),
	}

	head, tail := Split(messages, 12)
	if len(tail) == 0 || len(head) == 0 {
		t.Fatalf("切分结果异常: head=%d tail=%d", len(head), len(tail))
	}
	if head[len(head)-1].Role == message.RoleAssistantToolCall && tail[0].Role == message.RoleToolResult {
		t.Fatal("工具调用与结果被切开了")
	}
	if head[len(head)-1].Role == message.RoleToolResult && tail[0].Role == message.RoleAssistantToolCall {
		t.Fatal("工具结果与调用被切开了")
	}
}

func TestSplitEmptyInput(t *testing.T) {
	head, tail := Split(nil, 100)
	if head != nil || tail != nil {
		t.Fatal("空输入应返回空结果")
	}
	head, tail = Split([]message.Message{message.SystemMessage("only system")}, 100)
	if head != nil || tail != nil {
		t.Fatal("只有系统消息时无需压缩")
	}
}

func TestBuildRequestCarriesSystemAndTranscript(t *testing.T) {
	request := BuildRequest(message.SystemMessage("sys"), userTurn("hello"), 0)
	if len(request) != 2 {
		t.Fatalf("请求消息数 = %d, want 2", len(request))
	}
	if request[0].Role != message.RoleSystem || request[0].Content != "sys" {
		t.Fatalf("系统消息丢失: %#v", request[0])
	}
	if request[1].Role != message.RoleUser {
		t.Fatalf("摘要请求应为 user 消息: %#v", request[1])
	}
	if !contains(request[1].Content, "hello") {
		t.Fatal("摘要请求应包含被压缩的原文")
	}
}

func TestApplyRebuildsContext(t *testing.T) {
	tail := userTurn("recent")
	out := Apply(message.SystemMessage("sys"), "summary text", tail)
	if len(out) != len(tail)+2 {
		t.Fatalf("压缩后消息数 = %d, want %d", len(out), len(tail)+2)
	}
	if out[0].Role != message.RoleSystem {
		t.Fatal("系统消息应在最前")
	}
	if !contains(out[1].Content, "summary text") {
		t.Fatalf("摘要未注入: %#v", out[1])
	}
}

func TestApplyWithoutSystem(t *testing.T) {
	out := Apply(message.Message{}, "summary", userTurn("recent"))
	if len(out) != 3 {
		t.Fatalf("无系统消息时应只有摘要与尾部: %d", len(out))
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
