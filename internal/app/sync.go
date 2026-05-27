package app

import (
	"errors"
	"io"
	"time"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
	"github.com/zhangyoujun/agent-canon/internal/semanticdiff"
	"github.com/zhangyoujun/agent-canon/internal/snapshot"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func runSync(opts cli.Options, stdout io.Writer) error {
	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	current, err := snapshot.Build(scanReport)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	layout, err := workspace.New(opts.Project)
	if err != nil {
		return withExitCode(1, "%w", err)
	}

	base, ok, err := loadBaseSnapshots(layout)
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

	warnings := appendWarnings(scanReport.Warnings, current.Claude.Warnings...)
	warnings = appendWarnings(warnings, current.Codex.Warnings...)
	warnings = appendWarnings(warnings, current.Canon.Warnings...)

	var diffs []model.SemanticDiff
	var conflicts []model.Conflict
	if !ok {
		base = current
		warnings = appendWarnings(warnings, model.Warning{Code: "base-snapshots-initialized", Message: "base snapshots initialized; rerun sync after changes to detect diffs"})
		if err := saveBaseSnapshots(layout, current); err != nil {
			return withExitCode(1, "%w", err)
		}
	} else {
		comparison := semanticdiff.Compare(semanticdiff.Input{
			BaseClaude:    base.Claude,
			BaseCodex:     base.Codex,
			CurrentClaude: current.Claude,
			CurrentCodex:  current.Codex,
			Learned:       learned,
		})
		diffs = comparison.Diffs
		conflicts = preserveResolvedConflicts(comparison.Conflicts, previous.Conflicts)
	}

	state := model.SyncStateReport{
		SchemaVersion: model.SyncStateSchemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Project:       opts.Project,
		Source:        opts.SyncSource,
		Target:        opts.SyncTarget,
		BaseSnapshots: map[string]string{
			"claude": layout.BaseClaude,
			"codex":  layout.BaseCodex,
			"canon":  layout.BaseCanon,
		},
		Diffs:     diffs,
		Conflicts: conflicts,
		Warnings:  warnings,
	}
	state.Summary = syncSummary(state)
	if err := layout.SaveSyncState(state); err != nil {
		return withExitCode(1, "%w", err)
	}

	if opts.Format == "json" {
		if err := render.SyncJSON(stdout, state); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.SyncText(stdout, state, layout.Root, layout.SyncState); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func loadBaseSnapshots(layout workspace.Layout) (snapshot.Set, bool, error) {
	var base snapshot.Set
	if err := layout.LoadBaseClaude(&base.Claude); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return snapshot.Set{}, false, nil
		}
		return snapshot.Set{}, false, err
	}
	if err := layout.LoadBaseCodex(&base.Codex); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return snapshot.Set{}, false, nil
		}
		return snapshot.Set{}, false, err
	}
	if err := layout.LoadBaseCanon(&base.Canon); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return snapshot.Set{}, false, nil
		}
		return snapshot.Set{}, false, err
	}
	return base, true, nil
}

func saveBaseSnapshots(layout workspace.Layout, set snapshot.Set) error {
	if err := layout.SaveBaseClaude(set.Claude); err != nil {
		return err
	}
	if err := layout.SaveBaseCodex(set.Codex); err != nil {
		return err
	}
	return layout.SaveBaseCanon(set.Canon)
}

func loadLearnedResolutions(layout workspace.Layout, project string) (model.LearnedResolutionReport, error) {
	report := model.LearnedResolutionReport{SchemaVersion: model.LearnedResolutionsSchemaVersion, Project: project}
	if err := layout.LoadLearnedResolutions(&report); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return report, nil
		}
		return model.LearnedResolutionReport{}, err
	}
	return report, nil
}

func loadPreviousSyncState(layout workspace.Layout) (model.SyncStateReport, error) {
	var state model.SyncStateReport
	if err := layout.LoadSyncState(&state); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			return model.SyncStateReport{}, nil
		}
		return model.SyncStateReport{}, err
	}
	return state, nil
}

func preserveResolvedConflicts(current []model.Conflict, previous []model.Conflict) []model.Conflict {
	resolved := map[string]model.Conflict{}
	for _, conflict := range previous {
		if conflict.Status == model.ConflictStatusResolved && conflict.Fingerprint != "" {
			resolved[conflict.Fingerprint] = conflict
		}
	}
	for i := range current {
		previousConflict, ok := resolved[current[i].Fingerprint]
		if !ok {
			continue
		}
		current[i].Status = model.ConflictStatusResolved
		current[i].RequiresUserDecision = false
		current[i].ResolutionID = previousConflict.ResolutionID
	}
	return current
}

func syncSummary(state model.SyncStateReport) model.SyncSummary {
	summary := model.SyncSummary{Diffs: len(state.Diffs), Warnings: len(state.Warnings)}
	for _, conflict := range state.Conflicts {
		switch conflict.Status {
		case model.ConflictStatusResolved:
			summary.ResolvedConflicts++
		default:
			summary.OpenConflicts++
		}
	}
	return summary
}

func appendWarnings(existing []model.Warning, additions ...model.Warning) []model.Warning {
	out := append([]model.Warning{}, existing...)
	seen := map[string]bool{}
	for _, warning := range out {
		seen[warning.Code+"\x00"+warning.Message] = true
	}
	for _, warning := range additions {
		key := warning.Code + "\x00" + warning.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, warning)
	}
	return out
}
