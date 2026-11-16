package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBenchmark(t *testing.T, benchmark Benchmark) string {
	t.Helper()
	payload, err := json.Marshal(benchmark)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "benchmark.json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func validBenchmark() Benchmark {
	return Benchmark{
		Version: BenchmarkVersion,
		Tasks: []BenchmarkTask{{
			Name:   "新建文件",
			Prompt: "写一个 hello.txt",
			Check:  BenchmarkCheck{Type: CheckTypeFileContains, Path: "hello.txt", Text: "hi"},
		}},
	}
}

func TestLoadBenchmarkRoundTrip(t *testing.T) {
	benchmark, err := LoadBenchmark(writeBenchmark(t, validBenchmark()))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(benchmark.Tasks) != 1 || benchmark.Tasks[0].Name != "新建文件" {
		t.Fatalf("任务集解析错误: %#v", benchmark)
	}
	cases := benchmark.Cases()
	if len(cases) != 1 || cases[0].Check == nil {
		t.Fatalf("用例转换错误: %#v", cases)
	}
}

func TestValidateBenchmarkRejectsBadVersion(t *testing.T) {
	benchmark := validBenchmark()
	benchmark.Version = 99
	if err := ValidateBenchmark(benchmark); err == nil {
		t.Fatal("版本不支持应报错")
	}
}

func TestValidateBenchmarkRejectsEmptyTasks(t *testing.T) {
	if err := ValidateBenchmark(Benchmark{Version: BenchmarkVersion}); err == nil {
		t.Fatal("空任务集应报错")
	}
}

func TestValidateBenchmarkRejectsDuplicateNames(t *testing.T) {
	benchmark := validBenchmark()
	benchmark.Tasks = append(benchmark.Tasks, benchmark.Tasks[0])
	if err := ValidateBenchmark(benchmark); err == nil {
		t.Fatal("重名应报错")
	}
}

func TestValidateBenchmarkRejectsUnsupportedCheck(t *testing.T) {
	benchmark := validBenchmark()
	benchmark.Tasks[0].Check.Type = "regex"
	if err := ValidateBenchmark(benchmark); err == nil {
		t.Fatal("未知判定类型应报错")
	}
}

func TestValidateBenchmarkRejectsEscapePath(t *testing.T) {
	benchmark := validBenchmark()
	benchmark.Tasks[0].Files = map[string]string{"../outside.txt": "x"}
	if err := ValidateBenchmark(benchmark); err == nil {
		t.Fatal("跳出工作区的路径应报错")
	}
}

func TestBenchmarkCasesRunEndToEnd(t *testing.T) {
	benchmark := validBenchmark()
	model := &scriptedModel{responses: []string{
		`<tool name="write_file" path="hello.txt"><content>hi</content></tool>`,
		"<final>写好了</final>",
	}}
	runner := Runner{Model: model, MaxSteps: 4}

	results := runner.RunCases(context.Background(), benchmark.Cases())
	if len(results) != 1 || !results[0].Passed {
		t.Fatalf("任务应通过: %#v", results)
	}
}

func TestBenchmarkCheckFileMissing(t *testing.T) {
	benchmark := Benchmark{
		Version: BenchmarkVersion,
		Tasks: []BenchmarkTask{{
			Name:   "不该建文件",
			Prompt: "什么都别做",
			Check:  BenchmarkCheck{Type: CheckTypeFileMissing, Path: "unused.txt"},
		}},
	}
	if err := ValidateBenchmark(benchmark); err != nil {
		t.Fatalf("validate: %v", err)
	}
	root := t.TempDir()
	if err := benchmark.Cases()[0].Check(root); err != nil {
		t.Fatalf("文件不存在时应通过判定: %v", err)
	}
	if !strings.Contains(filepath.Base(writeBenchmark(t, benchmark)), "benchmark.json") {
		t.Fatal("辅助函数应写出文件")
	}
}
