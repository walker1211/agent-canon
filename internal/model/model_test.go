package model

import (
	"encoding/json"
	"testing"
)

func TestScanReportMarshalsRequiredJSONKeys(t *testing.T) {
	report := ScanReport{
		SchemaVersion: ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       "/repo",
		ClaudeHome:    "/home/.claude",
		CodexHome:     "/home/.codex",
		Resources: []Resource{
			{
				ID:             "instruction:global-claude-md",
				Kind:           KindInstruction,
				Scope:          ScopeGlobal,
				SourceTool:     "claude",
				SourcePath:     "/home/.claude/CLAUDE.md",
				TargetTool:     "codex",
				TargetPathHint: "/home/.codex/AGENTS.md",
				Status:         StatusCompatible,
				Strategy:       "append-to-agents-md",
				Warnings:       []Warning{},
			},
		},
		Warnings: []Warning{},
		Summary: ScanSummary{
			Compatible:  1,
			Partial:     0,
			Unsupported: 0,
			Dangerous:   0,
		},
	}

	got := marshalToMap(t, report)

	assertString(t, got, "schemaVersion", "agent-canon.scan.v1")
	assertString(t, got, "source", "claude")
	assertString(t, got, "target", "codex")
	assertString(t, got, "project", "/repo")
	assertString(t, got, "claudeHome", "/home/.claude")
	assertString(t, got, "codexHome", "/home/.codex")
	assertHasKey(t, got, "resources")
	assertHasKey(t, got, "warnings")
	assertHasKey(t, got, "summary")

	resource := got["resources"].([]any)[0].(map[string]any)
	assertString(t, resource, "id", "instruction:global-claude-md")
	assertString(t, resource, "kind", "Instruction")
	assertString(t, resource, "scope", "global")
	assertString(t, resource, "sourceTool", "claude")
	assertString(t, resource, "sourcePath", "/home/.claude/CLAUDE.md")
	assertString(t, resource, "targetTool", "codex")
	assertString(t, resource, "targetPathHint", "/home/.codex/AGENTS.md")
	assertString(t, resource, "status", "compatible")
	assertString(t, resource, "strategy", "append-to-agents-md")
	assertHasKey(t, resource, "warnings")

	summary := got["summary"].(map[string]any)
	assertNumber(t, summary, "compatible", 1)
	assertNumber(t, summary, "partial", 0)
	assertNumber(t, summary, "unsupported", 0)
	assertNumber(t, summary, "dangerous", 0)
}

func TestResourceOmitsEmptyOptionalTargetFields(t *testing.T) {
	resource := Resource{
		ID:         "session:history",
		Kind:       KindSession,
		Scope:      ScopeGlobal,
		SourceTool: "claude",
		SourcePath: "/home/.claude/projects/history.jsonl",
		Status:     StatusUnsupported,
		Strategy:   "skip-session-migration",
		Warnings:   []Warning{},
	}

	got := marshalToMap(t, resource)

	assertMissingKey(t, got, "targetTool")
	assertMissingKey(t, got, "targetPathHint")
}

