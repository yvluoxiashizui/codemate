package eval

import "testing"

func TestDefaultCasesAreVerifiable(t *testing.T) {
	cases := DefaultCases()
	if len(cases) == 0 {
		t.Fatal("任务集不应为空")
	}
	for _, item := range cases {
		if item.Name == "" || item.Prompt == "" || item.Check == nil {
			t.Fatalf("任务定义不完整: %#v", item)
		}
	}
}

func TestDefaultCasesChecksFailOnEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	for _, item := range DefaultCases() {
		if err := item.Check(root); err == nil {
			t.Fatalf("空工作区不应通过: %s", item.Name)
		}
	}
}
