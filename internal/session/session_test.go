package session

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yvluoxiashizui/codemate/internal/config"
	ctxpkg "github.com/yvluoxiashizui/codemate/internal/context"
	"github.com/yvluoxiashizui/codemate/internal/model"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

type scriptedCompleter struct {
	responses []string
	index     int
	err       error
}

func (s *scriptedCompleter) Complete(_ context.Context, _ string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if s.index >= len(s.responses) {
		return "<final>默认答案</final>", nil
	}
	out := s.responses[s.index]
	s.index++
	return out, nil
}

func TestRunOnceExecutesShortcut(t *testing.T) {
	dir := t.TempDir()
	registry := tools.Builtins(dir, nil, nil)
	var out bytes.Buffer
	s := New(Args{
		CWD:   dir,
		Tools: registry,
		Model: model.Mock{},
		Out:   &out,
	})
	if err := s.RunOnce(context.Background(), "/ls"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "(empty)") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunOnceHandlesRuntimeSlashCommands(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	registry := tools.NewRegistry(nil, tools.Metadata{
		Skills: []tools.SkillSummary{{Name: "demo", Description: "示例技能", Path: "skills/demo", Source: "project"}},
	})
	s := New(Args{
		CWD:   dir,
		Tools: registry,
		Model: model.Mock{},
		Out:   &out,
	})
	if err := s.RunOnce(context.Background(), "/skills"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "demo") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunOncePersistsHistoryEntries(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	store := Store{Dir: filepath.Join(dir, "sessions")}
	s := New(Args{
		CWD:       dir,
		Tools:     tools.NewRegistry(nil, tools.Metadata{}),
		Model:     &scriptedCompleter{responses: []string{"<final>改好了</final>"}},
		Store:     store,
		SessionID: "s1",
		Out:       &out,
	})

	if err := s.RunOnce(context.Background(), "修一下"); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load("s1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("存档条目数 = %d, want 2", len(loaded.Messages))
	}
	if loaded.Messages[0].Role != ctxpkg.RoleUser || loaded.Messages[0].Text != "修一下" {
		t.Fatalf("第一条应为用户请求: %#v", loaded.Messages[0])
	}
	if loaded.Messages[1].Role != ctxpkg.RoleAssistant || loaded.Messages[1].Text != "改好了" {
		t.Fatalf("第二条应为最终回答: %#v", loaded.Messages[1])
	}
}

func TestRunOnceReportsModelFailure(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	s := New(Args{
		CWD:   dir,
		Tools: tools.NewRegistry(nil, tools.Metadata{}),
		Model: &scriptedCompleter{err: errors.New("boom")},
		Out:   &out,
	})
	if err := s.RunOnce(context.Background(), "试一下"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "请求失败") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestRunOnceUsesResumedEntries(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	entries := []ctxpkg.HistoryEntry{{Role: ctxpkg.RoleUser, Text: "上一轮"}, {Role: ctxpkg.RoleAssistant, Text: "上一次回答"}}
	s := New(Args{
		CWD:     dir,
		Tools:   tools.NewRegistry(nil, tools.Metadata{}),
		Model:   &scriptedCompleter{responses: []string{"<final>继续</final>"}},
		Entries: entries,
		Out:     &out,
	})
	if err := s.RunOnce(context.Background(), "接着来"); err != nil {
		t.Fatal(err)
	}
	if len(s.args.Entries) != 4 {
		t.Fatalf("恢复后条目数 = %d, want 4", len(s.args.Entries))
	}
}

func TestRunOncePrintsFinalAnswer(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	s := New(Args{
		CWD:   dir,
		Tools: tools.NewRegistry(nil, tools.Metadata{}),
		Model: &scriptedCompleter{responses: []string{"<final>答案在这里</final>"}},
		Out:   &out,
	})
	if err := s.RunOnce(context.Background(), "问一句"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "答案在这里") {
		t.Fatalf("unexpected output: %q", out.String())
	}
}

func TestHistoryFilePersistsLastEntries(t *testing.T) {
	dir := t.TempDir()
	history := History{Path: filepath.Join(dir, "history.json")}
	if err := history.Save([]string{"第一条", "第二条"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := history.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 || loaded[1] != "第二条" {
		t.Fatalf("历史文件内容不符: %#v", loaded)
	}
}

var _ = config.Runtime{}
