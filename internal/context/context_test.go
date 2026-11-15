package context

import (
	"strings"
	"testing"
)

func sampleSections() Sections {
	return Sections{
		Prefix:         strings.Repeat("p", 3000),
		Memory:         strings.Repeat("m", 1200),
		Relevant:       strings.Repeat("r", 900),
		History:        strings.Repeat("h", 4000),
		CurrentRequest: "修复失败的测试",
	}
}

func TestBuildUnderBudgetKeepsEverything(t *testing.T) {
	result := Build(sampleSections(), Options{TotalBudget: 100000})
	if len(result.Reductions) != 0 {
		t.Fatalf("预算充足时不应收缩: %#v", result.Reductions)
	}
	if !strings.Contains(result.Prompt, "修复失败的测试") {
		t.Fatal("当前请求必须出现在 prompt 里")
	}
	if result.Sections[SectionHistory].Rendered != 4000 {
		t.Fatalf("历史不应被裁剪: %d", result.Sections[SectionHistory].Rendered)
	}
}

func TestBuildReducesInConfiguredOrder(t *testing.T) {
	if len(DefaultReductionOrder) != 4 || DefaultReductionOrder[0] != SectionRelevant {
		t.Fatalf("默认收缩顺序异常: %#v", DefaultReductionOrder)
	}

	result := Build(sampleSections(), Options{TotalBudget: 8000})
	if len(result.Reductions) == 0 {
		t.Fatal("超预算时应产生收缩记录")
	}
	if result.Reductions[0].Section != SectionRelevant {
		t.Fatalf("第一次收缩应针对相关笔记, got %s", result.Reductions[0].Section)
	}
	if len(result.Prompt) > 8000 {
		t.Fatalf("拼装结果仍超预算: %d", len(result.Prompt))
	}
}

func TestBuildStopsAtFloors(t *testing.T) {
	result := Build(sampleSections(), Options{TotalBudget: 100})
	if got := result.Sections[SectionRelevant].Budget; got < DefaultFloors[SectionRelevant] {
		t.Fatalf("相关笔记预算跌破下限: %d", got)
	}
	if got := result.Sections[SectionHistory].Budget; got < DefaultFloors[SectionHistory] {
		t.Fatalf("历史预算跌破下限: %d", got)
	}
	if got := result.Sections[SectionPrefix].Budget; got < DefaultFloors[SectionPrefix] {
		t.Fatalf("前缀预算跌破下限: %d", got)
	}
}

func TestBuildNeverClipsCurrentRequest(t *testing.T) {
	request := strings.Repeat("请", 500)
	sections := sampleSections()
	sections.CurrentRequest = request

	result := Build(sections, Options{TotalBudget: 500})
	if !strings.Contains(result.Prompt, request) {
		t.Fatal("当前请求不应被裁剪")
	}
	if result.Sections[SectionRequest].Rendered != len(request) {
		t.Fatalf("当前请求长度被改动: %d", result.Sections[SectionRequest].Rendered)
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	first := Build(sampleSections(), Options{TotalBudget: 7000})
	second := Build(sampleSections(), Options{TotalBudget: 7000})
	if first.Prompt != second.Prompt {
		t.Fatal("同一输入两次拼装结果应完全一致")
	}
	if len(first.Reductions) != len(second.Reductions) {
		t.Fatal("收缩记录应一致")
	}
	for i := range first.Reductions {
		if first.Reductions[i] != second.Reductions[i] {
			t.Fatalf("第 %d 次收缩记录不一致", i)
		}
	}
}

func TestBuildKeepsPrefixHead(t *testing.T) {
	sections := Sections{Prefix: "规则A规则B" + strings.Repeat("x", 100), CurrentRequest: "q"}
	result := Build(sections, Options{
		TotalBudget: 60,
		Budgets:     map[string]int{SectionPrefix: 20},
		Floors:      map[string]int{SectionPrefix: 20},
	})
	if !strings.HasPrefix(result.Prompt, "规则A") {
		t.Fatalf("前缀应保留开头: %q", result.Prompt)
	}
	if !strings.HasSuffix(strings.Split(result.Prompt, "Current user request:")[0], "...\n") &&
		!strings.Contains(result.Prompt, "...") {
		t.Fatal("被裁剪处应有省略标记")
	}
}

func TestBuildKeepsHistoryTail(t *testing.T) {
	sections := Sections{History: "旧的" + strings.Repeat("x", 100) + "最新的", CurrentRequest: "q"}
	result := Build(sections, Options{
		TotalBudget: 80,
		Budgets:     map[string]int{SectionHistory: 30},
		Floors:      map[string]int{SectionHistory: 30},
	})
	if !strings.Contains(result.Prompt, "最新的") {
		t.Fatalf("历史应保留结尾: %q", result.Prompt)
	}
}

func TestBuildSkipsEmptySections(t *testing.T) {
	result := Build(Sections{CurrentRequest: "只问一句"}, Options{TotalBudget: 1000})
	if strings.Contains(result.Prompt, "History:") {
		t.Fatal("空节不应输出标题")
	}
	if result.Prompt != "Current user request:\n只问一句" {
		t.Fatalf("prompt = %q", result.Prompt)
	}
}
