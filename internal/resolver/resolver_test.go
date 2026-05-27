package resolver_test

import (
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/resolver"
)

func TestResolveOursMarksConflictResolvedAndLearnsResolution(t *testing.T) {
	state := syncState(conflict("conflict-001"))
	learned := learnedReport()

	result, err := resolver.Resolve(resolver.Input{
		State:      state,
		Learned:    learned,
		ConflictID: "conflict-001",
		Decision:   model.ResolutionDecisionOurs,
		ResolvedAt: "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	resolved := result.State.Conflicts[0]
	if resolved.Status != model.ConflictStatusResolved || resolved.RequiresUserDecision || resolved.ResolutionID != "resolution-001" {
		t.Fatalf("resolved conflict = %#v", resolved)
	}
	if result.State.Summary.OpenConflicts != 0 || result.State.Summary.ResolvedConflicts != 1 {
		t.Fatalf("summary = %#v", result.State.Summary)
	}
	resolution := result.Learned.Resolutions[0]
	if resolution.ID != "resolution-001" || resolution.Decision != model.ResolutionDecisionOurs || resolution.Value != "ours text" || resolution.ConflictFingerprint != "fingerprint-001" {
		t.Fatalf("learned resolution = %#v", resolution)
	}
}

func TestResolveTheirsUsesTheirsValue(t *testing.T) {
	result, err := resolver.Resolve(resolver.Input{
		State:      syncState(conflict("conflict-001")),
		Learned:    learnedReport(),
		ConflictID: "conflict-001",
		Decision:   model.ResolutionDecisionTheirs,
		ResolvedAt: "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got := result.Learned.Resolutions[0].Value; got != "theirs text" {
		t.Fatalf("resolution value = %q, want theirs text", got)
	}
}

func TestResolveAcceptSuggestionRequiresSuggestion(t *testing.T) {
	baseConflict := conflict("conflict-001")
	baseConflict.Suggestion = "merged suggestion"
	result, err := resolver.Resolve(resolver.Input{
		State:      syncState(baseConflict),
		Learned:    learnedReport(),
		ConflictID: "conflict-001",
		Decision:   model.ResolutionDecisionSuggestion,
		ResolvedAt: "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got := result.Learned.Resolutions[0].Value; got != "merged suggestion" {
		t.Fatalf("resolution value = %q, want suggestion", got)
	}

	_, err = resolver.Resolve(resolver.Input{
		State:      syncState(conflict("conflict-001")),
		Learned:    learnedReport(),
		ConflictID: "conflict-001",
		Decision:   model.ResolutionDecisionSuggestion,
	})
	if err == nil || !strings.Contains(err.Error(), "suggestion") {
		t.Fatalf("Resolve missing suggestion error = %v", err)
	}
}

func TestResolveManualRequiresValue(t *testing.T) {
	result, err := resolver.Resolve(resolver.Input{
		State:       syncState(conflict("conflict-001")),
		Learned:     learnedReport(),
		ConflictID:  "conflict-001",
		Decision:    model.ResolutionDecisionManual,
		ManualValue: "manual merged value",
		ResolvedAt:  "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got := result.Learned.Resolutions[0].Value; got != "manual merged value" {
		t.Fatalf("resolution value = %q, want manual value", got)
	}

	_, err = resolver.Resolve(resolver.Input{
		State:      syncState(conflict("conflict-001")),
		Learned:    learnedReport(),
		ConflictID: "conflict-001",
		Decision:   model.ResolutionDecisionManual,
	})
	if err == nil || !strings.Contains(err.Error(), "manual") {
		t.Fatalf("Resolve missing manual error = %v", err)
	}
}

func TestResolveRedactsSecretResolutionValues(t *testing.T) {
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	manual, err := resolver.Resolve(resolver.Input{
		State:       syncState(conflict("conflict-001")),
		Learned:     learnedReport(),
		ConflictID:  "conflict-001",
		Decision:    model.ResolutionDecisionManual,
		ManualValue: "GITHUB_TOKEN=" + secret,
		ResolvedAt:  "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve manual returned error: %v", err)
	}
	if value := manual.Learned.Resolutions[0].Value; strings.Contains(value, secret) || !strings.Contains(value, "<REDACTED>") {
		t.Fatalf("manual resolution value = %q, want redacted", value)
	}

	suggestionConflict := conflict("conflict-001")
	suggestionConflict.Suggestion = "use " + secret
	suggestion, err := resolver.Resolve(resolver.Input{
		State:      syncState(suggestionConflict),
		Learned:    learnedReport(),
		ConflictID: "conflict-001",
		Decision:   model.ResolutionDecisionSuggestion,
		ResolvedAt: "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve suggestion returned error: %v", err)
	}
	if value := suggestion.Learned.Resolutions[0].Value; strings.Contains(value, secret) || !strings.Contains(value, "<REDACTED>") {
		t.Fatalf("suggestion resolution value = %q, want redacted", value)
	}
}

func TestResolveRejectsUnknownOrResolvedConflicts(t *testing.T) {
	_, err := resolver.Resolve(resolver.Input{State: syncState(conflict("conflict-001")), Learned: learnedReport(), ConflictID: "missing", Decision: model.ResolutionDecisionOurs})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown conflict error = %v", err)
	}

	resolvedConflict := conflict("conflict-001")
	resolvedConflict.Status = model.ConflictStatusResolved
	_, err = resolver.Resolve(resolver.Input{State: syncState(resolvedConflict), Learned: learnedReport(), ConflictID: "conflict-001", Decision: model.ResolutionDecisionOurs})
	if err == nil || !strings.Contains(err.Error(), "already resolved") {
		t.Fatalf("resolved conflict error = %v", err)
	}
}

func TestResolveValidatesSideAvailabilityAndDecision(t *testing.T) {
	withoutOurs := conflict("conflict-001")
	withoutOurs.Ours = nil
	_, err := resolver.Resolve(resolver.Input{State: syncState(withoutOurs), Learned: learnedReport(), ConflictID: "conflict-001", Decision: model.ResolutionDecisionOurs})
	if err == nil || !strings.Contains(err.Error(), "ours") {
		t.Fatalf("missing ours error = %v", err)
	}

	_, err = resolver.Resolve(resolver.Input{State: syncState(conflict("conflict-001")), Learned: learnedReport(), ConflictID: "conflict-001", Decision: "invalid"})
	if err == nil || !strings.Contains(err.Error(), "decision") {
		t.Fatalf("invalid decision error = %v", err)
	}
}

func TestResolveAppendsDeterministicResolutionIDs(t *testing.T) {
	learned := learnedReport()
	learned.Resolutions = append(learned.Resolutions, model.LearnedResolution{ID: "resolution-001"})

	result, err := resolver.Resolve(resolver.Input{
		State:      syncState(conflict("conflict-002")),
		Learned:    learned,
		ConflictID: "conflict-002",
		Decision:   model.ResolutionDecisionTheirs,
		ResolvedAt: "2026-05-27T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if got := result.Learned.Resolutions[1].ID; got != "resolution-002" {
		t.Fatalf("resolution ID = %q, want resolution-002", got)
	}
	if result.Resolution.ID != "resolution-002" {
		t.Fatalf("result resolution = %#v", result.Resolution)
	}
}

func syncState(conflicts ...model.Conflict) model.SyncStateReport {
	state := model.SyncStateReport{
		SchemaVersion: model.SyncStateSchemaVersion,
		Project:       "/project",
		Source:        "claude",
		Target:        "codex",
		Conflicts:     conflicts,
	}
	for _, conflict := range conflicts {
		if conflict.Status == model.ConflictStatusResolved {
			state.Summary.ResolvedConflicts++
		} else {
			state.Summary.OpenConflicts++
		}
	}
	return state
}

func learnedReport() model.LearnedResolutionReport {
	return model.LearnedResolutionReport{SchemaVersion: model.LearnedResolutionsSchemaVersion, Project: "/project"}
}

func conflict(id string) model.Conflict {
	return model.Conflict{
		ID:                   id,
		Kind:                 model.ConflictKindContent,
		ResourceID:           "instruction:project-claude-md",
		ResourceKind:         model.KindInstruction,
		Scope:                model.ScopeProject,
		Base:                 &model.ResourceState{ID: "instruction:project-claude-md", Kind: model.KindInstruction, ContentHash: "base", NormalizedText: "base text"},
		Ours:                 &model.ResourceState{ID: "instruction:project-claude-md", Kind: model.KindInstruction, ContentHash: "ours", NormalizedText: "ours text"},
		Theirs:               &model.ResourceState{ID: "instruction:project-claude-md", Kind: model.KindInstruction, ContentHash: "theirs", NormalizedText: "theirs text"},
		RequiresUserDecision: true,
		Status:               model.ConflictStatusOpen,
		Fingerprint:          "fingerprint-001",
	}
}
