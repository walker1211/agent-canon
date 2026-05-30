package resolver

import (
	"fmt"
	"strings"
	"time"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

type Input struct {
	State       model.SyncStateReport
	Learned     model.LearnedResolutionReport
	ConflictID  string
	Decision    model.ResolutionDecision
	ManualValue string
	ResolvedAt  string
}

type Result struct {
	State      model.SyncStateReport
	Learned    model.LearnedResolutionReport
	Resolution model.LearnedResolution
}

func Resolve(input Input) (Result, error) {
	state := cloneState(input.State)
	learned := cloneLearned(input.Learned, state.Project)
	index := -1
	for i, conflict := range state.Conflicts {
		if conflict.ID == input.ConflictID {
			index = i
			break
		}
	}
	if index < 0 {
		return Result{}, fmt.Errorf("conflict %s not found", input.ConflictID)
	}
	conflict := state.Conflicts[index]
	if conflict.Status == model.ConflictStatusResolved {
		return Result{}, fmt.Errorf("conflict %s is already resolved", input.ConflictID)
	}

	value, err := resolutionValue(conflict, input.Decision, input.ManualValue)
	if err != nil {
		return Result{}, err
	}
	value = redactResolutionValue(value)
	resolvedAt := input.ResolvedAt
	if resolvedAt == "" {
		resolvedAt = time.Now().UTC().Format(time.RFC3339)
	}
	resolution := model.LearnedResolution{
		ID:                  fmt.Sprintf("resolution-%03d", len(learned.Resolutions)+1),
		ConflictFingerprint: conflict.Fingerprint,
		ConflictKind:        conflict.Kind,
		ResourceID:          conflict.ResourceID,
		ResolvedAt:          resolvedAt,
		Decision:            input.Decision,
		Value:               value,
	}
	learned.Resolutions = append(learned.Resolutions, resolution)

	state.Conflicts[index].Status = model.ConflictStatusResolved
	state.Conflicts[index].RequiresUserDecision = false
	state.Conflicts[index].ResolutionID = resolution.ID
	state.Summary = summarize(state)
	return Result{State: state, Learned: learned, Resolution: resolution}, nil
}

func resolutionValue(conflict model.Conflict, decision model.ResolutionDecision, manualValue string) (string, error) {
	if conflict.Kind == model.ConflictKindConfigMerge && decision == model.ResolutionDecisionManual {
		return "", fmt.Errorf("manual TOML resolution is not supported for Codex MCP config merge conflicts")
	}

	switch decision {
	case model.ResolutionDecisionOurs:
		return valueFromState("ours", conflict.Ours)
	case model.ResolutionDecisionTheirs:
		return valueFromState("theirs", conflict.Theirs)
	case model.ResolutionDecisionSuggestion:
		if conflict.Suggestion == "" {
			return "", fmt.Errorf("conflict %s has no suggestion", conflict.ID)
		}
		return conflict.Suggestion, nil
	case model.ResolutionDecisionManual:
		if strings.TrimSpace(manualValue) == "" {
			return "", fmt.Errorf("manual resolution value is empty")
		}
		return manualValue, nil
	default:
		return "", fmt.Errorf("unsupported resolution decision %q", decision)
	}
}

func valueFromState(label string, state *model.ResourceState) (string, error) {
	if state == nil {
		return "", fmt.Errorf("%s state is missing", label)
	}
	if state.NormalizedText != "" {
		return state.NormalizedText, nil
	}
	return state.ContentHash, nil
}

func redactResolutionValue(value string) string {
	redacted, _ := security.RedactContent(value)
	return redacted
}

func cloneState(state model.SyncStateReport) model.SyncStateReport {
	state.Diffs = append([]model.SemanticDiff{}, state.Diffs...)
	state.Conflicts = append([]model.Conflict{}, state.Conflicts...)
	state.Warnings = append([]model.Warning{}, state.Warnings...)
	state.BaseSnapshots = cloneStringMap(state.BaseSnapshots)
	return state
}

func cloneLearned(report model.LearnedResolutionReport, project string) model.LearnedResolutionReport {
	if report.SchemaVersion == "" {
		report.SchemaVersion = model.LearnedResolutionsSchemaVersion
	}
	if report.Project == "" {
		report.Project = project
	}
	report.Resolutions = append([]model.LearnedResolution{}, report.Resolutions...)
	return report
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func summarize(state model.SyncStateReport) model.SyncSummary {
	summary := state.Summary
	summary.OpenConflicts = 0
	summary.ResolvedConflicts = 0
	for _, conflict := range state.Conflicts {
		if conflict.Status == model.ConflictStatusResolved {
			summary.ResolvedConflicts++
		} else {
			summary.OpenConflicts++
		}
	}
	return summary
}
