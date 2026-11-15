package permissions

import (
	"context"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	manager, err := New(t.TempDir(), "", nil)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return manager
}

func TestDefaultModeIsAsk(t *testing.T) {
	manager := newTestManager(t)
	if manager.Mode() != ModeAsk {
		t.Fatalf("默认策略 = %s", manager.Mode())
	}
}

func TestAutoModeAllowsWithoutPrompt(t *testing.T) {
	manager := newTestManager(t)
	manager.SetMode(ModeAuto)

	if err := manager.EnsureEdit(context.Background(), manager.workspaceRoot+"/a.go", "diff"); err != nil {
		t.Fatalf("auto 模式应放行: %v", err)
	}
}

func TestNeverModeRejectsWithoutPrompt(t *testing.T) {
	manager := newTestManager(t)
	manager.SetMode(ModeNever)

	err := manager.EnsureEdit(context.Background(), manager.workspaceRoot+"/a.go", "diff")
	if err == nil {
		t.Fatal("never 模式应拒绝")
	}
}

func TestAskModeWithoutPromptRejects(t *testing.T) {
	manager := newTestManager(t)

	if err := manager.EnsureEdit(context.Background(), manager.workspaceRoot+"/a.go", "diff"); err == nil {
		t.Fatal("没有交互终端时应拒绝并要求审批")
	}
}

func TestNeverModeStillHonorsExplicitDeny(t *testing.T) {
	manager := newTestManager(t)
	manager.SetMode(ModeAuto)
	target := manager.workspaceRoot + "/secret.txt"
	manager.sessionDeniedEdits[target] = true

	if err := manager.EnsureEdit(context.Background(), target, "diff"); err == nil {
		t.Fatal("硬性拒绝应优先于 auto 放行")
	}
}
