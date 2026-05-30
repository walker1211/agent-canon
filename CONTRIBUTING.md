# Contributing

Thanks for helping improve `agent-canon`.

## Local Setup

Requirements:

- Go 1.22 or newer compatible with the module version.
- Git.
- GitHub CLI only if you are using GitHub readiness checks.

Build the CLI locally:

```sh
./build.sh
```

The script writes `agent-canon` in the repository root.

Run the CLI from source:

```sh
go run ./cmd/agent-canon --help
```

## Test

Run the full test suite before opening a pull request:

```sh
go test ./...
```

Run vet locally:

```sh
go vet ./...
```

Check formatting:

```sh
find . -name '*.go' -not -path './.git/*' -print0 | xargs -0 gofmt -l
```

The formatting command should print no files. If it does, run `gofmt -w` on the reported Go files.

## Local CI

The GitHub CI workflow mirrors the local checks:

```sh
go test ./...
go vet ./...
find . -name '*.go' -not -path './.git/*' -print0 | xargs -0 gofmt -l
```

## Secret Scan

Do not commit secrets, tokens, private prompts, local databases, private screenshots, or machine-specific paths.

Before submitting docs or templates, check public-facing files for obvious credential-shaped leaks. Search for provider-specific token prefixes, assigned API key examples, and unredacted credentials.

Also review examples manually for machine-specific absolute paths. Use placeholders such as `<repo-root>`, `~`, `<token>`, or `<REDACTED>` instead of real local or credential values.

## GitHub Readiness Audit

Maintainers can run the read-only GitHub readiness audit before making the repository public or before cutting a release:

```sh
scripts/github-readiness.sh --repo OWNER/REPO
scripts/github-readiness.sh --repo OWNER/REPO --strict
```

The audit does not change GitHub settings. It reports Secret scanning, push protection, private vulnerability reporting, default branch protection or rulesets, and CodeQL/code-scanning status with remediation guidance for missing items.

The release preflight runs the strict audit before publishing tag artifacts, so missing readiness settings stop the release before a GitHub Release is created or updated.

## Pull Request Requirements

A pull request should include:

- A clear summary of the change.
- The test commands you ran.
- Notes for any write-boundary, secret-handling, backup, rollback, or global-home behavior change.
- Documentation updates when user-visible commands or safety guarantees change.

Do not include secrets, local absolute paths, private logs, or sensitive reproduction material in a public pull request.

## Commit Message

Use Conventional Commits style when possible:

```text
feat(cli): add a user-visible capability
fix(apply): correct writeback validation
docs: clarify quick start
```

## Release and Tag Ownership

Maintainers own release tags and published artifacts. Contributors should not create release tags unless a maintainer explicitly asks for it.

Build a local release archive with:

```sh
scripts/package-release.sh vX.Y.Z
```

Run the clean local release checks before tagging:

```sh
scripts/ci-local.sh clean
```

Create a local release tag with:

```sh
scripts/tag-release.sh vX.Y.Z
```

The tag helper creates the tag locally and prints the explicit `git push origin <tag>` command for maintainers to run after final GitHub readiness review.

Release archives should contain only:

- `agent-canon`
- `LICENSE`
- `README.md`
- `README.zh-CN.md`
- `README.en.md`

They should not contain local config, `.env`, secrets, databases, logs, generated workspace state, or private assets.
