package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
	"github.com/zhangyoujun/agent-canon/internal/snapshot"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runApply(opts cli.Options, stdin io.Reader, stdout io.Writer) error {
	switch opts.ApplyTarget {
	case "codex":
		return runApplyCodex(opts, stdin, stdout)
	case "claude":
		return runApplyClaude(opts, stdin, stdout)
	default:
		return withExitCode(1, "unsupported apply target %q", opts.ApplyTarget)
	}
}

func runApplyCodex(opts cli.Options, stdin io.Reader, stdout io.Writer) error {
	if opts.Format != "text" {
		return withExitCode(1, "--format json is not supported for apply codex")
	}
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	var state model.SyncStateReport
	if err := layout.LoadSyncState(&state); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "no sync state found; run \"agent-canon sync claude codex\" first")
		}
		return withExitCode(1, "%w", err)
	}
	if open := openConflictCount(state); open > 0 {
		return withExitCode(1, "apply codex blocked by %d open conflicts; run \"agent-canon conflicts\" and resolve them first", open)
	}

	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	planReport := planner.Build(scanReport)
	codexPlan, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scanReport, Plan: planReport, IncludeGlobal: opts.Global})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	plannedChanges := applyPlanChanges(codexPlan.Changes)
	if opts.DryRun {
		return renderApply(stdout, render.ApplyTextReport{Target: "codex", Project: opts.Project, Mode: "dry-run", Changes: plannedChanges, Warnings: codexPlan.Warnings})
	}

	if !opts.Yes {
		if err := renderApply(stdout, render.ApplyTextReport{Target: "codex", Project: opts.Project, Mode: "planned", Changes: plannedChanges, Warnings: codexPlan.Warnings}); err != nil {
			return err
		}
		confirmed, err := confirmApply(stdin, stdout)
		if err != nil {
			return withExitCode(1, "%w", err)
		}
		if !confirmed {
			return withExitCode(1, "apply cancelled")
		}
	}

	applyName := "apply-" + time.Now().UTC().Format("20060102T150405000000000Z")
	backupDir, err := layout.BackupDir(applyName)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	result, err := applypkg.WriteCodexPlan(applypkg.WriteInput{Plan: codexPlan, Project: opts.Project, CodexHome: opts.CodexHome, BackupDir: backupDir})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	baseSnapshots, err := refreshBaselineAfterFilesystemChange(opts, layout, codexPlan.Warnings)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	manifestPath, err := layout.SaveRollbackManifest(applyName, model.RollbackManifestReport{
		SchemaVersion: model.RollbackManifestSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:       opts.Project,
		Target:        "codex",
		BackupDir:     backupDir,
		Changes:       result.Changes,
		BaseSnapshots: baseSnapshots,
		Warnings:      codexPlan.Warnings,
	})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	return renderApply(stdout, render.ApplyTextReport{Target: "codex", Project: opts.Project, Mode: "applied", BackupDir: backupDir, ManifestPath: manifestPath, Changes: result.Changes, Warnings: codexPlan.Warnings})
}

