[中文](./README.zh-CN.md) | [English](./README.en.md)

# agent-canon

`agent-canon` 是一个面向 AI 编程工具配置的语义迁移、同步和冲突解决工具。当前重点是 Claude Code 与 Codex CLI 之间的双向迁移：先扫描和计划，再生成 preview，解决冲突后安全写回，并保留可验证、可回滚的状态。

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

从源码构建本地可执行文件：

```sh
./build.sh
```

确认 dry-run 输出后，可以显式执行写回：

```sh
agent-canon apply codex --yes --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

默认不会写入全局 home。需要写入全局 Claude 或 Codex 配置时，必须显式传 `--global` 并先审查 dry-run 输出。

## 常用命令

```sh
agent-canon scan
agent-canon plan
agent-canon export codex --out <preview-dir>
agent-canon export claude --out <preview-dir>
agent-canon compile codex --out <preview-dir>
agent-canon compile claude --out <preview-dir>
agent-canon sync claude codex
agent-canon conflicts
agent-canon resolve <conflict-id> --manual <value>
agent-canon apply codex --dry-run
agent-canon apply claude --dry-run
agent-canon verify codex
agent-canon verify claude
agent-canon rollback <apply-id> --dry-run
```

## 写入安全边界

`agent-canon` 默认保守：

- `scan`、`plan`、`verify`、`conflicts` 是只读命令。
- `export` 和 `compile` 只写 preview 目录。
- `sync` 和 `resolve` 只写项目内 `.agent-canon` 状态。
- `apply` 必须先通过 sync state 和 open conflict 检查，写入前创建备份，并生成 rollback manifest。
- `rollback` 只回滚 manifest 中列出的目标，并在写入前检查漂移。
- 默认不写全局 Claude/Codex home；全局写入需要 `--global`。
- Secret 默认 redacted，不应迁移到目标文件、日志或报告。

## 文档

- [文档索引](./docs/README.md)
- [产品与架构设计](./docs/design.md)
- [资源映射](./docs/resource-mapping.md)
- [冲突解决模型](./docs/conflict-resolution.md)
- [安全与边界](./docs/security-and-scope.md)
- [路线图](./docs/roadmap.md)

## 贡献与安全

- 贡献前请阅读 [CONTRIBUTING.md](./CONTRIBUTING.md)。
- 安全问题请阅读 [SECURITY.md](./SECURITY.md)，不要在公开 issue 中粘贴 secret、私有 prompt、私有日志或 exploit details。

## License

MIT. See [LICENSE](./LICENSE).
