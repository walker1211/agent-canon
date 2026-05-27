package render_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/render"
)

func TestScanJSONIsValidAndIncludesSchemaVersion(t *testing.T) {
	report := sampleScanReport()
	var out bytes.Buffer

	if err := render.ScanJSON(&out, report); err != nil {
		t.Fatalf("ScanJSON returned error: %v", err)
	}

	var decoded model.ScanReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("ScanJSON output is invalid JSON: %v\n%s", err, out.String())
	}
	if decoded.SchemaVersion != model.ScanSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", decoded.SchemaVersion, model.ScanSchemaVersion)
	}
}

func TestPlanJSONIsValidAndIncludesSchemaVersion(t *testing.T) {
	report := planner.Build(sampleScanReport())
	var out bytes.Buffer

	if err := render.PlanJSON(&out, report); err != nil {
		t.Fatalf("PlanJSON returned error: %v", err)
	}

	var decoded model.PlanReport
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("PlanJSON output is invalid JSON: %v\n%s", err, out.String())
	}
	if decoded.SchemaVersion != model.PlanSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", decoded.SchemaVersion, model.PlanSchemaVersion)
	}
}

func TestScanTextIncludesHeaderProjectAndGroupedSections(t *testing.T) {
	var out bytes.Buffer

	if err := render.ScanText(&out, sampleScanReport()); err != nil {
		t.Fatalf("ScanText returned error: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"agent-canon scan: claude -> codex",
		"Project: /repo",
		"Compatible:",
		"Partial:",
		"Unsupported:",
		"Dangerous:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ScanText output missing %q:\n%s", want, text)
		}
	}
}

func TestPlanTextIncludesHeaderProjectAndGroupedSections(t *testing.T) {
	var out bytes.Buffer

	if err := render.PlanText(&out, planner.Build(sampleScanReport())); err != nil {
		t.Fatalf("PlanText returned error: %v", err)
	}

	text := out.String()
	for _, want := range []string{
		"agent-canon plan: claude -> codex",
		"Project: /repo",
		"create-or-merge:",
		"manual:",
		"skip:",
		"redact:",
		"Requires review:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("PlanText output missing %q:\n%s", want, text)
		}
	}
}

func sampleScanReport() model.ScanReport {
	return model.ScanReport{
		SchemaVersion: model.ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       "/repo",
		ClaudeHome:    "/home/.claude",
		CodexHome:     "/home/.codex",
		Resources: []model.Resource{
			resource("instruction:one", model.KindInstruction, model.StatusCompatible, "/src/CLAUDE.md", "/dst/AGENTS.md", "merge-instructions"),
			resource("skill:two", model.KindSkill, model.StatusPartial, "/src/skills/two/SKILL.md", "/dst/skills/two/SKILL.md", "review-skill"),
			resource("session:three", model.KindSession, model.StatusUnsupported, "/src/session.jsonl", "", "skip-session"),
			withWarnings(resource("mcp:four", model.KindMCPServer, model.StatusDangerous, "/src/settings.json", "/dst/config.toml", "redact-env"), model.Warning{Code: "secret-redacted", Message: "token redacted"}),
		},
		Summary: model.ScanSummary{Compatible: 1, Partial: 1, Unsupported: 1, Dangerous: 1},
	}
}

func resource(id string, kind model.ResourceKind, status model.Status, sourcePath, targetPathHint, strategy string) model.Resource {
	return model.Resource{ID: id, Kind: kind, SourcePath: sourcePath, TargetPathHint: targetPathHint, Status: status, Strategy: strategy, Warnings: []model.Warning{}}
}

func withWarnings(resource model.Resource, warnings ...model.Warning) model.Resource {
	resource.Warnings = warnings
	return resource
}
