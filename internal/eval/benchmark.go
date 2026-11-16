package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BenchmarkVersion 是任务集文件的格式版本。
const BenchmarkVersion = 1

// Benchmark 是一份可校验的评测任务集。
type Benchmark struct {
	Version int             `json:"version"`
	Tasks   []BenchmarkTask `json:"tasks"`
}

// BenchmarkTask 是任务集里的一条任务。
type BenchmarkTask struct {
	Name   string            `json:"name"`
	Prompt string            `json:"prompt"`
	Files  map[string]string `json:"files,omitempty"`
	Check  BenchmarkCheck    `json:"check"`
}

// BenchmarkCheck 是结果判定方式。
type BenchmarkCheck struct {
	Type string `json:"type"`
	Path string `json:"path"`
	Text string `json:"text,omitempty"`
}

// 支持的判定类型。
const (
	CheckTypeFileEquals   = "file_equals"
	CheckTypeFileContains = "file_contains"
	CheckTypeFileMissing  = "file_missing"
)

// LoadBenchmark 读取并校验任务集文件。
func LoadBenchmark(path string) (Benchmark, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return Benchmark{}, err
	}
	var benchmark Benchmark
	if err := json.Unmarshal(payload, &benchmark); err != nil {
		return Benchmark{}, fmt.Errorf("解析任务集失败: %w", err)
	}
	if err := ValidateBenchmark(benchmark); err != nil {
		return Benchmark{}, err
	}
	return benchmark, nil
}

// ValidateBenchmark 校验任务集结构，拒绝会导致判定失真的写法。
func ValidateBenchmark(benchmark Benchmark) error {
	if benchmark.Version != BenchmarkVersion {
		return fmt.Errorf("任务集版本不支持: %d", benchmark.Version)
	}
	if len(benchmark.Tasks) == 0 {
		return errors.New("任务集为空")
	}
	seen := map[string]bool{}
	for index, task := range benchmark.Tasks {
		name := strings.TrimSpace(task.Name)
		if name == "" {
			return fmt.Errorf("第 %d 条任务缺少名称", index+1)
		}
		if seen[name] {
			return fmt.Errorf("任务名重复: %s", name)
		}
		seen[name] = true
		if strings.TrimSpace(task.Prompt) == "" {
			return fmt.Errorf("任务 %s 缺少请求", name)
		}
		for path := range task.Files {
			if err := validateFixturePath(path); err != nil {
				return fmt.Errorf("任务 %s: %w", name, err)
			}
		}
		if err := validateCheck(name, task.Check); err != nil {
			return err
		}
	}
	return nil
}

func validateCheck(name string, check BenchmarkCheck) error {
	if strings.TrimSpace(check.Path) == "" {
		return fmt.Errorf("任务 %s 的判定缺少路径", name)
	}
	if err := validateFixturePath(check.Path); err != nil {
		return fmt.Errorf("任务 %s: %w", name, err)
	}
	switch check.Type {
	case CheckTypeFileEquals, CheckTypeFileContains:
		if strings.TrimSpace(check.Text) == "" {
			return fmt.Errorf("任务 %s 的判定缺少期望文本", name)
		}
	case CheckTypeFileMissing:
	default:
		return fmt.Errorf("任务 %s 的判定类型不支持: %s", name, check.Type)
	}
	return nil
}

// validateFixturePath 拒绝绝对路径和向上跳目录。
func validateFixturePath(path string) error {
	if filepath.IsAbs(path) {
		return fmt.Errorf("准备文件路径不能是绝对路径: %s", path)
	}
	cleaned := filepath.Clean(path)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("准备文件路径不能跳出工作区: %s", path)
	}
	return nil
}

// Cases 把任务集转成可直接执行的用例。
func (b Benchmark) Cases() []Case {
	cases := make([]Case, 0, len(b.Tasks))
	for _, task := range b.Tasks {
		cases = append(cases, Case{
			Name:   task.Name,
			Prompt: task.Prompt,
			Files:  task.Files,
			Check:  task.checkFunc(),
		})
	}
	return cases
}

func (t BenchmarkTask) checkFunc() func(root string) error {
	switch t.Check.Type {
	case CheckTypeFileEquals:
		return CheckFileEquals(t.Check.Path, t.Check.Text)
	case CheckTypeFileContains:
		return CheckFileContains(t.Check.Path, t.Check.Text)
	default:
		return CheckFileMissing(t.Check.Path)
	}
}