func TestPlanReportMarshalsRequiredJSONKeys(t *testing.T) {
	report := PlanReport{
		SchemaVersion: PlanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       "/repo",
		Operations: []Operation{
			{
				ID:             "op-001",
				Action:         "create-or-merge",
				ResourceID:     "instruction:project-claude-md",
				Kind:           KindInstruction,
				SourcePath:     "/repo/CLAUDE.md",
				TargetPath:     "/repo/AGENTS.md",
				Status:         StatusCompatible,
				Strategy:       "merge-section-into-agents-md",
				RequiresReview: false,
				Warnings:       []Warning{},
			},
		},
		Warnings: []Warning{},
		NonGoals: []string{},
		Summary: PlanSummary{
			Create:    0,
			Modify:    0,
			Skip:      0,
			Manual:    0,
			Dangerous: 0,
		},
	}

	got := marshalToMap(t, report)

	assertString(t, got, "schemaVersion", "agent-canon.plan.v1")
	assertString(t, got, "source", "claude")
	assertString(t, got, "target", "codex")
	assertString(t, got, "project", "/repo")
	assertHasKey(t, got, "operations")
	assertHasKey(t, got, "warnings")
	assertHasKey(t, got, "nonGoals")
	assertHasKey(t, got, "summary")

	operation := got["operations"].([]any)[0].(map[string]any)
	assertString(t, operation, "id", "op-001")
	assertString(t, operation, "action", "create-or-merge")
	assertString(t, operation, "resourceId", "instruction:project-claude-md")
	assertString(t, operation, "kind", "Instruction")
	assertString(t, operation, "sourcePath", "/repo/CLAUDE.md")
	assertString(t, operation, "targetPath", "/repo/AGENTS.md")
	assertString(t, operation, "status", "compatible")
	assertString(t, operation, "strategy", "merge-section-into-agents-md")
	assertBool(t, operation, "requiresReview", false)
	assertHasKey(t, operation, "warnings")

	summary := got["summary"].(map[string]any)
	assertNumber(t, summary, "create", 0)
	assertNumber(t, summary, "modify", 0)
	assertNumber(t, summary, "skip", 0)
	assertNumber(t, summary, "manual", 0)
	assertNumber(t, summary, "dangerous", 0)
}

func TestSnapshotReportMarshalsResourceStates(t *testing.T) {
	report := SnapshotReport{
		SchemaVersion: SnapshotSchemaVersion,
		Tool:          "claude",
		CreatedAt:     "2026-05-27T10:00:00Z",
		Project:       "/repo",
		Resources: []ResourceState{
			{
				ID:             "rule:global-shell",
				Kind:           KindRule,
				Scope:          ScopeGlobal,
				Tool:           "claude",
				Path:           "/home/.claude/rules/shell.md",
				TargetPathHint: "/repo/AGENTS.md",
				Status:         StatusCompatible,
				Strategy:       "merge-rule",
				ContentHash:    "sha256:source",
				NormalizedText: "Prefer Go.",
				Warnings:       []Warning{{Code: "normalized", Message: "content normalized"}},
			},
		},
		Warnings: []Warning{},
	}

	got := marshalToMap(t, report)

	assertString(t, got, "schemaVersion", "agent-canon.snapshot.v1")
	assertString(t, got, "tool", "claude")
	assertString(t, got, "createdAt", "2026-05-27T10:00:00Z")
	assertString(t, got, "project", "/repo")
	assertHasKey(t, got, "resources")
	assertHasKey(t, got, "warnings")

	resource := got["resources"].([]any)[0].(map[string]any)
	assertString(t, resource, "id", "rule:global-shell")
	assertString(t, resource, "kind", "Rule")
	assertString(t, resource, "scope", "global")
	assertString(t, resource, "tool", "claude")
	assertString(t, resource, "path", "/home/.claude/rules/shell.md")
	assertString(t, resource, "targetPathHint", "/repo/AGENTS.md")
	assertString(t, resource, "status", "compatible")
	assertString(t, resource, "strategy", "merge-rule")
	assertString(t, resource, "contentHash", "sha256:source")
	assertString(t, resource, "normalizedText", "Prefer Go.")
	assertHasKey(t, resource, "warnings")
}

