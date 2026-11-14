# CodeMate

本地代码智能体，Go 实现。在终端里读取仓库、修改文件、执行命令，按多步任务推进，并把会话状态保存在本地，支持中断后继续。

## 能力

- **Agent Loop**：模型产出工具调用，宿主执行并把结果回灌，直到给出最终答案；单次任务最多 80 步。
- **工具**：`list_files`、`grep_files`、`read_file`、`write_file`、`edit_file`、`patch_file`、`run_command`、`load_skill` 八个内置工具，统一 JSON Schema 校验、统一执行入口。
- **权限审批**：路径越界、危险命令、文件改动三道门，拒绝原因回传模型。
- **文件改动评审**：写盘前生成 unified diff 供确认。
- **会话持久化**：消息与会话 ID 落盘，支持 `--resume` 续跑。
- **MCP**：外部 server 以子进程接入，工具挂进同一注册表。
- **技能**：扫描 `SKILL.md`，按需加载正文。

## 构建

```bash
go build -o bin/minicode ./cmd/minicode
```

## 配置

模型配置放在 `~/.mini-code/settings.json` 或项目的 `.mini-code/settings.json`。

支持 OpenAI 兼容与 Anthropic 兼容两类接口。

## License

MIT，见 [LICENSE](LICENSE)。
