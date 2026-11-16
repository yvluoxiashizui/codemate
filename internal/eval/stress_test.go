package eval

import "testing"

func TestMeasureStressGrowsWithHistory(t *testing.T) {
	points := MeasureStress("prefix", []int{2, 10, 40}, 100000)
	if len(points) != 3 {
		t.Fatalf("取样点 = %d", len(points))
	}
	for i := 1; i < len(points); i++ {
		if points[i].PromptChars < points[i-1].PromptChars {
			t.Fatalf("历史变长后 prompt 不应变短: %#v", points)
		}
	}
	if points[2].OverBudget {
		t.Fatal("预算充足时不应判超预算")
	}
}

func TestMeasureStressTriggersReductionUnderSmallBudget(t *testing.T) {
	points := MeasureStress("prefix", []int{40}, 600)

	point := points[0]
	if point.Reductions == 0 {
		t.Fatalf("小预算下应发生收缩: %#v", point)
	}
	if point.PromptChars == 0 {
		t.Fatal("应产出 prompt")
	}
}

func TestMeasureStressReportsOverBudget(t *testing.T) {
	// 前缀本身超过预算且当前请求不参与收缩时，总量仍可能超预算。
	longPrefix := ""
	for i := 0; i < 400; i++ {
		longPrefix += "前缀内容"
	}
	points := MeasureStress(longPrefix, []int{4}, 500)
	if !points[0].OverBudget {
		t.Fatalf("超预算应被标记: %#v", points[0])
	}
}

func TestSyntheticHistorySize(t *testing.T) {
	if got := len(syntheticHistory(7)); got != 7 {
		t.Fatalf("历史条数 = %d", got)
	}
}
