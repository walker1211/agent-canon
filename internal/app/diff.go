package app

import (
	"io"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
	"github.com/zhangyoujun/agent-canon/internal/semanticdiff"
	"github.com/zhangyoujun/agent-canon/internal/snapshot"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runDiff(opts cli.Options, stdout io.Writer) error {
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	base, ok, err := loadBaseSnapshots(layout)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	if !ok {
		return withExitCode(1, "no base snapshots found; run \"agent-canon sync claude codex\" first")
	}

	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	current, err := snapshot.Build(scanReport)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	learned, err := loadLearnedResolutions(layout, opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	previous, err := loadPreviousSyncState(layout)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	comparison := semanticdiff.Compare(semanticdiff.Input{
		BaseClaude:    base.Claude,
		BaseCodex:     base.Codex,
		CurrentClaude: current.Claude,
		CurrentCodex:  current.Codex,
		Learned:       learned,
	})
	warnings := appendWarnings(scanReport.Warnings, current.Claude.Warnings...)
	warnings = appendWarnings(warnings, current.Codex.Warnings...)
	warnings = appendWarnings(warnings, current.Canon.Warnings...)
	conflicts := comparison.Conflicts
	configConflicts, configWarnings, err := detectCodexMCPConfigConflicts(scanReport)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	conflicts = append(conflicts, configConflicts...)
	conflicts = assignConflictIDsPreservingResolved(conflicts, previous.Conflicts)
	warnings = appendWarnings(warnings, configWarnings...)
	report := model.DiffReport{
		SchemaVersion: model.DiffSchemaVersion,
		Project:       opts.Project,
		Target:        opts.DiffTarget,
		Diffs:         comparison.Diffs,
		Conflicts:     conflicts,
		Warnings:      warnings,
	}
	report.Summary = diffSummary(report)

	if opts.Format == "json" {
		if err := render.DiffJSON(stdout, report); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.DiffText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func diffSummary(report model.DiffReport) model.DiffSummary {
	summary := model.DiffSummary{Diffs: len(report.Diffs), Warnings: len(report.Warnings)}
	for _, conflict := range report.Conflicts {
		switch conflict.Status {
		case model.ConflictStatusResolved:
			summary.ResolvedConflicts++
		default:
			summary.OpenConflicts++
		}
	}
	return summary
}