func TestCanonSnapshotReportMarshalsResourceStates(t *testing.T) {
	report := CanonSnapshotReport{
		SchemaVersion: CanonSnapshotSchemaVersion,
		CreatedAt:     "2026-05-27T10:01:00Z",
		Project:       "/repo",
		Resources: []ResourceState{
			{
				ID:       "skill:project-review",
				Kind:     KindSkill,
				Scope:    ScopeProject,
				Tool:     "canon",
				Status:   StatusPartial,
				Strategy: "canonical-skill",
				Warnings: []Warning{},
			},
		},
		Warnings: []Warning{},
	}

	got := marshalToMap(t, report)

	assertString(t, got, "schemaVersion", "agent-canon.canon-snapshot.v1")
	assertString(t, got, "createdAt", "2026-05-27T10:01:00Z")
	assertString(t, got, "project", "/repo")
	assertHasKey(t, got, "resources")
	assertHasKey(t, got, "warnings")

	resource := got["resources"].([]any)[0].(map[string]any)
	assertString(t, resource, "tool", "canon")
	assertMissingKey(t, resource, "path")
	assertMissingKey(t, resource, "targetPathHint")
	assertMissingKey(t, resource, "contentHash")
	assertMissingKey(t, resource, "normalizedText")
}

func TestSyncStateReportMarshalsDiffsConflictsAndSummary(t *testing.T) {
	base := ResourceState{
		ID:          "instruction:project",
		Kind:        KindInstruction,
		Scope:       ScopeProject,
		Tool:        "canon",
		Status:      StatusCompatible,
		Strategy:    "merge-instruction",
		ContentHash: "sha256:base",
		Warnings:    []Warning{},
	}
	report := SyncStateReport{
		SchemaVersion: SyncStateSchemaVersion,
		CreatedAt:     "2026-05-27T10:02:00Z",
		Project:       "/repo",
		Source:        "claude",
		Target:        "codex",
		BaseSnapshots: map[string]string{"claude": "sha256:claude-base", "codex": "sha256:codex-base"},
		Diffs: []SemanticDiff{
			{
				ResourceID: "instruction:project",
				Kind:       KindInstruction,
				Scope:      ScopeProject,
				DiffKind:   DiffKindChanged,
				BaseHash:   "sha256:base",
				OursHash:   "sha256:ours",
				TheirsHash: "sha256:theirs",
				Summary:    "Instruction text differs.",
			},
		},
		Conflicts: []Conflict{
			{
				ID:                   "conflict-001",
				Kind:                 ConflictKindContent,
				ResourceID:           "instruction:project",
				ResourceKind:         KindInstruction,
				Scope:                ScopeProject,
				Base:                 &base,
				Ours:                 &base,
				Theirs:               &base,
				Suggestion:           "manual merge",
				SuggestionConfidence: 0.75,
				RequiresUserDecision: true,
				Status:               ConflictStatusOpen,
				ResolutionID:         "resolution-001",
				Fingerprint:          "content:instruction:project",
				Warnings:             []Warning{},
			},
		},
		Summary: SyncSummary{
			Diffs:             1,
			OpenConflicts:     1,
			ResolvedConflicts: 0,
			Warnings:          0,
		},
		Warnings: []Warning{},
	}

	got := marshalToMap(t, report)

	assertString(t, got, "schemaVersion", "agent-canon.sync-state.v1")
	assertString(t, got, "createdAt", "2026-05-27T10:02:00Z")
	assertString(t, got, "project", "/repo")
	assertString(t, got, "source", "claude")
	assertString(t, got, "target", "codex")
	assertHasKey(t, got, "baseSnapshots")
	assertHasKey(t, got, "diffs")
	assertHasKey(t, got, "conflicts")
	assertHasKey(t, got, "summary")
	assertHasKey(t, got, "warnings")

	baseSnapshots := got["baseSnapshots"].(map[string]any)
	assertString(t, baseSnapshots, "claude", "sha256:claude-base")
	assertString(t, baseSnapshots, "codex", "sha256:codex-base")

	diff := got["diffs"].([]any)[0].(map[string]any)
	assertString(t, diff, "resourceId", "instruction:project")
	assertString(t, diff, "kind", "Instruction")
	assertString(t, diff, "scope", "project")
	assertString(t, diff, "diffKind", "changed")
	assertString(t, diff, "baseHash", "sha256:base")
	assertString(t, diff, "oursHash", "sha256:ours")
	assertString(t, diff, "theirsHash", "sha256:theirs")
	assertString(t, diff, "summary", "Instruction text differs.")

	conflict := got["conflicts"].([]any)[0].(map[string]any)
	assertString(t, conflict, "id", "conflict-001")
	assertString(t, conflict, "kind", "ContentConflict")
	assertString(t, conflict, "resourceId", "instruction:project")
	assertString(t, conflict, "resourceKind", "Instruction")
	assertString(t, conflict, "scope", "project")
	assertHasKey(t, conflict, "base")
	assertHasKey(t, conflict, "ours")
	assertHasKey(t, conflict, "theirs")
	assertString(t, conflict, "suggestion", "manual merge")
	assertNumber(t, conflict, "suggestionConfidence", 0.75)
	assertBool(t, conflict, "requiresUserDecision", true)
	assertString(t, conflict, "status", "open")
	assertString(t, conflict, "resolutionId", "resolution-001")
	assertString(t, conflict, "fingerprint", "content:instruction:project")
	assertHasKey(t, conflict, "warnings")

	summary := got["summary"].(map[string]any)
	assertNumber(t, summary, "diffs", 1)
	assertNumber(t, summary, "openConflicts", 1)
	assertNumber(t, summary, "resolvedConflicts", 0)
	assertNumber(t, summary, "warnings", 0)
}

