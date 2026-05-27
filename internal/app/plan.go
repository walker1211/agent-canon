package app

import (
	"fmt"
	"io"
	"os"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
)

func runPlan(opts cli.Options, stdout io.Writer) error {
	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	planReport := planner.Build(scanReport)

	if opts.Out != "" {
		if err := writePlanFile(opts.Out, planReport); err != nil {
			return withExitCode(1, "%w", err)
		}
		if err := writeLine(stdout, "agent-canon plan: %s -> %s", planReport.Source, planReport.Target); err != nil {
			return withExitCode(1, "%w", err)
		}
		if err := writeLine(stdout, "Project: %s", planReport.Project); err != nil {
			return withExitCode(1, "%w", err)
		}
		return writeLine(stdout, "wrote JSON plan to %s (%d operations)", opts.Out, len(planReport.Operations))
	}

	if opts.Format == "json" {
		if err := render.PlanJSON(stdout, planReport); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.PlanText(stdout, planReport); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func writePlanFile(path string, report model.PlanReport) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("write plan %q: %w", path, err)
	}

	writeErr := render.PlanJSON(file, report)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write plan %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close plan %q: %w", path, closeErr)
	}
	return nil
}
