package planner

import (
	"slices"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestBuildMapsResourceStatusesToOperations(t *testing.T) {
	scan := model.ScanReport{
		Source:  "claude",
		Target:  "codex",
		Project: "/repo",
		Resources: []model.Resource{
			resource("instruction:one", model.KindInstruction, model.StatusCompatible, "/src/CLAUDE.md", "/dst/AGENTS.md", "merge-section"),
			resource("skill:two", model.KindSkill, model.StatusPartial, "/src/skills/two/SKILL.md", "/dst/skills/two/SKILL.md", "convert-skill"),
			resource("session:three", model.KindSession, model.StatusUnsupported, "/src/session.jsonl", "", "out-of-scope"),
			resource("hook:four", model.KindHook, model.StatusDangerous, "/src/settings.json", "", "manual-hook-review"),
		},
	}

	plan := Build(scan)

	if plan.SchemaVersion != model.PlanSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", plan.SchemaVersion, model.PlanSchemaVersion)
	}
	if plan.Source != scan.Source || plan.Target != scan.Target || plan.Project != scan.Project {
		t.Fatalf("top-level source/target/project = %q/%q/%q, want %q/%q/%q", plan.Source, plan.Target, plan.Project, scan.Source, scan.Target, scan.Project)
	}
	if len(plan.Operations) != 4 {
		t.Fatalf("len(Operations) = %d, want 4", len(plan.Operations))
	}

	assertOperation(t, plan.Operations[0], "op-001", "create-or-merge", false, scan.Resources[0])
	assertOperation(t, plan.Operations[1], "op-002", "manual", true, scan.Resources[1])
	assertOperation(t, plan.Operations[2], "op-003", "skip", true, scan.Resources[2])
	assertOperation(t, plan.Operations[3], "op-004", "manual", true, scan.Resources[3])

	wantSummary := model.PlanSummary{Modify: 1, Skip: 1, Manual: 2, Dangerous: 1}
	if plan.Summary != wantSummary {
		t.Fatalf("Summary = %+v, want %+v", plan.Summary, wantSummary)
	}
}

func TestBuildMapsDangerousSecretRedactedResourceToRedact(t *testing.T) {
	warning := model.Warning{Code: "secret-redacted", Message: "token redacted"}
	scan := model.ScanReport{
		Resources: []model.Resource{
			withWarnings(resource("mcp:github", model.KindMCPServer, model.StatusDangerous, "/src/settings.json", "/dst/config.toml", "redact-env"), warning),
		},
	}

	plan := Build(scan)

	if got := plan.Operations[0].Action; got != "redact" {
		t.Fatalf("Action = %q, want redact", got)
	}
	if !plan.Operations[0].RequiresReview {
		t.Fatal("RequiresReview = false, want true")
	}
	if plan.Summary.Dangerous != 1 {
		t.Fatalf("Summary.Dangerous = %d, want 1", plan.Summary.Dangerous)
	}
	if plan.Summary.Manual != 0 {
		t.Fatalf("Summary.Manual = %d, want 0 for redact action", plan.Summary.Manual)
	}
}

func TestBuildCopiesWarnings(t *testing.T) {
	scanWarning := model.Warning{Code: "scan-warning", Message: "scan-level warning"}
	resourceWarning := model.Warning{Code: "resource-warning", Message: "resource-level warning"}
	scan := model.ScanReport{
		Warnings: []model.Warning{scanWarning},
		Resources: []model.Resource{
			withWarnings(resource("instruction:one", model.KindInstruction, model.StatusCompatible, "/src/CLAUDE.md", "/dst/AGENTS.md", "merge-section"), resourceWarning),
		},
	}

	plan := Build(scan)

	if !slices.Equal(plan.Warnings, scan.Warnings) {
		t.Fatalf("Warnings = %+v, want %+v", plan.Warnings, scan.Warnings)
	}
	if !slices.Equal(plan.Operations[0].Warnings, scan.Resources[0].Warnings) {
		t.Fatalf("operation Warnings = %+v, want %+v", plan.Operations[0].Warnings, scan.Resources[0].Warnings)
	}
}

