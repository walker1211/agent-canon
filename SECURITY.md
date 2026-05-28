# Security Policy

## Reporting a Vulnerability

Use GitHub Security private vulnerability reporting for this repository when it is available.

If private vulnerability reporting is not available, open a public issue with only minimal coordination details. Do not include secrets, private prompts, private screenshots, databases, `.env`, `configs/config.yaml`, full local paths, or exploit details in a public issue.

A good public placeholder issue says what kind of component is affected and asks maintainers how to continue privately. Share sensitive reproduction material only through a private channel agreed with the maintainers.

## Scope

Security reports are especially useful for:

- Secret leakage in CLI output, reports, backups, rollback manifests, or generated target files.
- Unsafe writes outside the requested project, Claude home, or Codex home boundary.
- Symlink or path traversal behavior that bypasses write safety checks.
- Incorrect migration of dangerous permissions or hooks.
- Behavior that disables verification, confirmation, backups, or rollback unexpectedly.

## Project Security Principles

`agent-canon` is designed to be conservative:

- Dry-run before write.
- No global home writes unless explicitly requested.
- Backup before write.
- Rollback manifests for applied changes.
- No secret migration by default.
- Warnings for lossy or unsafe conversions.

## Public Disclosure

Please do not publish exploit details before maintainers have had time to investigate and prepare a fix. Public issues should avoid actionable attack instructions and should never contain real credentials or private project data.
