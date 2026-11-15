package eval

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
			Prompt: "找到 handler.go 里 TODO 那一行，把 TODO 换成实际实现说明。",
			Files: map[string]string{
				"handler.go": "package api\n\n// TODO: 实现\nfunc Handle() {}\n",
				"readme.md":  "# demo\n",
			},
			Check: CheckFileContains("handler.go", "实际实现"),
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
	}
}