func TestSyncStateOptionalFieldsAreOmitted(t *testing.T) {
	diff := SemanticDiff{
		ResourceID: "memory:local",
		Kind:       KindMemoryItem,
		Scope:      ScopeLocal,
		DiffKind:   DiffKindAdded,
		Summary:    "Local memory added.",
	}
	conflict := Conflict{
		ID:                   "conflict-002",
		Kind:                 ConflictKindSemantic,
		ResourceID:           "memory:local",
		ResourceKind:         KindMemoryItem,
		Scope:                ScopeLocal,
		RequiresUserDecision: false,
		Status:               ConflictStatusResolved,
		Fingerprint:          "semantic:memory:local",
		Warnings:             []Warning{},
	}

	diffJSON := marshalToMap(t, diff)
	assertMissingKey(t, diffJSON, "baseHash")
	assertMissingKey(t, diffJSON, "oursHash")
	assertMissingKey(t, diffJSON, "theirsHash")

	conflictJSON := marshalToMap(t, conflict)
	assertString(t, conflictJSON, "kind", "SemanticConflict")
	assertString(t, conflictJSON, "status", "resolved")
	assertMissingKey(t, conflictJSON, "base")
	assertMissingKey(t, conflictJSON, "ours")
	assertMissingKey(t, conflictJSON, "theirs")
	assertMissingKey(t, conflictJSON, "suggestion")
	assertMissingKey(t, conflictJSON, "suggestionConfidence")
	assertMissingKey(t, conflictJSON, "resolutionId")
}

func TestLearnedResolutionReportMarshalsResolutions(t *testing.T) {
	report := LearnedResolutionReport{
		SchemaVersion: LearnedResolutionsSchemaVersion,
		Project:       "/repo",
		Resolutions: []LearnedResolution{
			{
				ID:                  "resolution-001",
				ConflictFingerprint: "content:instruction:project",
				ConflictKind:        ConflictKindContent,
				ResourceID:          "instruction:project",
				ResolvedAt:          "2026-05-27T10:03:00Z",
				Decision:            ResolutionDecisionManual,
				Value:               "merged text",
			},
		},
	}

	got := marshalToMap(t, report)

	assertString(t, got, "schemaVersion", "agent-canon.learned-resolutions.v1")
	assertString(t, got, "project", "/repo")
	assertHasKey(t, got, "resolutions")

	resolution := got["resolutions"].([]any)[0].(map[string]any)
	assertString(t, resolution, "id", "resolution-001")
	assertString(t, resolution, "conflictFingerprint", "content:instruction:project")
	assertString(t, resolution, "conflictKind", "ContentConflict")
	assertString(t, resolution, "resourceId", "instruction:project")
	assertString(t, resolution, "resolvedAt", "2026-05-27T10:03:00Z")
	assertString(t, resolution, "decision", "manual")
	assertString(t, resolution, "value", "merged text")
}

