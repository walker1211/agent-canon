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
