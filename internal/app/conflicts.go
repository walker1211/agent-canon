package app

import (
	"errors"
	"io"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runConflicts(opts cli.Options, stdout io.Writer) error {
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
	if opts.Format == "json" {
		if err := render.SyncJSON(stdout, state); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.ConflictsText(stdout, state); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}
