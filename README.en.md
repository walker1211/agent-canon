[中文](./README.zh-CN.md) | [English](./README.en.md)

# agent-canon

`agent-canon` is a bidirectional CLI migration, readiness, and conflict-review workflow for Claude Code and Codex CLI configurations. It helps move AI coding agent instructions, rules, skills, commands, MCP server entries, permission boundaries, and memory boundaries between the two tools without treating the migration as a directory copy.

Claude Code and Codex CLI use different configuration models. The golden path is to scan and plan first, generate previews, resolve conflicts, apply safely, verify results, and keep rollback state.

## Why agent-canon

Moving from Claude Code to Codex CLI, or from Codex CLI back to Claude Code, is easy to underestimate. The hard part is not copying files; it is deciding what can be copied, what must be generated for the target tool, what can be merged, and what needs human review.

Common migration risks:

- Claude Code and Codex CLI do not store the same concepts in the same files.
- Existing target configs may already contain model, sandbox, auth, provider, feature, or MCP settings.
- Global homes may contain private local state and should not be written by default.
- Some semantics, such as path-scoped rules, local settings, memory boundaries, and conflicting MCP entries, need review instead of blind conversion.

`agent-canon` handles migration as copy / generate / merge, not cutover. Original tool configuration is preserved, real writes require explicit confirmation, and writebacks keep rollback manifests.

## Quick Start

The minimal source-build golden path is read-only scan, state sync, then a dry-run for the Codex writeback. These examples assume you built the local binary and run it from the repository root with `./agent-canon`.

```sh
./agent-canon scan --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon sync claude codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

Show all commands:

```sh
./agent-canon --help
```

For the reverse direction, pass the source and target explicitly:

```sh
./agent-canon scan codex claude --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon plan codex claude --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon apply claude --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

## Install and Release Archives

Download the archive for your platform from the GitHub Release. Archives are named `agent-canon_vX.Y.Z_<goos>_<goarch>.tar.gz` and include the `agent-canon` binary, `LICENSE`, `README.md`, `README.zh-CN.md`, and `README.en.md`.

Verify the downloaded archive with `checksums.txt` from the same release before running the binary. The first safe command after install is:

```sh
agent-canon --help
```

Use this README for the English guide and `README.zh-CN.md` for the Chinese guide. If you prefer a source build instead of downloading a release archive, run the local binary from the repository root with `./agent-canon`:

```sh
./build.sh
./agent-canon --help
```

After reviewing the dry-run output, apply explicitly:

```sh
./agent-canon apply codex --yes --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

Global homes are not written by default. To write global Claude or Codex configuration, pass `--global` explicitly and review the dry-run output first.

If you already have a Codex config and only want to merge safe Claude MCP server entries:

```sh
./agent-canon apply codex --global --merge-config --dry-run --only config --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

`--merge-config` only merges MCP server entries. It does not overwrite model, profile, sandbox, auth, provider, or feature settings.

If a bridge command, temporary skill, or local resource should not migrate to the target tool, skip it in the project-local `.agent-canon/config.toml`. The `.agent-canon/` directory is local workspace state and should not be committed:

```toml
[skip]
resources = [
  "command:global-ccs",
  "skill:global-ccs-delegation",
]
paths = [
  "~/.claude/commands/ccs",
  "~/.claude/commands/ccs.md",
]
```

Skip rules apply during scan, so they affect later `plan`, `export`, `compile`, `sync`, `apply`, and `verify` results.

## Current Scope

`agent-canon` currently focuses on bidirectional Claude Code and Codex CLI semantic migration and review. It supports project-local state, explicit global-home writebacks, conflict review, backups, rollback manifests, and MCP server entry merge support.

Non-goals for the current scope:

- No arbitrary TOML merge beyond MCP server entries.
- No secret migration into target files, logs, or reports.
- No default writes to global Claude or Codex homes.
- No full session history migration.
- No guarantee that hooks, permissions, agents, or memory convert losslessly across tools.

## Common Commands

```sh
./agent-canon scan
./agent-canon plan
./agent-canon export codex --out <preview-dir>
./agent-canon export claude --out <preview-dir>
./agent-canon compile codex --out <preview-dir>
./agent-canon compile claude --out <preview-dir>
./agent-canon sync claude codex
./agent-canon status
./agent-canon conflicts
./agent-canon resolve <conflict-id> --manual <value>
./agent-canon apply codex --dry-run
./agent-canon apply claude --dry-run
./agent-canon verify codex
./agent-canon verify claude
./agent-canon rollback <apply-id> --dry-run
```

## Scenario Examples

### Preview a migration without writing targets (Claude to Codex)

```sh
./agent-canon scan --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon sync claude codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon status --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon compile codex --out <preview-dir> --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon verify codex --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

### Preview a Codex to Claude migration without writing targets

```sh
./agent-canon scan codex claude --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon plan codex claude --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon compile claude --out <preview-dir> --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon verify claude --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
./agent-canon apply claude --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

### Review and resolve conflicts before applying

```sh
./agent-canon conflicts --project <repo-root>
./agent-canon resolve <conflict-id> --manual <value> --project <repo-root>
./agent-canon apply codex --dry-run --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

### Inspect global-home changes safely

```sh
./agent-canon apply codex --global --dry-run --only config --project <repo-root> --claude-home ~/.claude --codex-home ~/.codex
```

Only replace `--dry-run` with `--yes` after reviewing the output. Do not use `--global --yes` unless you intentionally want to write selected global home targets.

### Check rollback before restoring files

```sh
./agent-canon rollback <apply-id> --dry-run
```

Rollback only touches files listed in the apply manifest and checks drift before writing.

## Write Safety Boundary

`agent-canon` is conservative by default:

- `scan`, `plan`, `verify`, and `conflicts` are read-only.
- `export` and `compile` only write preview directories.
- `sync` and `resolve` only write project-local `.agent-canon` state.
- `apply` requires sync state and no open conflicts, creates backups before writing, and writes a rollback manifest.
- `rollback` only touches manifest-listed targets and checks drift before writing.
- Global Claude/Codex homes are not written by default; global writes require `--global`.
- Secrets are redacted by default and should not be migrated into target files, logs, or reports.

## Contributing and Security

- Read [CONTRIBUTING.md](./CONTRIBUTING.md) before contributing.
- For security issues, read [SECURITY.md](./SECURITY.md). Do not paste secrets, private prompts, private logs, or exploit details into public issues.

## License

MIT. See [LICENSE](./LICENSE).
