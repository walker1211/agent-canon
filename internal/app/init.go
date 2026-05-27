package app

import (
	"errors"
	"io"
	"time"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runInit(opts cli.Options, stdout io.Writer) error {
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	report := model.WorkspaceManifestReport{
		SchemaVersion: model.WorkspaceManifestSchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
		Project:       opts.Project,
		Source:        opts.From,
		Target:        opts.To,
		WorkspaceRoot: layout.Root,
		Warnings:      []model.Warning{},
	}

	var existing model.WorkspaceManifestReport
	if err := layout.LoadManifest(&existing); err != nil {
		if !errors.Is(err, workspace.ErrNotFound) {
			return withExitCode(1, "read workspace manifest: %w", err)
		}
	} else if existing.CreatedAt != "" {
		report.CreatedAt = existing.CreatedAt
	}

	if err := layout.SaveManifest(report); err != nil {
		return withExitCode(1, "%w", err)
	}
	if opts.Format == "json" {
		if err := render.InitJSON(stdout, report); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.InitText(stdout, report, layout.Manifest); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}
