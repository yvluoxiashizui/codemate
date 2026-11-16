package eval

import (
	"context"
	"strings"
	"testing"
)

func TestRegressWithoutBaselineRecordsOnly(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>ok</final>"}}
	runner := Runner{Model: model, MaxSteps: 2}

	regression := runner.Regress(context.Background(), []Case{longPrefixCase()}, Summary{}, 0.1)
	if !regression.Passed {
		t.Fatalf("无基线时应通过: %s", regression.Reason)
	}
	if regression.Current.Total != 1 {
		t.Fatalf("应记录本次结果: %#v", regression.Current)
	}
}

func TestRegressDetectsDrop(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>ok</final>"}}
	runner := Runner{Model: model, MaxSteps: 2}

	baseline := Summary{Total: 1, Passed: 1, PassRate: 1.0}
	failing := Case{
		Name:   "必须写文件",
		Prompt: "写 hello.txt",
		Check:  CheckFileContains("hello.txt", "hi"),
	}
	regression := runner.Regress(context.Background(), []Case{failing}, baseline, 0.1)

	if regression.Current.PassRate != 0 {
		t.Fatalf("任务应判定失败: %#v", regression.Current)
	}
	if regression.Passed {
		t.Fatal("通过率下降超过容忍度应判为回归")
	}
	if !strings.Contains(regression.Reason, "下降") {
		t.Fatalf("原因应说明下降: %s", regression.Reason)
	}
}

func TestRegressAcceptsSmallDrop(t *testing.T) {
	model := &scriptedModel{responses: []string{"<final>ok</final>"}}
	runner := Runner{Model: model, MaxSteps: 2}

	baseline := Summary{Total: 10, Passed: 9, PassRate: 0.9}
	regression := runner.Regress(context.Background(), nil, baseline, 1.0)

	if !regression.Passed {
		t.Fatalf("容忍度内应通过: %s", regression.Reason)
	}
}

func TestDigestCarriesSummaryFields(t *testing.T) {
	digest := Digest(Summary{Total: 5, Passed: 4, PassRate: 0.8, AvgAttempts: 2.5})
	if digest["total"] != 5 || digest["pass_rate"] != 0.8 {
		t.Fatalf("摘要字段不符: %#v", digest)
	}
}
