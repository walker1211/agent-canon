[中文](./README.zh-CN.md) | [English](./README.en.md)

# agent-canon

`agent-canon` is a git-like semantic migration, sync, and conflict-resolution tool for AI coding agent configurations. Its current focus is two-way migration between Claude Code and Codex CLI: scan and plan first, generate previews, resolve conflicts, apply safely, verify results, and keep rollback state.

The core idea: Claude Code to Codex CLI is not a directory copy. It maps project instructions, rules, skills, commands, MCP configuration, permission boundaries, and memory boundaries into the target tool's configuration model.

## Quick Start

The minimal golden path is read-only scan, state sync, then a dry-run for the Codex writeback.

```sh
agent-canon scan --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon sync claude codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

Show all commands:

```sh
agent-canon --help
```

Build a local binary from source:

```sh
./build.sh
```

After reviewing the dry-run output, apply explicitly:

```sh
agent-canon apply codex --yes --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

Global homes are not written by default. To write global Claude or Codex configuration, pass `--global` explicitly and review the dry-run output first.

If you already have a Codex config and only want to merge safe Claude MCP server entries:

```sh
agent-canon apply codex --global --merge-config --dry-run --only config --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

`--merge-config` only merges MCP server entries. It does not overwrite model, profile, sandbox, auth, provider, or feature settings.

## Common Commands

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

## Write Safety Boundary

`agent-canon` is conservative by default:

- `scan`, `plan`, `verify`, and `conflicts` are read-only.
- `export` and `compile` only write preview directories.
- `sync` and `resolve` only write project-local `.agent-canon` state.
- `apply` requires sync state and no open conflicts, creates backups before writing, and writes a rollback manifest.
- `rollback` only touches manifest-listed targets and checks drift before writing.
- Global Claude/Codex homes are not written by default; global writes require `--global`.
- Secrets are redacted by default and should not be migrated into target files, logs, or reports.

## Documentation

- [Documentation index](./docs/README.md)
- [Product and architecture design](./docs/design.md)
- [Resource mapping](./docs/resource-mapping.md)
- [Conflict resolution model](./docs/conflict-resolution.md)
- [Security and scope](./docs/security-and-scope.md)
- [Roadmap](./docs/roadmap.md)

## Contributing and Security

- Read [CONTRIBUTING.md](./CONTRIBUTING.md) before contributing.
- For security issues, read [SECURITY.md](./SECURITY.md). Do not paste secrets, private prompts, private logs, or exploit details into public issues.

## License

MIT. See [LICENSE](./LICENSE).
