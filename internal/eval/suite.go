package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultCases 是随仓库提供的固定任务集。
// 每条任务都能用工作区结果判定，不依赖模型自述。
func DefaultCases() []Case {
	return []Case{
		{
			Name:   "新建文件",
			Prompt: "在当前目录新建 hello.txt，内容写 hi。",
			Check:  CheckFileEquals("hello.txt", "hi"),
		},
		{
			Name:   "修改文件内容",
			Prompt: "把 main.go 里的 return 1 改成 return 2。",
			Files:  map[string]string{"main.go": "package main\n\nfunc answer() int {\n\treturn 1\n}\n"},
			Check:  CheckFileContains("main.go", "return 2"),
		},
		{
			Name:   "补充缺失的函数",
			Prompt: "在 util.go 里补一个 Add 函数，返回两个整数之和。",
			Files:  map[string]string{"util.go": "package util\n"},
			Check:  CheckFileContains("util.go", "func Add("),
		},
		{
			Name:   "按内容定位再修改",
			Prompt: "handler.go 里有一行 TODO 注释，把它替换成说明这个函数做什么的注释。",
			Files: map[string]string{
				"handler.go": "package api\n\n// TODO: 实现\nfunc Handle() {}\n",
				"readme.md":  "# demo\n",
			},
			Check: func(root string) error {
				payload, err := os.ReadFile(filepath.Join(root, "handler.go"))
				if err != nil {
					return err
				}
				content := string(payload)
				if strings.Contains(content, "TODO") {
					return fmt.Errorf("TODO 注释仍然存在")
				}
				if !strings.Contains(content, "func Handle()") {
					return fmt.Errorf("函数定义被改坏了")
				}
				return nil
			},
		},
		{
			Name:   "不应删除无关文件",
			Prompt: "在 notes.md 末尾追加一行 done。",
			Files: map[string]string{
				"notes.md": "# 记录\n",
				"keep.txt": "不能删\n",
			},
			Check: func(root string) error {
				if err := CheckFileContains("notes.md", "done")(root); err != nil {
					return err
				}
				return CheckFileContains("keep.txt", "不能删")(root)
			},
		},
		{
			// 工具集里没有删除能力，这条改成「清空」，仍能检验是否只动目标文件。
			Name:   "清空临时文件",
			Prompt: "把 temp.log 的内容清空，别动其它文件。",
			Files: map[string]string{
				"temp.log": "debug output\n",
				"keep.md":  "# 保留\n",
			},
			Check: func(root string) error {
				if err := CheckFileEquals("temp.log", "")(root); err != nil {
					return err
				}
				return CheckFileContains("keep.md", "保留")(root)
			},
		},
		{
			Name:   "在子目录新建文件",
			Prompt: "在 docs 目录下新建 guide.md，内容写 getting started。",
			Check:  CheckFileContains("docs/guide.md", "getting started"),
		},
		{
			Name:   "追加一行",
			Prompt: "在 changelog.md 末尾追加一行 - fix login。",
			Files:  map[string]string{"changelog.md": "# 变更记录\n"},
			Check:  CheckFileContains("changelog.md", "fix login"),
		},
		{
			Name:   "修正返回值",
			Prompt: "calc.go 里的 Add 函数返回了差，改成返回两个参数的和。",
			Files:  map[string]string{"calc.go": "package calc\n\nfunc Add(a, b int) int {\n\treturn a - b\n}\n"},
			Check:  CheckFileContains("calc.go", "return a + b"),
		},
		{
			Name:   "不改无关文件",
			Prompt: "把 config.yaml 里的 port 从 8080 改成 9090。",
			Files: map[string]string{
				"config.yaml": "port: 8080\nhost: localhost\n",
				"main.go":     "package main\n",
			},
			Check: func(root string) error {
				if err := CheckFileContains("config.yaml", "9090")(root); err != nil {
					return err
				}
				return CheckFileEquals("main.go", "package main")(root)
			},
		},
	}
}
