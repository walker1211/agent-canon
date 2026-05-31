[中文](./README.zh-CN.md) | [English](./README.en.md)

# agent-canon

`agent-canon` 是一个面向 AI 编程工具配置的语义迁移、readiness 和冲突审查工作流。当前黄金路径是把 Claude Code 语义迁移到 Codex CLI：先扫描和计划，再生成 preview，解决冲突后安全写回，并保留可验证、可回滚的状态。

核心原则：Claude Code 到 Codex CLI 不是整目录复制，而是把项目指令、规则、技能、命令、MCP 配置、权限边界和记忆边界映射到目标工具能理解的配置模型。

## Quick Start

下面是最小黄金路径：先只读扫描，再同步生成状态，最后用 dry-run 查看将要写入 Codex 的内容。

```sh
agent-canon scan --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon sync claude codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

查看完整命令：

```sh
agent-canon --help
```

## 安装与 Release 归档

请从 GitHub Release 下载对应平台的归档。归档命名为 `agent-canon_vX.Y.Z_<goos>_<goarch>.tar.gz`，包含 `agent-canon` 可执行文件、`LICENSE`、`README.md`、`README.zh-CN.md` 和 `README.en.md`。

运行前，请使用同一 release 中的 `checksums.txt` 校验下载的归档。安装后的第一个安全命令是：

```sh
agent-canon --help
```

中文指南请继续阅读本文档；英文指南请查看 `README.en.md`。如果不下载 release 归档，也可以从源码构建本地可执行文件：

```sh
./build.sh
```

确认 dry-run 输出后，可以显式执行写回：

```sh
agent-canon apply codex --yes --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

默认不会写入全局 home。需要写入全局 Claude 或 Codex 配置时，必须显式传 `--global` 并先审查 dry-run 输出。

如果你已经有一份 Codex 配置，只想合并安全的 Claude MCP server entries：

```sh
agent-canon apply codex --global --merge-config --dry-run --only config --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

`--merge-config` 只合并 MCP server entries，不会覆盖 model、profile、sandbox、auth、provider 或 feature settings。

## 当前范围

`agent-canon` 当前聚焦 Claude Code 到 Codex CLI 的语义迁移与审查。它支持项目内状态、显式 global home 写回、冲突审查、备份、rollback manifest，以及 MCP server entry 合并。

当前非目标：

- 不做 MCP server entries 之外的任意 TOML merge。
- 不把 secret 迁移到目标文件、日志或报告中。
- 不默认写入全局 Claude 或 Codex home。
- 不迁移完整会话历史。
- 不承诺 hooks、权限、agents 或 memory 能在不同工具间无损转换。

## 常用命令

```sh
agent-canon scan
agent-canon plan
agent-canon export codex --out <preview-dir>
agent-canon export claude --out <preview-dir>
agent-canon compile codex --out <preview-dir>
agent-canon compile claude --out <preview-dir>
agent-canon sync claude codex
agent-canon status
agent-canon conflicts
agent-canon resolve <conflict-id> --manual <value>
agent-canon apply codex --dry-run
agent-canon apply claude --dry-run
agent-canon verify codex
agent-canon verify claude
agent-canon rollback <apply-id> --dry-run
```

## 场景示例

### 只预览迁移结果

```sh
agent-canon scan --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon sync claude codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon status --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon compile codex --out <preview-dir> --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon verify codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

### 写入前审查并解决冲突

```sh
agent-canon conflicts --project <repo-root>
agent-canon resolve <conflict-id> --manual <value> --project <repo-root>
agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

### 安全检查 global home 变更

```sh
agent-canon apply codex --global --dry-run --only config --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

只有审查输出后，才把 `--dry-run` 换成 `--yes`。除非明确要写入选定的 global home 目标，否则不要使用 `--global --yes`。

## 写入安全边界

`agent-canon` 默认保守：

- `scan`、`plan`、`verify`、`conflicts` 是只读命令。
- `export` 和 `compile` 只写 preview 目录。
- `sync` 和 `resolve` 只写项目内 `.agent-canon` 状态。
- `apply` 必须先通过 sync state 和 open conflict 检查，写入前创建备份，并生成 rollback manifest。
- `rollback` 只回滚 manifest 中列出的目标，并在写入前检查漂移。
- 默认不写全局 Claude/Codex home；全局写入需要 `--global`。
- Secret 默认 redacted，不应迁移到目标文件、日志或报告。

## 贡献与安全

- 贡献前请阅读 [CONTRIBUTING.md](./CONTRIBUTING.md)。
- 安全问题请阅读 [SECURITY.md](./SECURITY.md)，不要在公开 issue 中粘贴 secret、私有 prompt、私有日志或 exploit details。

## License

MIT. See [LICENSE](./LICENSE).
