package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yvluoxiashizui/codemate/internal/config"
	"github.com/yvluoxiashizui/codemate/internal/eval"
	"github.com/yvluoxiashizui/codemate/internal/message"
	"github.com/yvluoxiashizui/codemate/internal/model"
	"github.com/yvluoxiashizui/codemate/internal/tools"
)

func main() {
	limit := flag.Int("limit", 0, "只跑前 N 条任务，0 表示全部")
	out := flag.String("out", "", "把结果写成 JSON 的路径")
	cwd := flag.String("cwd", "", "读取模型配置的目录，默认当前目录")
	benchmarkPath := flag.String("benchmark", "", "任务集 JSON 路径，留空时用内置任务集")
	ablate := flag.String("ablate", "", "关掉指定开关做消融对照，取值 memory、relevant_memory、context_reduction、prompt_cache")
	stress := flag.Bool("stress", false, "测量不同历史长度下的上下文装配")
	flag.Parse()

	workdir := *cwd
	if workdir == "" {
		workdir, _ = os.Getwd()
	}

	var adapter message.Completer
	if os.Getenv("MINI_CODE_MODEL_MODE") == "mock" {
		adapter = model.Mock{}
	} else {
		runtime, err := config.LoadRuntime(workdir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "模型配置不可用:", err)
			os.Exit(1)
		}
		adapter, err = model.NewFromRuntime(runtime, tools.NewRegistry(nil, tools.Metadata{}))
		if err != nil {
			fmt.Fprintln(os.Stderr, "构造模型失败:", err)
			os.Exit(1)
		}
	}

	cases := eval.DefaultCases()
	if *benchmarkPath != "" {
		benchmark, err := eval.LoadBenchmark(*benchmarkPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "任务集不可用:", err)
			os.Exit(1)
		}
		cases = benchmark.Cases()
	}
	if *limit > 0 && *limit < len(cases) {
		cases = cases[:*limit]
	}

	runner := eval.Runner{
		Model:    adapter,
		MaxSteps: 24,
		OnResult: func(result eval.Result) {
			mark := "PASS"
			if !result.Passed {
				mark = "FAIL"
			}
			fmt.Printf("%s  %-16s attempts=%d steps=%d prompt=%d %s\n",
				mark, result.Case, result.Attempts, result.ToolSteps, result.PromptChars, result.Reason)
		},
	}
	if *ablate != "" {
		report := runner.Ablate(context.Background(), cases, *ablate)
		fmt.Printf("消融 %s\n", report.Feature)
		fmt.Printf("基线   通过率 %.1f%%  prompt %.0f 字符\n", report.Baseline.PassRate*100, report.Baseline.AvgPromptChars)
		fmt.Printf("已关闭 通过率 %.1f%%  prompt %.0f 字符\n", report.Disabled.PassRate*100, report.Disabled.AvgPromptChars)
		fmt.Printf("差值   prompt %+.0f 字符  通过率 %+.1f 个百分点\n", report.PromptCharsDelta(), report.PassRateDelta()*100)
		return
	}

	if *stress {
		points := eval.MeasureStress("prefix", []int{0, 4, 12, 24, 48}, 12000)
		fmt.Printf("%-8s %-12s %-10s %s\n", "历史条数", "prompt", "收缩次数", "超预算")
		for _, point := range points {
			over := "no"
			if point.OverBudget {
				over = "yes"
			}
			fmt.Printf("%-8d %-12d %-10d %s\n", point.Entries, point.PromptChars, point.Reductions, over)
		}
		return
	}

	results := runner.RunCases(context.Background(), cases)
	summary := eval.Summarize(results)

	fmt.Printf("\n通过 %d/%d  通过率 %.1f%%  平均请求 %.1f  平均工具步数 %.1f  平均 prompt %.0f 字符  工具失败率 %.1f%%\n",
		summary.Passed, summary.Total, summary.PassRate*100,
		summary.AvgAttempts, summary.AvgToolSteps, summary.AvgPromptChars,
		summary.ToolFailureRate*100)

	if *out != "" {
		if err := writeReport(*out, results, summary); err != nil {
			fmt.Fprintln(os.Stderr, "写报告失败:", err)
			os.Exit(1)
		}
	}
}

func writeReport(path string, results []eval.Result, summary eval.Summary) error {
	payload := map[string]any{
		"summary": summary,
		"results": results,
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}
