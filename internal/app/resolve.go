package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/resolver"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runResolve(opts cli.Options, stdout io.Writer) error {
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
	learned, err := loadLearnedResolutions(layout, opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	decision, err := resolveDecision(opts.ResolveDecision)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	result, err := resolver.Resolve(resolver.Input{
		State:       state,
		Learned:     learned,
		ConflictID:  opts.ConflictID,
		Decision:    decision,
		ManualValue: opts.ManualValue,
	})
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := layout.SaveSyncState(result.State); err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := layout.SaveLearnedResolutions(result.Learned); err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := render.ResolveText(stdout, opts.ConflictID, decision, result.Resolution.ID); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func resolveDecision(value string) (model.ResolutionDecision, error) {
	switch value {
	case "ours":
		return model.ResolutionDecisionOurs, nil
	case "theirs":
		return model.ResolutionDecisionTheirs, nil
	case "accept-suggestion":
		return model.ResolutionDecisionSuggestion, nil
	case "manual":
		return model.ResolutionDecisionManual, nil
	default:
		return "", fmt.Errorf("unsupported resolve decision %q", value)
	}
}
