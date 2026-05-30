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

func TestApplyTextDryRunPrintsNoWriteNote(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:  "codex",
		Project: "/repo",
		Mode:    "dry-run",
		Changes: []model.ApplyFileChange{{Path: "/home/.codex/config.toml", Scope: model.ScopeGlobal, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Dry-run: no files were written.",
		"Backup and rollback manifest are created only when apply runs without --dry-run.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApplyText dry-run output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyTextDryRunPrintsNextSteps(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:  "codex",
		Project: "/repo",
		Mode:    "dry-run",
		Changes: []model.ApplyFileChange{{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Next steps:",
		"- Review Changed files before any write.",
		"- Run `agent-canon apply codex --yes` only after dry-run looks correct.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApplyText dry-run output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyTextPlannedModeDoesNotPrintDryRunNextSteps(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:  "codex",
		Project: "/repo",
		Mode:    "planned",
		Changes: []model.ApplyFileChange{{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, notWant := range []string{
		"Next steps:",
		"dry-run looks correct",
	} {
		if strings.Contains(text, notWant) {
			t.Fatalf("ApplyText planned output contains dry-run next step %q:\n%s", notWant, text)
		}
	}
}

func TestApplyTextDryRunExplainsDefaultGlobalBoundary(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:  "codex",
		Project: "/repo",
		Mode:    "dry-run",
		Changes: []model.ApplyFileChange{{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	want := "Global boundary: global Claude/Codex home writes are intentionally excluded unless --global is used."
	if !strings.Contains(text, want) {
		t.Fatalf("ApplyText dry-run output missing %q:\n%s", want, text)
	}
}

func TestApplyTextDryRunWithGlobalExplainsRealHomeTargets(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:        "codex",
		Project:       "/repo",
		Mode:          "dry-run",
		IncludeGlobal: true,
		Changes:       []model.ApplyFileChange{{Path: "/home/.codex/config.toml", Scope: model.ScopeGlobal, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Global boundary: listed global paths point at real Claude/Codex homes, but dry-run does not write them.",
		"- Run `agent-canon apply codex --global --yes` only after dry-run looks correct.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApplyText --global dry-run output missing %q:\n%s", want, text)
		}
	}
}

func TestApplyTextDryRunWithMergeConfigPreservesFlagInNextStep(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:        "codex",
		Project:       "/repo",
		Mode:          "dry-run",
		IncludeGlobal: true,
		MergeConfig:   true,
		Filters:       render.ApplyFilterTextReport{Only: []string{"config"}},
		Changes:       []model.ApplyFileChange{{Path: "/home/.codex/config.toml", Scope: model.ScopeGlobal, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	want := "- Run `agent-canon apply codex --global --merge-config --yes --only config` only after dry-run looks correct."
	if !strings.Contains(text, want) {
		t.Fatalf("ApplyText merge-config dry-run output missing %q:\n%s", want, text)
	}
}

func TestApplyTextDryRunWithGlobalSkippedSuggestsGlobalDryRunFirst(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:   "codex",
		Project:  "/repo",
		Mode:     "dry-run",
		Filters:  render.ApplyFilterTextReport{Only: []string{"config"}},
		Changes:  []model.ApplyFileChange{{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify}},
		Warnings: []model.Warning{{Code: "global-skipped", Message: "global Codex targets were skipped"}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	want := "- To inspect skipped global home changes first, run `agent-canon apply codex --global --dry-run --only config`."
	if !strings.Contains(text, want) {
		t.Fatalf("ApplyText dry-run output missing %q:\n%s", want, text)
	}
}

func TestApplyTextDryRunWithNoChangesAvoidsWriteCommand(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:   "codex",
		Project:  "/repo",
		Mode:     "dry-run",
		Warnings: []model.Warning{{Code: "review-only-config-skipped", Message: "config target skipped"}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"- No apply command is needed because there are no changed files.",
		"- Review skipped config warnings and migrate those settings manually.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApplyText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "--yes") {
		t.Fatalf("ApplyText suggested write command despite no changes:\n%s", text)
	}
}

func TestApplyTextPrintsFiltersAndGlobalGroups(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:        "codex",
		Project:       "/repo",
		Mode:          "dry-run",
		IncludeGlobal: true,
		Filters:       render.ApplyFilterTextReport{Only: []string{"config"}, Exclude: []string{"tokens/" + fixtureSecret + "/SKILL.md"}},
		GlobalGroups: []render.ApplyChangeGroupTextReport{{
			Name:    "config",
			Changes: []model.ApplyFileChange{{Path: "/home/.codex/config.toml", Scope: model.ScopeGlobal, Action: model.ApplyActionModify}},
		}},
		Changes: []model.ApplyFileChange{{Path: "/home/.codex/config.toml", Scope: model.ScopeGlobal, Action: model.ApplyActionModify}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"Filters: only=config exclude=tokens/" + security.RedactedValue + "/SKILL.md",
		"Global groups:",
		"- config: 1",
		"  - /home/.codex/config.toml",
		"- Run `agent-canon apply codex --global --yes --only config --exclude 'tokens/" + security.RedactedValue + "/SKILL.md'` only after dry-run looks correct.",
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

func TestRollbackTextPrintsChangesAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := render.RollbackTextReport{
		Target:       "codex",
		Project:      "/repo",
		Mode:         "dry-run",
		BackupDir:    "/repo/.agent-canon/backups/apply-001",
		ManifestPath: "/repo/.agent-canon/rollback/apply-001.json",
		Changes: []model.ApplyFileChange{
			{Path: "/repo/generated.md", Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: "sha256:after"},
			{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify, BeforeHash: "sha256:before", AfterHash: "sha256:after"},
			{Path: "/home/.codex/config.toml", Scope: model.ScopeGlobal, Action: model.ApplyActionNoop, BeforeHash: "sha256:same", AfterHash: "sha256:same"},
		},
		Warnings: []model.Warning{{Code: "secret-redacted", Message: "GITHUB_TOKEN=" + fixtureSecret}},
	}

	if err := render.RollbackText(&out, report); err != nil {
		t.Fatalf("RollbackText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon rollback codex: dry-run",
		"Project: /repo",
		"Summary: restore=1 delete=1 noop=1 warnings=1",
		"Backup: /repo/.agent-canon/backups/apply-001",
		"Manifest: /repo/.agent-canon/rollback/apply-001.json",
		"Rollback changes:",
		"- delete [project] /repo/generated.md",
		"- restore [project] /repo/AGENTS.md",
		"- noop [global] /home/.codex/config.toml",
		"GITHUB_TOKEN=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("RollbackText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("RollbackText leaked fixture secret:\n%s", text)
	}
}

func TestRollbackTextPrintsAppliedMode(t *testing.T) {
	var out strings.Builder
	report := render.RollbackTextReport{Target: "codex", Project: "/repo", Mode: "applied"}
	if err := render.RollbackText(&out, report); err != nil {
		t.Fatalf("RollbackText returned error: %v", err)
	}
	if !strings.Contains(out.String(), "agent-canon rollback codex: applied") {
		t.Fatalf("RollbackText output missing applied mode:\n%s", out.String())
	}
}

func TestRollbackTextPropagatesWriteErrors(t *testing.T) {
	err := render.RollbackText(failingWriter{}, render.RollbackTextReport{Target: "codex", Mode: "dry-run"})
	if err == nil {
		t.Fatalf("RollbackText returned nil error for failing writer")
	}
}

func TestImportTextPrintsSummaryPathsAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.ImportReport{
		SchemaVersion: model.ImportSchemaVersion,
		Project:       "/repo",
		Tool:          "codex",
		WorkspaceRoot: "/repo/.agent-canon",
		SnapshotPath:  "/repo/.agent-canon/base/codex.snapshot.json",
		ReportPath:    "/repo/.agent-canon/imports/codex.import.json",
		Summary:       model.ImportSummary{Resources: 2, Warnings: 1},
		Warnings:      []model.Warning{{Code: "secret-redacted", Message: "GITHUB_TOKEN=" + fixtureSecret}},
	}

	if err := render.ImportText(&out, report); err != nil {
		t.Fatalf("ImportText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon import codex",
		"Project: /repo",
		"Workspace: /repo/.agent-canon",
		"Snapshot: /repo/.agent-canon/base/codex.snapshot.json",
		"Report: /repo/.agent-canon/imports/codex.import.json",
		"Summary: resources=2 warnings=1",
		"Warnings:",
		"- warning[secret-redacted]: GITHUB_TOKEN=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ImportText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("ImportText leaked fixture secret:\n%s", text)
	}
}

func TestImportTextPropagatesWriteErrors(t *testing.T) {
	err := render.ImportText(failingWriter{}, model.ImportReport{Tool: "codex"})
	if err == nil {
		t.Fatalf("ImportText returned nil error for failing writer")
	}
}

func TestImportJSONWritesReportShape(t *testing.T) {
	var out strings.Builder
	report := model.ImportReport{
		SchemaVersion: model.ImportSchemaVersion,
		Project:       "/repo",
		Tool:          "codex",
		WorkspaceRoot: "/repo/.agent-canon",
		SnapshotPath:  "/repo/.agent-canon/base/codex.snapshot.json",
		ReportPath:    "/repo/.agent-canon/imports/codex.import.json",
		Summary:       model.ImportSummary{Resources: 2, Warnings: 0},
		Warnings:      []model.Warning{},
	}

	if err := render.ImportJSON(&out, report); err != nil {
		t.Fatalf("ImportJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("ImportJSON emitted invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != "agent-canon.import.v1" || got["tool"] != "codex" || got["snapshotPath"] != "/repo/.agent-canon/base/codex.snapshot.json" || got["reportPath"] != "/repo/.agent-canon/imports/codex.import.json" {
		t.Fatalf("ImportJSON output = %#v", got)
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

func TestSyncTextPrintsSummaryAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.SyncStateReport{
		Source:   "claude",
		Target:   "codex",
		Project:  "/repo",
		Summary:  model.SyncSummary{Diffs: 1, OpenConflicts: 0, ResolvedConflicts: 0, Warnings: 1},
		Warnings: []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.SyncText(&out, report, "/repo/.agent-canon", "/repo/.agent-canon/sync-state.json"); err != nil {
		t.Fatalf("SyncText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon sync: claude -> codex",
		"Project: /repo",
		"Workspace: /repo/.agent-canon",
		"State: /repo/.agent-canon/sync-state.json",
		"Summary: diffs=1 open=0 resolved=0 warnings=1",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("SyncText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("SyncText leaked fixture secret:\n%s", text)
	}
}

func TestConflictsTextPrintsConflictsAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.SyncStateReport{
		Source:  "claude",
		Target:  "codex",
		Project: "/repo",
		Conflicts: []model.Conflict{{
			ID:           "conflict-001",
			Kind:         model.ConflictKindContent,
			ResourceID:   "instruction:project-claude-md",
			ResourceKind: model.KindInstruction,
			Scope:        model.ScopeProject,
			Base:         &model.ResourceState{Path: "/repo/CLAUDE.md", Status: model.StatusCompatible, Strategy: "merge-instructions", ContentHash: "sha256:base"},
			Ours:         &model.ResourceState{Path: "/repo/CLAUDE.md", Status: model.StatusCompatible, Strategy: "merge-instructions", ContentHash: "sha256:ours"},
			Theirs:       &model.ResourceState{Path: "/repo/AGENTS.md", Status: model.StatusCompatible, Strategy: "merge-instructions", ContentHash: "sha256:theirs"},
			Status:       model.ConflictStatusOpen,
		}},
		Summary:  model.SyncSummary{Diffs: 1, OpenConflicts: 1, ResolvedConflicts: 0, Warnings: 1},
		Warnings: []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.ConflictsText(&out, report); err != nil {
		t.Fatalf("ConflictsText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon conflicts: claude -> codex",
		"Project: /repo",
		"Summary: open=1 resolved=0 diffs=1 warnings=1",
		"Open conflicts:",
		"- conflict-001 ContentConflict instruction:project-claude-md [Instruction] scope=project",
		"  why: both sides changed content differently",
		"  base: hash=sha256:base path=/repo/CLAUDE.md status=compatible strategy=merge-instructions",
		"  ours: hash=sha256:ours path=/repo/CLAUDE.md status=compatible strategy=merge-instructions",
		"  theirs: hash=sha256:theirs path=/repo/AGENTS.md status=compatible strategy=merge-instructions",
		"Resolved conflicts: 0",
		"Next steps:",
		"- Keep Claude side: `agent-canon resolve conflict-001 --ours`",
		"- Keep Codex side: `agent-canon resolve conflict-001 --theirs`",
		"- Write a manual value: `agent-canon resolve conflict-001 --manual <value>`",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ConflictsText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("ConflictsText leaked fixture secret:\n%s", text)
	}
}

func TestConflictsTextPrintsConfigMergeConflictSafeDetailsAndNextSteps(t *testing.T) {
	var out strings.Builder
	report := model.SyncStateReport{
		Source:  "claude",
		Target:  "codex",
		Project: "/repo",
		Conflicts: []model.Conflict{{
			ID:           "conflict-config-001",
			Kind:         model.ConflictKindConfigMerge,
			ResourceID:   "mcp:global-github",
			ResourceKind: model.KindMCPServer,
			Scope:        model.ScopeGlobal,
			Ours: &model.ResourceState{
				ID:             "mcp:global-github",
				Kind:           model.KindMCPServer,
				Scope:          model.ScopeGlobal,
				Tool:           "claude",
				Path:           "/home/user/.claude.json",
				Status:         model.StatusCompatible,
				Strategy:       "mcp-server-summary",
				ContentHash:    "sha256:ours",
				NormalizedText: "MCP server \"github\" normalized configuration summary; sha256=ours",
			},
			Theirs: &model.ResourceState{
				ID:             "mcp:global-github",
				Kind:           model.KindMCPServer,
				Scope:          model.ScopeGlobal,
				Tool:           "codex",
				Path:           "/home/user/.codex/config.toml",
				Status:         model.StatusPartial,
				Strategy:       "manual-mcp-server-review",
				ContentHash:    "sha256:theirs",
				NormalizedText: "MCP server \"github\" normalized configuration summary; sha256=theirs",
			},
			Status: model.ConflictStatusOpen,
			Details: map[string]string{
				"serverName": "github",
				"targetPath": "/home/user/.codex/config.toml",
				"sourcePath": "/home/user/.claude.json",
				"reason":     "existing Codex MCP server differs from Claude-derived MCP server",
				"command":    "mcp-server --token " + fixtureSecret,
				"env":        "GITHUB_TOKEN=" + fixtureSecret,
			},
		}},
		Summary: model.SyncSummary{Diffs: 1, OpenConflicts: 1},
	}

	if err := render.ConflictsText(&out, report); err != nil {
		t.Fatalf("ConflictsText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"- conflict-config-001 ConfigMergeConflict mcp:global-github [MCPServer] scope=global",
		"  why: Codex MCP server already exists with a different configuration",
		"  server: github",
		"  target: /home/user/.codex/config.toml",
		"  source: /home/user/.claude.json",
		"  reason detail: existing Codex MCP server differs from Claude-derived MCP server",
		"- Use Claude-derived MCP block: `agent-canon resolve conflict-config-001 --ours`",
		"- Keep existing Codex MCP block: `agent-canon resolve conflict-config-001 --theirs`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ConflictsText config conflict output missing %q:\n%s", want, text)
		}
	}
	for _, notWant := range []string{
		"--manual",
		"mcp-server --token",
		"GITHUB_TOKEN=",
		fixtureSecret,
	} {
		if strings.Contains(text, notWant) {
			t.Fatalf("ConflictsText config conflict output contains unsafe/unwanted %q:\n%s", notWant, text)
		}
	}
}

func TestConflictsTextPrintsSuggestionNextStep(t *testing.T) {
	var out strings.Builder
	report := model.SyncStateReport{
		Source:  "claude",
		Target:  "codex",
		Project: "/repo",
		Conflicts: []model.Conflict{{
			ID:                   "conflict-001",
			Kind:                 model.ConflictKindSemantic,
			ResourceID:           "instruction:project-claude-md",
			ResourceKind:         model.KindInstruction,
			Scope:                model.ScopeProject,
			Suggestion:           "merged instruction text",
			SuggestionConfidence: 0.82,
			Status:               model.ConflictStatusOpen,
		}},
		Summary: model.SyncSummary{Diffs: 1, OpenConflicts: 1},
	}

	if err := render.ConflictsText(&out, report); err != nil {
		t.Fatalf("ConflictsText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"  suggestion: confidence=0.82",
		"- Accept suggestion: `agent-canon resolve conflict-001 --accept-suggestion`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ConflictsText output missing %q:\n%s", want, text)
		}
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
