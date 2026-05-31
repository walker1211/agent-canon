package app

import (
	"io"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/verifier"
)

func runVerify(opts cli.Options, stdout io.Writer) error {
	report, err := verifier.Verify(verifier.Options{Target: opts.VerifyTarget, Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	if opts.Format == "json" {
		if err := render.VerifyJSON(stdout, report); err != nil {
			return withExitCode(1, "%w", err)
		}
	} else if err := render.VerifyText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	if report.Summary.Fail > 0 {
		return withExitCode(1, "verify %s failed with %d failed checks; see failed checks above, fix the target files or conflicts, then rerun verify", opts.VerifyTarget, report.Summary.Fail)
	}
	return nil
}
