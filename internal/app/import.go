package app

import (
	"errors"
	"io"
	"time"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
	"github.com/zhangyoujun/agent-canon/internal/security"
	"github.com/zhangyoujun/agent-canon/internal/snapshot"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

type importTarget struct {
	tool         string
	snapshot     model.SnapshotReport
	snapshotPath string
	reportPath   string
	saveSnapshot func(any) error
	saveReport   func(any) error
}

func runImport(opts cli.Options, stdout io.Writer) error {
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	current, err := snapshot.Build(scanReport)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	var target importTarget
	switch opts.ImportTarget {
	case "claude":
		target = importTarget{
			tool:         "claude",
			snapshot:     redactedSnapshotWarnings(current.Claude),
			snapshotPath: layout.BaseClaude,
			reportPath:   layout.ImportClaude,
			saveSnapshot: layout.SaveBaseClaude,
			saveReport:   layout.SaveImportClaude,
		}
	case "codex":
		target = importTarget{
			tool:         "codex",
			snapshot:     redactedSnapshotWarnings(current.Codex),
			snapshotPath: layout.BaseCodex,
			reportPath:   layout.ImportCodex,
			saveSnapshot: layout.SaveBaseCodex,
			saveReport:   layout.SaveImportCodex,
		}
	default:
		return withExitCode(1, "unsupported import target %q", opts.ImportTarget)
	}
	return runImportTarget(opts, stdout, layout, scanReport, target)
}

func runImportTarget(opts cli.Options, stdout io.Writer, layout workspace.Layout, scanReport model.ScanReport, target importTarget) error {
	if err := ensureImportManifest(opts, layout); err != nil {
		return err
	}
	if err := target.saveSnapshot(target.snapshot); err != nil {
		return withExitCode(1, "%w", err)
	}

	scanWarnings := redactedWarnings(scanReport.Warnings)
	warnings := appendWarnings(scanWarnings, target.snapshot.Warnings...)
	report := model.ImportReport{
		SchemaVersion: model.ImportSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:       opts.Project,
		Tool:          target.tool,
		WorkspaceRoot: layout.Root,
		SnapshotPath:  target.snapshotPath,
		ReportPath:    target.reportPath,
		Summary: model.ImportSummary{
			Resources: len(target.snapshot.Resources),
			Warnings:  len(warnings),
		},
		Warnings: warnings,
	}
	if err := target.saveReport(report); err != nil {
		return withExitCode(1, "%w", err)
	}

	if opts.Format == "json" {
		if err := render.ImportJSON(stdout, report); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.ImportText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func ensureImportManifest(opts cli.Options, layout workspace.Layout) error {
	var existing model.WorkspaceManifestReport
	if err := layout.LoadManifest(&existing); err != nil {
		if !errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "read workspace manifest: %w", err)
		}
	} else {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	report := model.WorkspaceManifestReport{
		SchemaVersion: model.WorkspaceManifestSchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
		Project:       opts.Project,
		Source:        "claude",
		Target:        "codex",
		WorkspaceRoot: layout.Root,
		Warnings:      []model.Warning{},
	}
	if err := layout.SaveManifest(report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func redactedSnapshotWarnings(report model.SnapshotReport) model.SnapshotReport {
	report.Warnings = redactedWarnings(report.Warnings)
	for i := range report.Resources {
		report.Resources[i].Warnings = redactedWarnings(report.Resources[i].Warnings)
	}
	return report
}

func redactedWarnings(warnings []model.Warning) []model.Warning {
	out := make([]model.Warning, len(warnings))
	for i, warning := range warnings {
		warning.Message, _ = security.RedactContent(warning.Message)
		out[i] = warning
	}
	return out
}
