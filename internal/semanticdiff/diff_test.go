package semanticdiff

import (
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestCompareReturnsNoDiffsForUnchangedResources(t *testing.T) {
	baseClaude := snapshotReport("claude", state("resource-a", "claude", "hash-a", model.StatusCompatible, nil))
	baseCodex := snapshotReport("codex", state("resource-a", "codex", "hash-a", model.StatusCompatible, nil))
	currentClaude := snapshotReport("claude", state("resource-a", "claude", "hash-a", model.StatusCompatible, nil))
	currentCodex := snapshotReport("codex", state("resource-a", "codex", "hash-a", model.StatusCompatible, nil))

	result := Compare(Input{BaseClaude: baseClaude, BaseCodex: baseCodex, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Diffs) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("Compare result = %#v, want no diffs or conflicts", result)
	}
}

func TestCompareReturnsNoConflictsForUnchangedRiskyResources(t *testing.T) {
	securityWarning := []model.Warning{{Code: "secret-redacted", Message: "redacted"}}
	baseClaude := snapshotReport("claude",
		stateWithKind("dangerous", model.KindCommand, "claude", "hash-secret", model.StatusDangerous, securityWarning),
		stateWithKind("hook", model.KindHook, "claude", "", model.StatusUnsupported, nil),
	)
	currentClaude := snapshotReport("claude",
		stateWithKind("dangerous", model.KindCommand, "claude", "hash-secret", model.StatusDangerous, securityWarning),
		stateWithKind("hook", model.KindHook, "claude", "", model.StatusUnsupported, nil),
	)

	result := Compare(Input{BaseClaude: baseClaude, CurrentClaude: currentClaude})

	if len(result.Diffs) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("Compare result = %#v, want no diffs or conflicts", result)
	}
}

func TestCompareReportsOneSidedChangeWithoutConflict(t *testing.T) {
	baseClaude := snapshotReport("claude", state("resource-a", "claude", "hash-a", model.StatusCompatible, nil))
	baseCodex := snapshotReport("codex", state("resource-a", "codex", "hash-a", model.StatusCompatible, nil))
	currentClaude := snapshotReport("claude", state("resource-a", "claude", "hash-b", model.StatusCompatible, nil))
	currentCodex := snapshotReport("codex", state("resource-a", "codex", "hash-a", model.StatusCompatible, nil))

	result := Compare(Input{BaseClaude: baseClaude, BaseCodex: baseCodex, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Diffs) != 1 {
		t.Fatalf("diffs = %#v, want one", result.Diffs)
	}
	if result.Diffs[0].DiffKind != model.DiffKindChanged || result.Diffs[0].ResourceID != "resource-a" {
		t.Fatalf("diff = %#v", result.Diffs[0])
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", result.Conflicts)
	}
}

func TestCompareCreatesContentConflictWhenBothSidesChangeDifferently(t *testing.T) {
	baseClaude := snapshotReport("claude", state("resource-a", "claude", "hash-base", model.StatusCompatible, nil))
	baseCodex := snapshotReport("codex", state("resource-a", "codex", "hash-base", model.StatusCompatible, nil))
	currentClaude := snapshotReport("claude", state("resource-a", "claude", "hash-ours", model.StatusCompatible, nil))
	currentCodex := snapshotReport("codex", state("resource-a", "codex", "hash-theirs", model.StatusCompatible, nil))

	result := Compare(Input{BaseClaude: baseClaude, BaseCodex: baseCodex, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Conflicts) != 1 {
		t.Fatalf("conflicts = %#v, want one", result.Conflicts)
	}
	conflict := result.Conflicts[0]
	if conflict.ID != "conflict-001" || conflict.Kind != model.ConflictKindContent || conflict.Status != model.ConflictStatusOpen {
		t.Fatalf("conflict = %#v", conflict)
	}
	if conflict.Ours == nil || conflict.Ours.ContentHash != "hash-ours" || conflict.Theirs == nil || conflict.Theirs.ContentHash != "hash-theirs" {
		t.Fatalf("conflict sides = %#v", conflict)
	}
	if conflict.Fingerprint == "" {
		t.Fatalf("conflict missing fingerprint: %#v", conflict)
	}
}

func TestCompareClassifiesSecurityAndCapabilityConflicts(t *testing.T) {
	securityWarning := []model.Warning{{Code: "secret-redacted", Message: "redacted"}}
	currentClaude := snapshotReport("claude",
		stateWithKind("dangerous", model.KindCommand, "claude", "hash-secret", model.StatusDangerous, securityWarning),
		stateWithKind("hook", model.KindHook, "claude", "", model.StatusUnsupported, nil),
	)

	result := Compare(Input{CurrentClaude: currentClaude})

	if len(result.Conflicts) != 2 {
		t.Fatalf("conflicts = %#v, want two", result.Conflicts)
	}
	if result.Conflicts[0].ID != "conflict-001" || result.Conflicts[0].Kind != model.ConflictKindSecurity || result.Conflicts[0].ResourceID != "dangerous" {
		t.Fatalf("first conflict = %#v", result.Conflicts[0])
	}
	if result.Conflicts[1].ID != "conflict-002" || result.Conflicts[1].Kind != model.ConflictKindCapability || result.Conflicts[1].ResourceID != "hook" {
		t.Fatalf("second conflict = %#v", result.Conflicts[1])
	}
}

func TestCompareIgnoresSkippedSessions(t *testing.T) {
	baseClaude := snapshotReport("claude", stateWithKindAndStrategy("session:global-removed", model.KindSession, "claude", "hash-old", model.StatusUnsupported, "skip-session-migration", nil))
	currentClaude := snapshotReport("claude", stateWithKindAndStrategy("session:global-added", model.KindSession, "claude", "hash-new", model.StatusUnsupported, "skip-session-migration", nil))

	result := Compare(Input{BaseClaude: baseClaude, CurrentClaude: currentClaude})

	if len(result.Diffs) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("Compare result = %#v, want no diffs or conflicts", result)
	}
}

func TestCompareIgnoresPluginAdaptationMetadata(t *testing.T) {
	baseClaude := snapshotReport("claude", stateWithKindAndStrategy("plugin:global-cache", model.KindConfig, "claude", "hash-old", model.StatusPartial, "review-plugin-adaptation", nil))
	currentClaude := snapshotReport("claude", stateWithKindAndStrategy("plugin:global-cache", model.KindConfig, "claude", "hash-new", model.StatusPartial, "review-plugin-adaptation", nil))

	result := Compare(Input{BaseClaude: baseClaude, CurrentClaude: currentClaude})

	if len(result.Diffs) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("Compare result = %#v, want no diffs or conflicts", result)
	}
}

