package app

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runRollback(opts cli.Options, stdin io.Reader, stdout io.Writer) error {
	if opts.Format != "text" {
		return withExitCode(1, "--format json is not supported for rollback")
	}
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	var manifest model.RollbackManifestReport
	manifestPath, err := layout.LoadRollbackManifest(opts.RollbackID, &manifest)
	if err != nil {
		return withExitCode(1, "rollback manifest %s: %w", opts.RollbackID, err)
	}

	planned, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: opts.Project, CodexHome: opts.CodexHome, IncludeGlobal: opts.Global, DryRun: true})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	target := manifest.Target
	if target == "" {
		target = "codex"
	}
	baseReport := render.RollbackTextReport{Target: target, Project: opts.Project, BackupDir: manifest.BackupDir, ManifestPath: manifestPath, Warnings: manifest.Warnings}
	if opts.DryRun {
		baseReport.Mode = "dry-run"
		baseReport.Changes = planned.Changes
		return renderRollback(stdout, baseReport)
	}

	if !opts.Yes {
		baseReport.Mode = "planned"
		baseReport.Changes = planned.Changes
		if err := renderRollback(stdout, baseReport); err != nil {
			return err
		}
		confirmed, err := confirmRollback(stdin, stdout)
		if err != nil {
			return withExitCode(1, "%w", err)
		}
		if !confirmed {
			return withExitCode(1, "rollback cancelled")
		}
	}

	result, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: opts.Project, CodexHome: opts.CodexHome, IncludeGlobal: opts.Global})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	if _, err := refreshBaselineAfterFilesystemChange(opts, layout, manifest.Warnings); err != nil {
		return withExitCode(1, "%w", err)
	}
	baseReport.Mode = "applied"
	baseReport.Changes = result.Changes
	return renderRollback(stdout, baseReport)
}

func renderRollback(stdout io.Writer, report render.RollbackTextReport) error {
	if err := render.RollbackText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func confirmRollback(stdin io.Reader, stdout io.Writer) (bool, error) {
	if stdin == nil {
		return false, fmt.Errorf("rollback confirmation requires stdin; rerun with --yes to skip the prompt")
	}
	if _, err := io.WriteString(stdout, "Roll back these changes? [y/N]: "); err != nil {
		return false, fmt.Errorf("write confirmation prompt: %w", err)
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
