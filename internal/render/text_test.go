package render_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

const fixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestApplyTextPrintsChangedFilesAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:       "codex",
		Project:      "/repo",
		Mode:         "dry-run",
		BackupDir:    "/repo/.agent-canon/backups/apply-001",
		ManifestPath: "/repo/.agent-canon/rollback/apply-001.json",
		Changes: []model.ApplyFileChange{
			{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify, BeforeHash: "sha256:before", AfterHash: "sha256:after"},
			{Path: "/repo/.codex/config.toml", Scope: model.ScopeProject, Action: model.ApplyActionNoop, BeforeHash: "sha256:same", AfterHash: "sha256:same"},
		},
		Warnings: []model.Warning{{Code: "secret-redacted", Message: "GITHUB_TOKEN=" + fixtureSecret}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon apply codex: dry-run",
		"Project: /repo",
		"Summary: create=0 modify=1 noop=1 warnings=1",
		"- modify [project] /repo/AGENTS.md",
		"Backup: /repo/.agent-canon/backups/apply-001",
		"Manifest: /repo/.agent-canon/rollback/apply-001.json",
		"GITHUB_TOKEN=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApplyText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("ApplyText leaked fixture secret:\n%s", text)
	}
}

func TestApplyTextPropagatesWriteErrors(t *testing.T) {
	err := render.ApplyText(failingWriter{}, render.ApplyTextReport{Target: "codex", Mode: "dry-run"})
	if err == nil {
		t.Fatalf("ApplyText returned nil error for failing writer")
	}
}

func TestInitTextPrintsManifestSummaryAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.WorkspaceManifestReport{
		SchemaVersion: model.WorkspaceManifestSchemaVersion,
		Project:       "/repo",
		Source:        "claude",
		Target:        "codex",
		WorkspaceRoot: "/repo/.agent-canon",
		Warnings:      []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.InitText(&out, report, "/repo/.agent-canon/manifest.json"); err != nil {
		t.Fatalf("InitText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon init: claude -> codex",
		"Project: /repo",
		"Workspace: /repo/.agent-canon",
		"Manifest: /repo/.agent-canon/manifest.json",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("InitText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("InitText leaked fixture secret:\n%s", text)
	}
}

func TestInitTextPropagatesWriteErrors(t *testing.T) {
	err := render.InitText(failingWriter{}, model.WorkspaceManifestReport{Source: "claude", Target: "codex"}, "manifest.json")
	if err == nil {
		t.Fatalf("InitText returned nil error for failing writer")
	}
}

func TestInitJSONWritesReportShape(t *testing.T) {
	var out strings.Builder
	report := model.WorkspaceManifestReport{SchemaVersion: model.WorkspaceManifestSchemaVersion, Source: "claude", Target: "codex", Project: "/repo", Warnings: []model.Warning{}}

	if err := render.InitJSON(&out, report); err != nil {
		t.Fatalf("InitJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("InitJSON emitted invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != "agent-canon.workspace-manifest.v1" || got["source"] != "claude" || got["target"] != "codex" {
		t.Fatalf("InitJSON output = %#v", got)
	}
}

func TestInitJSONPropagatesWriteErrors(t *testing.T) {
	err := render.InitJSON(failingWriter{}, model.WorkspaceManifestReport{Source: "claude", Target: "codex"})
	if err == nil {
		t.Fatalf("InitJSON returned nil error for failing writer")
	}
}

func TestStatusTextPrintsWorkspaceSummaryAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.StatusReport{
		SchemaVersion: model.StatusSchemaVersion,
		Project:       "/repo",
		WorkspaceRoot: "/repo/.agent-canon",
		Initialized:   true,
		ManifestPath:  "/repo/.agent-canon/manifest.json",
		SyncStatePath: "/repo/.agent-canon/sync-state.json",
		BaseSnapshots: map[string]bool{"claude": true, "codex": true, "canon": false},
		Summary:       model.StatusSummary{HasManifest: true, HasSyncState: true, HasBaseClaude: true, HasBaseCodex: true, OpenConflicts: 1, ResolvedConflicts: 2, Warnings: 1},
		Warnings:      []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.StatusText(&out, report); err != nil {
		t.Fatalf("StatusText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon status",
		"Project: /repo",
		"Workspace: /repo/.agent-canon",
		"Initialized: true",
		"Manifest: /repo/.agent-canon/manifest.json",
		"Sync state: /repo/.agent-canon/sync-state.json",
		"Summary: manifest=true syncState=true baseClaude=true baseCodex=true baseCanon=false open=1 resolved=2 warnings=1",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("StatusText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("StatusText leaked fixture secret:\n%s", text)
	}
}

func TestStatusTextPropagatesWriteErrors(t *testing.T) {
	err := render.StatusText(failingWriter{}, model.StatusReport{})
	if err == nil {
		t.Fatalf("StatusText returned nil error for failing writer")
	}
}

func TestStatusJSONWritesReportShape(t *testing.T) {
	var out strings.Builder
	report := model.StatusReport{SchemaVersion: model.StatusSchemaVersion, Project: "/repo", WorkspaceRoot: "/repo/.agent-canon", Initialized: true, BaseSnapshots: map[string]bool{"claude": true}, Warnings: []model.Warning{}}

	if err := render.StatusJSON(&out, report); err != nil {
		t.Fatalf("StatusJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("StatusJSON emitted invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != "agent-canon.status.v1" || got["project"] != "/repo" || got["initialized"] != true {
		t.Fatalf("StatusJSON output = %#v", got)
	}
}

func TestStatusJSONPropagatesWriteErrors(t *testing.T) {
	err := render.StatusJSON(failingWriter{}, model.StatusReport{})
	if err == nil {
		t.Fatalf("StatusJSON returned nil error for failing writer")
	}
}

func TestDiffTextPrintsDiffsConflictsAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.DiffReport{
		SchemaVersion: model.DiffSchemaVersion,
		Project:       "/repo",
		Target:        "codex",
		Diffs: []model.SemanticDiff{{
			ResourceID: "instruction:project-claude-md",
			Kind:       model.KindInstruction,
			Scope:      model.ScopeProject,
			DiffKind:   model.DiffKindChanged,
			Summary:    "GITHUB_TOKEN=" + fixtureSecret,
		}},
		Conflicts: []model.Conflict{{
			ID:           "conflict-001",
			Kind:         model.ConflictKindContent,
			ResourceID:   "instruction:project-claude-md",
			ResourceKind: model.KindInstruction,
			Status:       model.ConflictStatusOpen,
		}},
		Summary:  model.DiffSummary{Diffs: 1, OpenConflicts: 1, ResolvedConflicts: 0, Warnings: 1},
		Warnings: []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.DiffText(&out, report); err != nil {
		t.Fatalf("DiffText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon diff codex",
		"Project: /repo",
		"Summary: diffs=1 open=1 resolved=0 warnings=1",
		"Diffs:",
		"- changed [project] instruction:project-claude-md: GITHUB_TOKEN=" + security.RedactedValue,
		"Conflicts:",
		"- open conflict-001 ContentConflict instruction:project-claude-md [Instruction]",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("DiffText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("DiffText leaked fixture secret:\n%s", text)
	}
}

func TestDiffTextPropagatesWriteErrors(t *testing.T) {
	err := render.DiffText(failingWriter{}, model.DiffReport{Target: "codex"})
	if err == nil {
		t.Fatalf("DiffText returned nil error for failing writer")
	}
}

func TestDiffJSONWritesReportShape(t *testing.T) {
	var out strings.Builder
	report := model.DiffReport{SchemaVersion: model.DiffSchemaVersion, Project: "/repo", Target: "codex", Diffs: []model.SemanticDiff{}, Conflicts: []model.Conflict{}, Warnings: []model.Warning{}}

	if err := render.DiffJSON(&out, report); err != nil {
		t.Fatalf("DiffJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("DiffJSON emitted invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != "agent-canon.diff.v1" || got["target"] != "codex" || got["project"] != "/repo" {
		t.Fatalf("DiffJSON output = %#v", got)
	}
}

func TestDiffJSONPropagatesWriteErrors(t *testing.T) {
	err := render.DiffJSON(failingWriter{}, model.DiffReport{Target: "codex"})
	if err == nil {
		t.Fatalf("DiffJSON returned nil error for failing writer")
	}
}

func TestVerifyTextPrintsSummaryChecksAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.VerifyReport{
		SchemaVersion: model.VerifySchemaVersion,
		Target:        "codex",
		Project:       "/repo",
		Checks: []model.VerifyCheck{
			{ID: "codex-config-project", Target: "codex", Status: model.VerifyStatusPass, Message: "Codex config passed.", Path: "/repo/.codex/config.toml"},
			{ID: "codex-mcp-list", Target: "codex", Status: model.VerifyStatusWarn, Message: "GITHUB_TOKEN=" + fixtureSecret},
			{ID: "sync-conflicts", Target: "codex", Status: model.VerifyStatusFail, Message: "1 open conflict remains"},
		},
		Summary:  model.VerifySummary{Pass: 1, Warn: 1, Fail: 1},
		Warnings: []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.VerifyText(&out, report); err != nil {
		t.Fatalf("VerifyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon verify codex",
		"Project: /repo",
		"Summary: pass=1 warn=1 fail=1 warnings=1",
		"Checks:",
		"- pass codex-config-project: Codex config passed. (/repo/.codex/config.toml)",
		"- warn codex-mcp-list: GITHUB_TOKEN=" + security.RedactedValue,
		"- fail sync-conflicts: 1 open conflict remains",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("VerifyText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("VerifyText leaked fixture secret:\n%s", text)
	}
}

func TestVerifyTextPropagatesWriteErrors(t *testing.T) {
	err := render.VerifyText(failingWriter{}, model.VerifyReport{Target: "codex"})
	if err == nil {
		t.Fatalf("VerifyText returned nil error for failing writer")
	}
}

func TestVerifyJSONWritesReportShape(t *testing.T) {
	var out strings.Builder
	report := model.VerifyReport{
		SchemaVersion: model.VerifySchemaVersion,
		Target:        "claude",
		Project:       "/repo",
		Checks:        []model.VerifyCheck{{ID: "claude-instructions", Target: "claude", Status: model.VerifyStatusPass, Message: "CLAUDE.md passed."}},
		Summary:       model.VerifySummary{Pass: 1},
		Warnings:      []model.Warning{},
	}

	if err := render.VerifyJSON(&out, report); err != nil {
		t.Fatalf("VerifyJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("VerifyJSON emitted invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != "agent-canon.verify.v1" || got["target"] != "claude" {
		t.Fatalf("VerifyJSON output = %#v", got)
	}
}

func TestVerifyJSONPropagatesWriteErrors(t *testing.T) {
	err := render.VerifyJSON(failingWriter{}, model.VerifyReport{Target: "codex"})
	if err == nil {
		t.Fatalf("VerifyJSON returned nil error for failing writer")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
