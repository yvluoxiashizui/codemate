package message

import "testing"

func TestMessageConstructors(t *testing.T) {
	assistant := AssistantMessage("hello")
	if assistant.Role != RoleAssistant || assistant.Content != "hello" {
		t.Fatalf("unexpected assistant message: %#v", assistant)
	}

	progress := ProgressMessage("working")
	if progress.Role != RoleAssistantProgress || progress.Content != "working" {
		t.Fatalf("unexpected progress message: %#v", progress)
	}
}
