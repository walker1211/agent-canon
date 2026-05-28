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

func runImport(opts cli.Options, stdout io.Writer) error {
	switch opts.ImportTarget {
	case "codex":
		return runImportCodex(opts, stdout)
	default:
		return withExitCode(1, "unsupported import target %q", opts.ImportTarget)
	}
}

func runImportCodex(opts cli.Options, stdout io.Writer) error {
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
	current.Codex = redactedSnapshotWarnings(current.Codex)

	if err := ensureImportManifest(opts, layout); err != nil {
		return err
	}
	if err := layout.SaveBaseCodex(current.Codex); err != nil {
		return withExitCode(1, "%w", err)
	}

	scanWarnings := redactedWarnings(scanReport.Warnings)
	warnings := appendWarnings(scanWarnings, current.Codex.Warnings...)
	report := model.ImportReport{
		SchemaVersion: model.ImportSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:       opts.Project,
		Tool:          "codex",
		WorkspaceRoot: layout.Root,
		SnapshotPath:  layout.BaseCodex,
		ReportPath:    layout.ImportCodex,
		Summary: model.ImportSummary{
			Resources: len(current.Codex.Resources),
			Warnings:  len(warnings),
		},
		Warnings: warnings,
	}
	if err := layout.SaveImportCodex(report); err != nil {
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
