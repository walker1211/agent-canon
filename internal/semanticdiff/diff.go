package semanticdiff

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

type Input struct {
	BaseClaude    model.SnapshotReport
	BaseCodex     model.SnapshotReport
	CurrentClaude model.SnapshotReport
	CurrentCodex  model.SnapshotReport
	Learned       model.LearnedResolutionReport
}

type Result struct {
	Diffs     []model.SemanticDiff
	Conflicts []model.Conflict
}

func Compare(input Input) Result {
	baseClaude := stateMap(input.BaseClaude.Resources)
	baseCodex := stateMap(input.BaseCodex.Resources)
	currentClaude := stateMap(input.CurrentClaude.Resources)
	currentCodex := stateMap(input.CurrentCodex.Resources)
	learned := learnedSuggestions(input.Learned.Resolutions)

	var result Result
	ids := sortedIDs(baseClaude, baseCodex, currentClaude, currentCodex)
	for _, id := range ids {
		bc := baseClaude[id]
		bt := baseCodex[id]
		ours := currentClaude[id]
		theirs := currentCodex[id]
		changedOurs := stateChanged(bc, ours)
		changedTheirs := stateChanged(bt, theirs)
		if isNonActionableDiff(bc, bt, ours, theirs) {
			continue
		}
		if changedOurs || changedTheirs {
			result.Diffs = append(result.Diffs, buildDiff(id, bc, bt, ours, theirs))
		}

		kind, ok := classifyConflict(changedOurs, changedTheirs, bc, bt, ours, theirs)
		if !ok {
			continue
		}
		conflict := buildConflict(kind, id, bc, bt, ours, theirs)
		if suggestion, ok := learned[conflict.Fingerprint]; ok {
			conflict.Suggestion = suggestion
			conflict.SuggestionConfidence = 1
		}
		result.Conflicts = append(result.Conflicts, conflict)
	}

	for i := range result.Conflicts {
		result.Conflicts[i].ID = fmt.Sprintf("conflict-%03d", i+1)
	}
	return result
}

func stateMap(states []model.ResourceState) map[string]*model.ResourceState {
	out := make(map[string]*model.ResourceState, len(states))
	for i := range states {
		state := states[i]
		out[state.ID] = &state
	}
	return out
}

