package app

import (
	"errors"
	"io"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/exporter"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

type compilePreviewBuilder func(model.ScanReport, model.PlanReport) (exporter.CodexPreview, error)

func runCompile(opts cli.Options, stdout io.Writer) error {
	switch opts.CompileTarget {
	case "codex":
		return runCompilePreview(opts, stdout, "codex", exporter.BuildCodexPreview)
	case "claude":
		return runCompilePreview(opts, stdout, "claude", exporter.BuildClaudePreview)
	default:
		return withExitCode(1, "unsupported compile target %q", opts.CompileTarget)
	}
}

func runCompilePreview(opts cli.Options, stdout io.Writer, target string, buildPreview compilePreviewBuilder) error {
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	var canon model.CanonSnapshotReport
	if err := layout.LoadBaseCanon(&canon); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "compile %s requires canon baseline; run \"agent-canon sync claude codex\" first", target)
		}
		return withExitCode(1, "%w", err)
	}

	var state model.SyncStateReport
	if err := layout.LoadSyncState(&state); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "compile %s requires sync state; run \"agent-canon sync claude codex\" first", target)
		}
		return withExitCode(1, "%w", err)
	}
	if open := openConflictCount(state); open > 0 {
		return openConflictBlockerError("compile "+target, open)
	}
	if err := validateExportOutputRoot(opts); err != nil {
		return withExitCode(1, "%w", err)
	}

	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	planReport := planner.Build(scanReport)
	preview, err := buildPreview(scanReport, planReport)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := exporter.WritePreview(opts.Out, preview); err != nil {
		return withExitCode(1, "%w", err)
	}
	for _, line := range []struct {
		format string
		args   []any
	}{
		{format: "agent-canon compile %s", args: []any{target}},
		{format: "Project: %s", args: []any{opts.Project}},
		{format: "Workspace: %s", args: []any{layout.Root}},
		{format: "Canon snapshot: %s", args: []any{layout.BaseCanon}},
		{format: "Sync state: %s", args: []any{layout.SyncState}},
		{format: "Output: %s", args: []any{opts.Out}},
		{format: "Summary: files=%d", args: []any{len(preview.Files)}},
	} {
		if err := writeLine(stdout, line.format, line.args...); err != nil {
			return withExitCode(1, "%w", err)
		}
	}
	return nil
}
