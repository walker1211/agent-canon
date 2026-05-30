package app

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/configmerge"
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
		conflicts = comparison.Conflicts
		configConflicts, configWarnings, err := detectCodexMCPConfigConflicts(scanReport)
		if err != nil {
			return withExitCode(1, "%w", err)
		}
		conflicts = append(conflicts, configConflicts...)
		conflicts = assignConflictIDsPreservingResolved(conflicts, previous.Conflicts)
		warnings = appendWarnings(warnings, configWarnings...)
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

func detectCodexMCPConfigConflicts(scanReport model.ScanReport) ([]model.Conflict, []model.Warning, error) {
	var conflicts []model.Conflict
	var warnings []model.Warning
	for _, targetPath := range []string{
		filepath.Join(scanReport.Project, ".codex", "config.toml"),
		filepath.Join(scanReport.CodexHome, "config.toml"),
	} {
		targetScan := scanReport
		targetScan.Resources = mcpResourcesForTarget(scanReport.Resources, targetPath)
		if len(targetScan.Resources) == 0 {
			continue
		}
		analysis, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: targetScan, TargetPath: targetPath})
		if err != nil {
			return nil, nil, err
		}
		conflicts = append(conflicts, analysis.Conflicts...)
		warnings = appendWarnings(warnings, analysis.Warnings...)
	}
	return conflicts, warnings, nil
}

func mcpResourcesForTarget(resources []model.Resource, targetPath string) []model.Resource {
	cleanTarget := filepath.Clean(targetPath)
	var out []model.Resource
	for _, resource := range resources {
		if resource.Kind != model.KindMCPServer || resource.Scope == model.ScopeLocal || resource.TargetPathHint == "" {
			continue
		}
		if filepath.Clean(resource.TargetPathHint) != cleanTarget {
			continue
		}
		out = append(out, resource)
	}
	return out
}

func assignConflictIDsPreservingResolved(current []model.Conflict, previous []model.Conflict) []model.Conflict {
	out := append([]model.Conflict{}, current...)
	resolved := map[string]model.Conflict{}
	for _, conflict := range previous {
		if conflict.Status == model.ConflictStatusResolved && conflict.Fingerprint != "" {
			resolved[conflict.Fingerprint] = conflict
		}
	}

	usedIDs := map[string]bool{}
	preservedID := make([]bool, len(out))
	for i := range out {
		previousConflict, ok := resolved[out[i].Fingerprint]
		if !ok {
			continue
		}
		out[i].Status = model.ConflictStatusResolved
		out[i].RequiresUserDecision = false
		out[i].ResolutionID = previousConflict.ResolutionID
		if previousConflict.ID == "" || usedIDs[previousConflict.ID] {
			continue
		}
		out[i].ID = previousConflict.ID
		usedIDs[previousConflict.ID] = true
		preservedID[i] = true
	}

	next := 1
	for i := range out {
		if preservedID[i] {
			continue
		}
		out[i].ID = nextConflictID(&next, usedIDs)
		usedIDs[out[i].ID] = true
	}
	return out
}

func nextConflictID(next *int, used map[string]bool) string {
	for {
		id := fmt.Sprintf("conflict-%03d", *next)
		(*next)++
		if !used[id] {
			return id
		}
	}
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