func sortedIDs(maps ...map[string]*model.ResourceState) []string {
	seen := map[string]bool{}
	for _, values := range maps {
		for id := range values {
			seen[id] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func stateChanged(base *model.ResourceState, current *model.ResourceState) bool {
	if base == nil || current == nil {
		return base != current
	}
	return stateSignature(base) != stateSignature(current)
}

func stateSignature(state *model.ResourceState) string {
	return strings.Join([]string{state.ContentHash, string(state.Status), state.Strategy}, "\n")
}

func buildDiff(id string, baseClaude *model.ResourceState, baseCodex *model.ResourceState, ours *model.ResourceState, theirs *model.ResourceState) model.SemanticDiff {
	state := firstState(ours, theirs, baseClaude, baseCodex)
	diffKind := model.DiffKindChanged
	if baseClaude == nil && baseCodex == nil {
		diffKind = model.DiffKindAdded
	} else if ours == nil && theirs == nil {
		diffKind = model.DiffKindRemoved
	}
	return model.SemanticDiff{
		ResourceID: id,
		Kind:       state.Kind,
		Scope:      state.Scope,
		DiffKind:   diffKind,
		BaseHash:   firstHash(baseClaude, baseCodex),
		OursHash:   hashOf(ours),
		TheirsHash: hashOf(theirs),
		Summary:    fmt.Sprintf("%s %s", id, diffKind),
	}
}

func isNonActionableDiff(baseClaude *model.ResourceState, baseCodex *model.ResourceState, ours *model.ResourceState, theirs *model.ResourceState) bool {
	return currentStatesEquivalent(ours, theirs) || isSkippedSession(baseClaude, baseCodex, ours, theirs) || isPluginAdaptation(baseClaude, baseCodex, ours, theirs) || isAgentsAggregateTargetDrift(baseClaude, ours, theirs)
}

func isAgentsAggregateTargetDrift(baseClaude *model.ResourceState, ours *model.ResourceState, theirs *model.ResourceState) bool {
	if sourceContentChanged(baseClaude, ours) || ours == nil || theirs == nil {
		return false
	}
	return isAgentsAggregateState(ours) && isAgentsAggregateState(theirs) && filepath.Base(theirs.Path) == "AGENTS.md"
}

func sourceContentChanged(base *model.ResourceState, current *model.ResourceState) bool {
	if base == nil || current == nil {
		return base != current
	}
	return hashOf(base) != hashOf(current)
}

func isAgentsAggregateState(state *model.ResourceState) bool {
	if state.Kind != model.KindInstruction && state.Kind != model.KindRule {
		return false
	}
	switch state.Strategy {
	case "append-to-agents-md", "merge-section-into-agents-md", "merge-rule-into-agents-md", "review-path-scoped-rule":
		return true
	default:
		return false
	}
}

func classifyConflict(changedOurs bool, changedTheirs bool, states ...*model.ResourceState) (model.ConflictKind, bool) {
	if !changedOurs && !changedTheirs {
		return "", false
	}
	ours := states[2]
	theirs := states[3]
	if currentStatesEquivalent(ours, theirs) || isSkippedSession(states...) || isCurrentReviewPathScopedRule(ours, theirs) {
		return "", false
	}
	if hasSecurityRisk(states...) {
		return model.ConflictKindSecurity, true
	}
	if hasCapabilityRisk(states...) {
		return model.ConflictKindCapability, true
	}
	if changedOurs && changedTheirs && ours != nil && theirs != nil && hashOf(ours) != hashOf(theirs) {
		return model.ConflictKindContent, true
	}
	return "", false
}

func currentStatesEquivalent(ours *model.ResourceState, theirs *model.ResourceState) bool {
	if ours == nil || theirs == nil {
		return false
	}
	return ours.Kind == theirs.Kind && ours.Scope == theirs.Scope && stateSignature(ours) == stateSignature(theirs)
}

func isSkippedSession(states ...*model.ResourceState) bool {
	found := false
	for _, state := range states {
		if state == nil {
			continue
		}
		if state.Kind != model.KindSession || state.Strategy != "skip-session-migration" {
			return false
		}
		found = true
	}
	return found
}

func isPluginAdaptation(states ...*model.ResourceState) bool {
	found := false
	for _, state := range states {
		if state == nil {
			continue
		}
		if !strings.HasPrefix(state.ID, "plugin:") || state.Kind != model.KindConfig || state.Strategy != "review-plugin-adaptation" {
			return false
		}
		found = true
	}
	return found
}

func isCurrentReviewPathScopedRule(states ...*model.ResourceState) bool {
	found := false
	for _, state := range states {
		if state == nil {
			continue
		}
		if state.Kind != model.KindRule || state.Strategy != "review-path-scoped-rule" {
			return false
		}
		found = true
	}
	return found
}

func hasSecurityRisk(states ...*model.ResourceState) bool {
	for _, state := range states {
		if state == nil {
			continue
		}
		if state.Status == model.StatusDangerous {
			return true
		}
		for _, warning := range state.Warnings {
			if warning.Code == "secret-redacted" {
				return true
			}
		}
	}
	return false
}

func hasCapabilityRisk(states ...*model.ResourceState) bool {
	for _, state := range states {
		if state == nil {
			continue
		}
		if state.Status == model.StatusUnsupported || state.Kind == model.KindHook || state.Kind == model.KindSession {
			return true
		}
	}
	return false
}

func buildConflict(kind model.ConflictKind, id string, baseClaude *model.ResourceState, baseCodex *model.ResourceState, ours *model.ResourceState, theirs *model.ResourceState) model.Conflict {
	state := firstState(ours, theirs, baseClaude, baseCodex)
	conflict := model.Conflict{
		Kind:                 kind,
		ResourceID:           id,
		ResourceKind:         state.Kind,
		Scope:                state.Scope,
		Base:                 cloneState(firstState(baseClaude, baseCodex)),
		Ours:                 cloneState(ours),
		Theirs:               cloneState(theirs),
		RequiresUserDecision: true,
		Status:               model.ConflictStatusOpen,
		Warnings:             combinedWarnings(baseClaude, baseCodex, ours, theirs),
	}
	conflict.Fingerprint = conflictFingerprint(conflict)
	return conflict
}

func firstState(states ...*model.ResourceState) *model.ResourceState {
	for _, state := range states {
		if state != nil {
			return state
		}
	}
	return &model.ResourceState{}
}

func firstHash(states ...*model.ResourceState) string {
	for _, state := range states {
		if hash := hashOf(state); hash != "" {
			return hash
		}
	}
	return ""
}

func hashOf(state *model.ResourceState) string {
	if state == nil {
		return ""
	}
	return state.ContentHash
}

func cloneState(state *model.ResourceState) *model.ResourceState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.Warnings = append([]model.Warning{}, state.Warnings...)
	return &clone
}

func combinedWarnings(states ...*model.ResourceState) []model.Warning {
	var out []model.Warning
	seen := map[string]bool{}
	for _, state := range states {
		if state == nil {
			continue
		}
		for _, warning := range state.Warnings {
			key := warning.Code + "\x00" + warning.Message
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, warning)
		}
	}
	return out
}

func conflictFingerprint(conflict model.Conflict) string {
	identity := strings.Join([]string{
		string(conflict.Kind),
		conflict.ResourceID,
		string(conflict.ResourceKind),
		hashOf(conflict.Base),
		hashOf(conflict.Ours),
		hashOf(conflict.Theirs),
	}, "\n")
	hash := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(hash[:])
}

func learnedSuggestions(resolutions []model.LearnedResolution) map[string]string {
	out := map[string]string{}
	for _, resolution := range resolutions {
		if resolution.ConflictFingerprint == "" {
			continue
		}
		if resolution.Value != "" {
			out[resolution.ConflictFingerprint] = resolution.Value
			continue
		}
		if resolution.Decision != "" {
			out[resolution.ConflictFingerprint] = string(resolution.Decision)
		}
	}
	return out
}