func runApplyClaude(opts cli.Options, stdin io.Reader, stdout io.Writer) error {
	if opts.Format != "text" {
		return withExitCode(1, "--format json is not supported for apply claude")
	}
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	var state model.SyncStateReport
	if err := layout.LoadSyncState(&state); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "no sync state found; run \"agent-canon sync claude codex\" first")
		}
		return withExitCode(1, "%w", err)
	}
	if open := openConflictCount(state); open > 0 {
		return withExitCode(1, "apply claude blocked by %d open conflicts; run \"agent-canon conflicts\" and resolve them first", open)
	}

	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	planReport := planner.Build(scanReport)
	claudePlan, err := applypkg.BuildClaudePlan(applypkg.ClaudePlanInput{Scan: scanReport, Plan: planReport, IncludeGlobal: opts.Global})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	plannedChanges := applyPlanChanges(claudePlan.Changes)
	if opts.DryRun {
		return renderApply(stdout, render.ApplyTextReport{Target: "claude", Project: opts.Project, Mode: "dry-run", Changes: plannedChanges, Warnings: claudePlan.Warnings})
	}

	if !opts.Yes {
		if err := renderApply(stdout, render.ApplyTextReport{Target: "claude", Project: opts.Project, Mode: "planned", Changes: plannedChanges, Warnings: claudePlan.Warnings}); err != nil {
			return err
		}
		confirmed, err := confirmApply(stdin, stdout)
		if err != nil {
			return withExitCode(1, "%w", err)
		}
		if !confirmed {
			return withExitCode(1, "apply cancelled")
		}
	}

	applyName := "apply-" + time.Now().UTC().Format("20060102T150405000000000Z")
	backupDir, err := layout.BackupDir(applyName)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	result, err := applypkg.WriteClaudePlan(applypkg.WriteClaudeInput{Plan: claudePlan, Project: opts.Project, ClaudeHome: opts.ClaudeHome, BackupDir: backupDir})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	baseSnapshots, err := refreshBaselineAfterFilesystemChange(opts, layout, claudePlan.Warnings)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	manifestPath, err := layout.SaveRollbackManifest(applyName, model.RollbackManifestReport{
		SchemaVersion: model.RollbackManifestSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:       opts.Project,
		Target:        "claude",
		BackupDir:     backupDir,
		Changes:       result.Changes,
		BaseSnapshots: baseSnapshots,
		Warnings:      claudePlan.Warnings,
	})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	return renderApply(stdout, render.ApplyTextReport{Target: "claude", Project: opts.Project, Mode: "applied", BackupDir: backupDir, ManifestPath: manifestPath, Changes: result.Changes, Warnings: claudePlan.Warnings})
}

func refreshBaselineAfterFilesystemChange(opts cli.Options, layout workspace.Layout, applyWarnings []model.Warning) (map[string]string, error) {
	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return nil, err
	}
	current, err := snapshot.Build(scanReport)
	if err != nil {
		return nil, err
	}
	if err := saveBaseSnapshots(layout, current); err != nil {
		return nil, err
	}
	baseSnapshots := baseSnapshotPaths(layout)
	warnings := appendWarnings(scanReport.Warnings, current.Claude.Warnings...)
	warnings = appendWarnings(warnings, current.Codex.Warnings...)
	warnings = appendWarnings(warnings, current.Canon.Warnings...)
	warnings = appendWarnings(warnings, applyWarnings...)
	state := model.SyncStateReport{
		SchemaVersion: model.SyncStateSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:       opts.Project,
		Source:        "claude",
		Target:        "codex",
		BaseSnapshots: baseSnapshots,
		Warnings:      warnings,
	}
	state.Summary = syncSummary(state)
	if err := layout.SaveSyncState(state); err != nil {
		return nil, err
	}
	return baseSnapshots, nil
}

func baseSnapshotPaths(layout workspace.Layout) map[string]string {
	return map[string]string{
		"claude": layout.BaseClaude,
		"codex":  layout.BaseCodex,
		"canon":  layout.BaseCanon,
	}
}

func renderApply(stdout io.Writer, report render.ApplyTextReport) error {
	if err := render.ApplyText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func confirmApply(stdin io.Reader, stdout io.Writer) (bool, error) {
	if stdin == nil {
		return false, fmt.Errorf("apply confirmation requires stdin; rerun with --yes to skip the prompt")
	}
	if _, err := io.WriteString(stdout, "Apply these changes? [y/N]: "); err != nil {
		return false, fmt.Errorf("write confirmation prompt: %w", err)
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func openConflictCount(state model.SyncStateReport) int {
	open := 0
	for _, conflict := range state.Conflicts {
		if conflict.Status != model.ConflictStatusResolved {
			open++
		}
	}
	return open
}

func applyPlanChanges(changes []applypkg.FileChange) []model.ApplyFileChange {
	out := make([]model.ApplyFileChange, 0, len(changes))
	for _, change := range changes {
		out = append(out, change.ApplyFileChange)
	}
	return out
}