func TestBuildPopulatesNonGoals(t *testing.T) {
	plan := Build(model.ScanReport{})

	if slices.Contains(plan.NonGoals, "export") {
		t.Fatalf("NonGoals = %#v, want export absent", plan.NonGoals)
	}

	for _, want := range []string{
		"apply",
		"sync",
		"conflicts/resolve",
		"three-way merge",
		"real home writes",
		"complete memory migration",
		"historical session migration",
		"hook execution",
		"model invocation",
		"plugin/skill/MCP installation",
		"automatic git push/commit/config",
	} {
		if !slices.Contains(plan.NonGoals, want) {
			t.Fatalf("NonGoals = %#v, missing %q", plan.NonGoals, want)
		}
	}
}

func TestBuildUsesStablePaddedOperationIDsInScanOrder(t *testing.T) {
	scan := model.ScanReport{Resources: []model.Resource{
		resource("one", model.KindInstruction, model.StatusCompatible, "/one", "/target-one", "merge"),
		resource("two", model.KindRule, model.StatusCompatible, "/two", "/target-two", "merge"),
		resource("three", model.KindConfig, model.StatusPartial, "/three", "/target-three", "review"),
	}}

	plan := Build(scan)

	for i, want := range []string{"op-001", "op-002", "op-003"} {
		if got := plan.Operations[i].ID; got != want {
			t.Fatalf("Operations[%d].ID = %q, want %q", i, got, want)
		}
	}
}

func TestBuildNeverCreatesAutomaticWritesForUnsupportedOrDangerousResources(t *testing.T) {
	scan := model.ScanReport{Resources: []model.Resource{
		resource("unsupported", model.KindSession, model.StatusUnsupported, "/session", "/target-session", "unsupported"),
		resource("dangerous", model.KindHook, model.StatusDangerous, "/hook", "/target-hook", "dangerous"),
		withWarnings(resource("secret", model.KindMCPServer, model.StatusDangerous, "/secret", "/target-secret", "redact"), model.Warning{Code: "secret-redacted", Message: "secret"}),
	}}

	plan := Build(scan)

	for _, operation := range plan.Operations {
		if operation.Status != model.StatusUnsupported && operation.Status != model.StatusDangerous {
			continue
		}
		if slices.Contains([]string{"create", "modify", "create-or-merge"}, operation.Action) {
			t.Fatalf("operation %+v uses automatic write action for %s resource", operation, operation.Status)
		}
	}
}

func resource(id string, kind model.ResourceKind, status model.Status, sourcePath, targetPathHint, strategy string) model.Resource {
	return model.Resource{
		ID:             id,
		Kind:           kind,
		SourcePath:     sourcePath,
		TargetPathHint: targetPathHint,
		Status:         status,
		Strategy:       strategy,
		Warnings:       []model.Warning{},
	}
}

func withWarnings(resource model.Resource, warnings ...model.Warning) model.Resource {
	resource.Warnings = warnings
	return resource
}

func assertOperation(t *testing.T, operation model.Operation, wantID, wantAction string, wantRequiresReview bool, resource model.Resource) {
	t.Helper()

	if operation.ID != wantID {
		t.Fatalf("ID = %q, want %q", operation.ID, wantID)
	}
	if operation.Action != wantAction {
		t.Fatalf("Action = %q, want %q", operation.Action, wantAction)
	}
	if operation.RequiresReview != wantRequiresReview {
		t.Fatalf("RequiresReview = %v, want %v", operation.RequiresReview, wantRequiresReview)
	}
	if operation.ResourceID != resource.ID {
		t.Fatalf("ResourceID = %q, want %q", operation.ResourceID, resource.ID)
	}
	if operation.Kind != resource.Kind {
		t.Fatalf("Kind = %q, want %q", operation.Kind, resource.Kind)
	}
	if operation.SourcePath != resource.SourcePath {
		t.Fatalf("SourcePath = %q, want %q", operation.SourcePath, resource.SourcePath)
	}
	if operation.TargetPath != resource.TargetPathHint {
		t.Fatalf("TargetPath = %q, want %q", operation.TargetPath, resource.TargetPathHint)
	}
	if operation.Status != resource.Status {
		t.Fatalf("Status = %q, want %q", operation.Status, resource.Status)
	}
	if operation.Strategy != resource.Strategy {
		t.Fatalf("Strategy = %q, want %q", operation.Strategy, resource.Strategy)
	}
}
