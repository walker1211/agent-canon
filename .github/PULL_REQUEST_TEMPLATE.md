## Summary

- 

## Test plan

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `find . -name '*.go' -not -path './.git/*' -print0 | xargs -0 gofmt -l` returns no files

## Safety checklist

- [ ] I did not include secrets, tokens, private prompts, private screenshots, databases, full local paths, or sensitive logs.
- [ ] I updated docs for user-visible command or behavior changes.
- [ ] I documented any write-boundary, backup, rollback, secret-handling, or global-home impact.
- [ ] I reviewed generated files and templates for placeholder-only examples.