func TestCompareIgnoresAgentsAggregateTargetDriftWhenSourceContentIsUnchanged(t *testing.T) {
	baseClaude := snapshotReport("claude", stateWithPath("rule:global-go", model.KindRule, "claude", "/claude/rules/go.md", "hash-rule", model.StatusCompatible, "merge-rule-into-agents-md", nil))
	currentClaude := snapshotReport("claude", stateWithPath("rule:global-go", model.KindRule, "claude", "/claude/rules/go.md", "hash-rule", model.StatusPartial, "review-path-scoped-rule", nil))
	currentCodex := snapshotReport("codex", stateWithPath("rule:global-go", model.KindRule, "codex", "/codex/AGENTS.md", "hash-agents", model.StatusPartial, "review-path-scoped-rule", nil))

	result := Compare(Input{BaseClaude: baseClaude, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Diffs) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("Compare result = %#v, want no diffs or conflicts", result)
	}
}

func TestCompareReportsAgentsAggregateDiffWhenSourceContentChanges(t *testing.T) {
	baseClaude := snapshotReport("claude", stateWithPath("rule:global-go", model.KindRule, "claude", "/claude/rules/go.md", "hash-old", model.StatusCompatible, "merge-rule-into-agents-md", nil))
	currentClaude := snapshotReport("claude", stateWithPath("rule:global-go", model.KindRule, "claude", "/claude/rules/go.md", "hash-new", model.StatusCompatible, "merge-rule-into-agents-md", nil))
	currentCodex := snapshotReport("codex", stateWithPath("rule:global-go", model.KindRule, "codex", "/codex/AGENTS.md", "hash-agents", model.StatusCompatible, "merge-rule-into-agents-md", nil))

	result := Compare(Input{BaseClaude: baseClaude, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Diffs) != 1 || result.Diffs[0].ResourceID != "rule:global-go" {
		t.Fatalf("diffs = %#v, want source content diff", result.Diffs)
	}
}

func TestCompareDoesNotConflictForReviewPathScopedRules(t *testing.T) {
	warning := []model.Warning{{Code: "secret-redacted", Message: "redacted"}}
	baseClaude := snapshotReport("claude", stateWithKindAndStrategy("rule:global-github-actions", model.KindRule, "claude", "hash-rule", model.StatusCompatible, "merge-rule-into-agents-md", warning))
	baseCodex := snapshotReport("codex", stateWithKindAndStrategy("rule:global-github-actions", model.KindRule, "codex", "hash-rule", model.StatusCompatible, "merge-rule-into-agents-md", nil))
	currentClaude := snapshotReport("claude", stateWithKindAndStrategy("rule:global-github-actions", model.KindRule, "claude", "hash-rule", model.StatusPartial, "review-path-scoped-rule", warning))
	currentCodex := snapshotReport("codex", stateWithKindAndStrategy("rule:global-github-actions", model.KindRule, "codex", "hash-agents", model.StatusPartial, "review-path-scoped-rule", nil))

	result := Compare(Input{BaseClaude: baseClaude, BaseCodex: baseCodex, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Diffs) != 1 {
		t.Fatalf("diffs = %#v, want one", result.Diffs)
	}
	if len(result.Conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", result.Conflicts)
	}
}

func TestCompareIgnoresConvergedCurrentStates(t *testing.T) {
	warning := []model.Warning{{Code: "secret-redacted", Message: "redacted"}}
	baseClaude := snapshotReport("claude", stateWithKindAndStrategy("skill:global-skill-creator", model.KindSkill, "claude", "hash-skill", model.StatusPartial, "convert-skill-with-review", warning))
	currentClaude := snapshotReport("claude", stateWithKindAndStrategy("skill:global-skill-creator", model.KindSkill, "claude", "hash-skill", model.StatusPartial, "convert-skill-with-review", warning))
	currentCodex := snapshotReport("codex", stateWithKindAndStrategy("skill:global-skill-creator", model.KindSkill, "codex", "hash-skill", model.StatusPartial, "convert-skill-with-review", warning))

	result := Compare(Input{BaseClaude: baseClaude, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Diffs) != 0 || len(result.Conflicts) != 0 {
		t.Fatalf("Compare result = %#v, want no diffs or conflicts", result)
	}
}

func TestCompareConflictIDsAreDeterministic(t *testing.T) {
	baseClaude := snapshotReport("claude", state("b", "claude", "base", model.StatusCompatible, nil), state("a", "claude", "base", model.StatusCompatible, nil))
	baseCodex := snapshotReport("codex", state("b", "codex", "base", model.StatusCompatible, nil), state("a", "codex", "base", model.StatusCompatible, nil))
	currentClaude := snapshotReport("claude", state("b", "claude", "ours-b", model.StatusCompatible, nil), state("a", "claude", "ours-a", model.StatusCompatible, nil))
	currentCodex := snapshotReport("codex", state("b", "codex", "theirs-b", model.StatusCompatible, nil), state("a", "codex", "theirs-a", model.StatusCompatible, nil))

	result := Compare(Input{BaseClaude: baseClaude, BaseCodex: baseCodex, CurrentClaude: currentClaude, CurrentCodex: currentCodex})

	if len(result.Conflicts) != 2 {
		t.Fatalf("conflicts = %#v, want two", result.Conflicts)
	}
	if result.Conflicts[0].ID != "conflict-001" || result.Conflicts[0].ResourceID != "a" {
		t.Fatalf("first conflict = %#v", result.Conflicts[0])
	}
	if result.Conflicts[1].ID != "conflict-002" || result.Conflicts[1].ResourceID != "b" {
		t.Fatalf("second conflict = %#v", result.Conflicts[1])
	}
}

func TestCompareAttachesLearnedResolutionSuggestionByFingerprint(t *testing.T) {
	input := Input{
		BaseClaude:    snapshotReport("claude", state("resource-a", "claude", "hash-base", model.StatusCompatible, nil)),
		BaseCodex:     snapshotReport("codex", state("resource-a", "codex", "hash-base", model.StatusCompatible, nil)),
		CurrentClaude: snapshotReport("claude", state("resource-a", "claude", "hash-ours", model.StatusCompatible, nil)),
		CurrentCodex:  snapshotReport("codex", state("resource-a", "codex", "hash-theirs", model.StatusCompatible, nil)),
	}
	first := Compare(input)
	fingerprint := first.Conflicts[0].Fingerprint
	input.Learned = model.LearnedResolutionReport{Resolutions: []model.LearnedResolution{{
		ConflictFingerprint: fingerprint,
		Decision:            model.ResolutionDecisionManual,
		Value:               "merged value",
	}}}

	result := Compare(input)

	if result.Conflicts[0].Suggestion != "merged value" || result.Conflicts[0].SuggestionConfidence != 1 {
		t.Fatalf("conflict suggestion = %#v", result.Conflicts[0])
	}
}

func snapshotReport(tool string, states ...model.ResourceState) model.SnapshotReport {
	return model.SnapshotReport{Tool: tool, Resources: states}
}

func state(id string, tool string, hash string, status model.Status, warnings []model.Warning) model.ResourceState {
	return stateWithKind(id, model.KindInstruction, tool, hash, status, warnings)
}

func stateWithKind(id string, kind model.ResourceKind, tool string, hash string, status model.Status, warnings []model.Warning) model.ResourceState {
	return stateWithKindAndStrategy(id, kind, tool, hash, status, "test-strategy", warnings)
}

func stateWithKindAndStrategy(id string, kind model.ResourceKind, tool string, hash string, status model.Status, strategy string, warnings []model.Warning) model.ResourceState {
	return stateWithPath(id, kind, tool, "", hash, status, strategy, warnings)
}

func stateWithPath(id string, kind model.ResourceKind, tool string, path string, hash string, status model.Status, strategy string, warnings []model.Warning) model.ResourceState {
	return model.ResourceState{
		ID:          id,
		Kind:        kind,
		Scope:       model.ScopeProject,
		Tool:        tool,
		Path:        path,
		Status:      status,
		Strategy:    strategy,
		ContentHash: hash,
		Warnings:    warnings,
	}
}
