package app

import (
	"errors"
	"io"
	"os"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runStatus(opts cli.Options, stdout io.Writer) error {
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	report := model.StatusReport{
		SchemaVersion: model.StatusSchemaVersion,
		Project:       opts.Project,
		WorkspaceRoot: layout.Root,
		BaseSnapshots: map[string]bool{},
		Warnings:      []model.Warning{},
	}

	var manifest model.WorkspaceManifestReport
	if err := layout.LoadManifest(&manifest); err != nil {
		if !errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "read workspace manifest: %w", err)
		}
	} else {
		report.Initialized = true
		report.ManifestPath = layout.Manifest
		report.Summary.HasManifest = true
	}

	baseClaude, err := statusFileExists(layout.BaseClaude)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	baseCodex, err := statusFileExists(layout.BaseCodex)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	baseCanon, err := statusFileExists(layout.BaseCanon)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	report.BaseSnapshots["claude"] = baseClaude
	report.BaseSnapshots["codex"] = baseCodex
	report.BaseSnapshots["canon"] = baseCanon
	report.Summary.HasBaseClaude = baseClaude
	report.Summary.HasBaseCodex = baseCodex
	report.Summary.HasBaseCanon = baseCanon

	var state model.SyncStateReport
	if err := layout.LoadSyncState(&state); err != nil {
		if !errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "read sync state: %w", err)
		}
	} else {
		report.SyncStatePath = layout.SyncState
		report.Summary.HasSyncState = true
		report.Summary.OpenConflicts = state.Summary.OpenConflicts
		report.Summary.ResolvedConflicts = state.Summary.ResolvedConflicts
	}
	report.Summary.Warnings = len(report.Warnings)

	if opts.Format == "json" {
		if err := render.StatusJSON(stdout, report); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.StatusText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func statusFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !info.IsDir(), nil
}