func TestLearnedResolutionOmitsEmptyValue(t *testing.T) {
	resolution := LearnedResolution{
		ID:                  "resolution-002",
		ConflictFingerprint: "location:skill:project-review",
		ConflictKind:        ConflictKindLocation,
		ResourceID:          "skill:project-review",
		ResolvedAt:          "2026-05-27T10:04:00Z",
		Decision:            ResolutionDecisionOurs,
	}

	got := marshalToMap(t, resolution)

	assertString(t, got, "conflictKind", "LocationConflict")
	assertString(t, got, "decision", "ours")
	assertMissingKey(t, got, "value")
}

func TestModelEnumsMarshalExpectedValues(t *testing.T) {
	enums := struct {
		Added              DiffKind           `json:"added"`
		Removed            DiffKind           `json:"removed"`
		Changed            DiffKind           `json:"changed"`
		Unchanged          DiffKind           `json:"unchanged"`
		LocationConflict   ConflictKind       `json:"locationConflict"`
		CapabilityConflict ConflictKind       `json:"capabilityConflict"`
		SecurityConflict   ConflictKind       `json:"securityConflict"`
		Theirs             ResolutionDecision `json:"theirs"`
		Suggestion         ResolutionDecision `json:"suggestion"`
	}{
		Added:              DiffKindAdded,
		Removed:            DiffKindRemoved,
		Changed:            DiffKindChanged,
		Unchanged:          DiffKindUnchanged,
		LocationConflict:   ConflictKindLocation,
		CapabilityConflict: ConflictKindCapability,
		SecurityConflict:   ConflictKindSecurity,
		Theirs:             ResolutionDecisionTheirs,
		Suggestion:         ResolutionDecisionSuggestion,
	}

	got := marshalToMap(t, enums)

	assertString(t, got, "added", "added")
	assertString(t, got, "removed", "removed")
	assertString(t, got, "changed", "changed")
	assertString(t, got, "unchanged", "unchanged")
	assertString(t, got, "locationConflict", "LocationConflict")
	assertString(t, got, "capabilityConflict", "CapabilityConflict")
	assertString(t, got, "securityConflict", "SecurityConflict")
	assertString(t, got, "theirs", "theirs")
	assertString(t, got, "suggestion", "suggestion")
}

func marshalToMap(t *testing.T, value any) map[string]any {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; payload = %s", err, payload)
	}
	return got
}

func assertHasKey(t *testing.T, values map[string]any, key string) {
	t.Helper()

	if _, ok := values[key]; !ok {
		t.Fatalf("marshaled JSON missing key %q in %#v", key, values)
	}
}

func assertMissingKey(t *testing.T, values map[string]any, key string) {
	t.Helper()

	if _, ok := values[key]; ok {
		t.Fatalf("marshaled JSON contains key %q in %#v", key, values)
	}
}

func assertString(t *testing.T, values map[string]any, key, want string) {
	t.Helper()

	got, ok := values[key].(string)
	if !ok {
		t.Fatalf("%s has type %T, want string", key, values[key])
	}
	if got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}

func assertNumber(t *testing.T, values map[string]any, key string, want float64) {
	t.Helper()

	got, ok := values[key].(float64)
	if !ok {
		t.Fatalf("%s has type %T, want number", key, values[key])
	}
	if got != want {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
}

func assertBool(t *testing.T, values map[string]any, key string, want bool) {
	t.Helper()

	got, ok := values[key].(bool)
	if !ok {
		t.Fatalf("%s has type %T, want bool", key, values[key])
	}
	if got != want {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
}
