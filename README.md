# CodeMate

本地代码智能体，Go 实现。在终端里读取仓库、修改文件、执行命令，按多步任务推进；工具调用走文本协议，上下文按预算分节装配，会话状态与记忆保存在本地，中断后可以继续。

## 能力

- **回合循环**：拼 prompt、请求模型、解析应答、执行工具、结果回灌，直到模型给出最终答案。工具步数与模型重试分别计数，步数用尽后追加一次收尾请求。
- **工具协议**：模型每次只返回一个 `<tool>...</tool>` 或 `<final>...</final>`。格式不合法时不重试同一请求，而是把格式提示写回历史，让模型看到自己的错误再改。
- **内置工具**：`list_files`、`search`、`read_file`、`write_file`、`edit_file`、`patch_file`、`run_shell`、`load_skill`、`delegate`。执行前做参数语义校验，执行后带状态、错误码、风险等级、受影响路径。
- **子 agent 委派**：`delegate` 开一个只读子 agent 做调查，只把结论带回主上下文；嵌套深度受上限约束。
- **上下文治理**：一轮 prompt 分五节（前缀、记忆、相关笔记、历史、当前请求），各有预算与下限；总量超上限时按固定顺序收缩。历史按新旧分层，近期保留正文，旧条目压成一行并按键去重。
- **记忆**：短期记忆存任务摘要、文件摘要与过程笔记；长期记忆按项目约定、关键决策、依赖事实、用户偏好四类跨会话保存。说「记住」时自动提升，同主语的新记录替换旧记录。
- **权限审批**：路径越界、危险命令、文件改动三道门；策略可选逐次询问、自动放行、全部拒绝。支持只读模式与按次运行的工具白名单。
- **安全边界**：密钥类环境变量在工件与存档写入前统一打码；子进程只继承白名单环境变量。
- **文件改动评审**：写盘前生成 unified diff 供确认。
- **会话恢复**：历史、工作区指纹与检查点一起落盘；指纹、模型或工具签名变化时拒绝恢复，并记一条身份不匹配事件。
- **运行工件**：每次运行的模型请求、解析结果、工具执行、任务状态、检查点与汇总报告都落在运行目录。
- **模型接入**：OpenAI 兼容、Anthropic 兼容、Ollama 三家；支持请求超时、采样参数、输出上限与提示词缓存。
- **评测**：任务集可从文件加载并校验；指标既能从内存结果算，也能从运行工件聚合；内置消融、上下文压力、记忆依赖与回归对比四种实验。
- **技能**：扫描 `SKILL.md`，按需加载正文。

## 构建

```bash
go build -o bin/codemate ./cmd/codemate
```

## 运行参数

```bash
bin/codemate                       # 进入交互模式
bin/codemate --resume latest       # 恢复最近一次会话
bin/codemate --read-only           # 只允许只读工具
bin/codemate --tool read_file --tool search
bin/codemate --approval auto       # ask | auto | never
bin/codemate --max-new-tokens 512
```

## 评测

```bash
go run ./cmd/eval                                  # 跑内置任务集
go run ./cmd/eval -benchmark tasks.json            # 跑任务集文件
go run ./cmd/eval -ablate context_reduction        # 消融对照
go run ./cmd/eval -stress                          # 上下文压力测量
go run ./cmd/eval -save-baseline base.json         # 记基线
go run ./cmd/eval -baseline base.json -tolerance 0.05   # 回归对比，超容忍度时退出码非零
go run ./cmd/eval -memory "token 过期时间是 30 分钟"    # 记忆依赖实验
go run ./cmd/eval -runs .codemate/runs             # 聚合历史运行工件
```

任务集文件格式：

```json
{
  "version": 1,
  "tasks": [
    {
      "name": "新建文件",
      "prompt": "把 hi 写进 hello.txt",
      "files": {"a.txt": "初始内容"},
      "check": {"type": "file_contains", "path": "hello.txt", "text": "hi"}
    }
  ]
}
```

判定类型支持 `file_equals`、`file_contains`、`file_missing`。准备文件路径不允许绝对路径或跳出工作区。

`MINI_CODE_MODEL_MODE=mock` 可以离线跑通链路，但结果全部失败，只用于验证流程。

## 配置

模型配置放在 `~/.mini-code/settings.json` 或项目内的 `.mini-code/settings.json`，支持 OpenAI 兼容、Anthropic 兼容与 Ollama 三类接口。

也可以在项目里放 `.env`，程序会从当前目录向上查找并补足缺失项，真实环境变量优先。可用键：`ANTHROPIC_MODEL`、`ANTHROPIC_AUTH_TOKEN`、`ANTHROPIC_API_KEY`、`OPENAI_MODEL`、`OLLAMA_MODEL`、`MINI_CODE_PROVIDER`、`MINI_CODE_TIMEOUT`、`MINI_CODE_TEMPERATURE`、`MINI_CODE_TOP_P`、`MINI_CODE_MAX_OUTPUT_TOKENS`。

## 已知边界

单机单用户，没有并发与多租户设计。

`run_shell` 靠审批与白名单约束，不是沙箱隔离，没有资源限额。

工具调用走文本协议：不接 MCP，也没有原生 function calling。代价是依赖模型遵循格式，格式遵从度差的模型会频繁解析失败。

长期记忆与短期记忆都写在工作区内的 `.codemate/` 下，不加密；密钥类内容会被打码，但普通业务文本按原样保存。

## License

MIT，见 [LICENSE](LICENSE)。
