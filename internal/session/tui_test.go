package session

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/model"
	"github.com/yvluoxiashizui/codemate/internal/permissions"
	"github.com/yvluoxiashizui/codemate/internal/tools"
	"github.com/yvluoxiashizui/codemate/internal/tui"
)

var tuiAnsiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderTUIScreenIncludesTranscriptAndPrompt(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	s := New(Args{
		CWD:     dir,
		Tools:   tools.NewRegistry(nil, tools.Metadata{}),
		Model:   model.Mock{},
		Entries: nil,
		Out:     &out,
	})
	screen := s.renderTUIScreen(tuiState{
		input:  "hello",
		cursor: 5,
		transcript: []transcriptEntry{
			{kind: "user", body: "hello"},
			{kind: "assistant", body: "world"},
		},
	}, 80, 30)
	plain := strings.ReplaceAll(tuiAnsiPattern.ReplaceAllString(screen, ""), " ", "")
	for _, want := range []string{"MiniCode", "sessionfeed", "you", "assistant", "mini-code>hello|"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("screen missing %q:\n%s", want, screen)
		}
	}
}

func TestTUIHistoryNavigation(t *testing.T) {
	s := New(Args{Tools: tools.NewRegistry(nil, tools.Metadata{}), Model: model.Mock{}})
	state := tuiState{history: []string{"first", "second"}, historyIndex: 2}

	if _, err := s.handleTUIEvent(context.Background(), &state, tui.InputEvent{Kind: tui.EventKey, Name: tui.KeyUp}); err != nil {
		t.Fatal(err)
	}
	if state.input != "second" || state.cursor != len("second") {
		t.Fatalf("unexpected history up state: %#v", state)
	}

	if _, err := s.handleTUIEvent(context.Background(), &state, tui.InputEvent{Kind: tui.EventKey, Name: tui.KeyDown}); err != nil {
		t.Fatal(err)
	}
	if state.input != "" || state.cursor != 0 {
		t.Fatalf("unexpected history down state: %#v", state)
	}
}

func TestRenderTUIScreenCollapsesSuccessfulToolOutputToSummary(t *testing.T) {
	s := New(Args{CWD: t.TempDir(), Tools: tools.NewRegistry(nil, tools.Metadata{}), Model: model.Mock{}})
	screen := s.renderTUIScreen(tuiState{
		transcript: []transcriptEntry{
			{kind: "tool", toolName: "read_file", status: "success", body: "FILE: main.go\nOFFSET: 0\nEND: 10\nTOTAL_CHARS: 100\nTRUNCATED: yes - call read_file again with offset 10\n\npackage main"},
		},
	}, 100, 30)
	if !strings.Contains(screen, "read_file completed: FILE: main.go") {
		t.Fatalf("expected collapsed tool summary:\n%s", screen)
	}
	if strings.Contains(screen, "package main") {
		t.Fatalf("expected successful tool body to be collapsed:\n%s", screen)
	}
}

func TestFinishToolResultAppendsPatchEntry(t *testing.T) {
	state := &tuiState{}
	pending := map[string][]int{}

	finishToolResult(state, pending, "patch_file", "Patched main.go", false)

	if len(state.transcript) != 1 {
		t.Fatalf("应记录一条工具结果: %#v", state.transcript)
	}
	if state.transcript[0].toolName != "patch_file" {
		t.Fatalf("工具名 = %s", state.transcript[0].toolName)
	}
}
func TestFinishToolResultDoesNotCollapseErrors(t *testing.T) {
	state := tuiState{}
	index := appendToolStart(&state, "run_shell", map[string]any{"command": "go", "args": []any{"test", "./..."}})
	finishToolResult(&state, map[string][]int{"run_shell": []int{index}}, "run_shell", "compiler error\nline 2", true)
	screen := New(Args{CWD: t.TempDir(), Tools: tools.NewRegistry(nil, tools.Metadata{}), Model: model.Mock{}}).renderTUIScreen(state, 100, 30)
	if !strings.Contains(screen, "compiler error") || !strings.Contains(screen, "line 2") {
		t.Fatalf("expected error output to stay expanded:\n%s", screen)
	}
}

func TestParseTerminalSize(t *testing.T) {
	width, height := parseTerminalSize("40 120\n")
	if width != 120 || height != 40 {
		t.Fatalf("got width=%d height=%d", width, height)
	}
}

func TestRenderTUIScreenShowsApprovalPanel(t *testing.T) {
	s := New(Args{CWD: t.TempDir(), Tools: tools.NewRegistry(nil, tools.Metadata{}), Model: model.Mock{}})
	screen := s.renderTUIScreen(tuiState{
		pendingApproval: &approvalState{
			request: permissions.Request{
				Kind:    permissions.KindEdit,
				Summary: "mini-code wants to apply a file modification",
				Details: []string{"target: file.txt", "", "--- a/file.txt\n+++ b/file.txt"},
				Choices: []permissions.Choice{
					{Label: "apply once", Decision: permissions.DecisionAllowOnce},
					{Label: "reject once", Decision: permissions.DecisionDenyOnce},
				},
			},
		},
	}, 80, 30)
	for _, want := range []string{"approval", "Approval Required", "apply once", "reject once"} {
		if !strings.Contains(screen, want) {
			t.Fatalf("approval screen missing %q:\n%s", want, screen)
		}
	}
}

func TestRenderApprovalPromptHighlightsUnifiedDiff(t *testing.T) {
	out := renderApprovalPrompt(approvalState{
		request: permissions.Request{
			Summary: "edit file",
			Details: []string{"target: file.txt", "--- a/file.txt\n+++ b/file.txt\n@@ -1 +1 @@\n-oldName\n+newName"},
			Choices: []permissions.Choice{{Label: "allow", Decision: permissions.DecisionAllowOnce}},
		},
	}, 30)
	for _, want := range []string{"\x1b[36m--- a/file.txt", "\x1b[35m@@ -1 +1 @@", "\x1b[1mold", "\x1b[1mnew"} {
		if !strings.Contains(out, want) {
			t.Fatalf("approval prompt missing highlighted diff %q:\n%q", want, out)
		}
	}
}

func TestApprovalSelectionAndFeedback(t *testing.T) {
	state := approvalState{request: permissions.Request{Choices: []permissions.Choice{
		{Label: "allow", Decision: permissions.DecisionAllowOnce},
		{Label: "reject with guidance", Decision: permissions.DecisionDenyWithFeedback},
	}}}

	state.handle(tui.InputEvent{Kind: tui.EventKey, Name: tui.KeyDown})
	if state.selected != 1 {
		t.Fatalf("expected selected=1 got %d", state.selected)
	}
	done, result := state.handle(tui.InputEvent{Kind: tui.EventKey, Name: tui.KeyReturn})
	if done || !state.feedbackMode || result.Decision != "" {
		t.Fatalf("expected feedback mode, done=%v state=%#v result=%#v", done, state, result)
	}
	state.handle(tui.InputEvent{Kind: tui.EventText, Text: "n"})
	state.handle(tui.InputEvent{Kind: tui.EventText, Text: "o"})
	done, result = state.handle(tui.InputEvent{Kind: tui.EventKey, Name: tui.KeyReturn})
	if !done || result.Decision != permissions.DecisionDenyWithFeedback || result.Feedback != "no" {
		t.Fatalf("unexpected feedback result done=%v result=%#v", done, result)
	}
}
