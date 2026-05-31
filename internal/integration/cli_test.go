package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

const fixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestBasicScanCommandFromSpecIsReadOnly(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"scan", "--from", "claude", "--to", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	var report model.ScanReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid scan JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.ScanSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", report.SchemaVersion, model.ScanSchemaVersion)
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestBasicPlanCommandFromSpecIsReadOnly(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"plan", "--from", "claude", "--to", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	var report model.PlanReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid plan JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.PlanSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", report.SchemaVersion, model.PlanSchemaVersion)
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestRealWorldSanitizedFixturesScanAndPlanAreReadOnly(t *testing.T) {
	for _, fixtureName := range []string{"real-world-mcp", "real-world-discovery", "real-world-conflict"} {
		t.Run(fixtureName, func(t *testing.T) {
			fixture := tempFixturePathsFor(t, fixtureName)
			before := snapshotFiles(t, fixture.root)

			var scanStdout, scanStderr bytes.Buffer
			scanCode := app.Run([]string{"scan", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &scanStdout, &scanStderr)
			assertNoUnsafePublicMarkers(t, scanStdout.String(), fixtureName+" scan stdout")
			assertNoUnsafePublicMarkers(t, scanStderr.String(), fixtureName+" scan stderr")
			if scanCode != 0 {
				t.Fatalf("scan exit code = %d, want 0; stdout=%q stderr=%q", scanCode, scanStdout.String(), scanStderr.String())
			}
			var scanReport model.ScanReport
			if err := json.Unmarshal(scanStdout.Bytes(), &scanReport); err != nil {
				t.Fatalf("scan stdout is not valid JSON: %v\n%s", err, scanStdout.String())
			}

			var planStdout, planStderr bytes.Buffer
			planCode := app.Run([]string{"plan", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &planStdout, &planStderr)
			assertNoUnsafePublicMarkers(t, planStdout.String(), fixtureName+" plan stdout")
			assertNoUnsafePublicMarkers(t, planStderr.String(), fixtureName+" plan stderr")
			if planCode != 0 {
				t.Fatalf("plan exit code = %d, want 0; stdout=%q stderr=%q", planCode, planStdout.String(), planStderr.String())
			}
			var planReport model.PlanReport
			if err := json.Unmarshal(planStdout.Bytes(), &planReport); err != nil {
				t.Fatalf("plan stdout is not valid JSON: %v\n%s", err, planStdout.String())
			}

			assertFilesUnchanged(t, fixture.root, before)
		})
	}
}

func TestInvalidDirectionReturnsExitOne(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"scan", "--from", "codex", "--to", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestNonexistentExplicitCustomPathReturnsExitOne(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "project",
			args: []string{"scan", "--project", filepath.Join(fixture.root, "missing-project"), "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		},
		{
			name: "claude home",
			args: []string{"scan", "--project", fixture.project, "--claude-home", filepath.Join(fixture.root, "missing-claude-home"), "--codex-home", fixture.codexHome},
		},
		{
			name: "codex home",
			args: []string{"scan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", filepath.Join(fixture.root, "missing-codex-home")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := app.Run(tt.args, fixture.project, fixture.home, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestMalformedSettingsJSONReturnsExitTwo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	mustWriteFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"scan", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestPlanOutWritesOnlyRequestedPlanFile(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	outDir := t.TempDir()
	outPath := filepath.Join(outDir, "plan.json")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"plan", "--from", "claude", "--to", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outPath}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatalf("read out dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "plan.json" || entries[0].IsDir() {
		t.Fatalf("out dir entries = %#v, want only plan.json file", entries)
	}
	payload, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read plan output: %v", err)
	}
	var report model.PlanReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("plan output is not valid JSON: %v\n%s", err, string(payload))
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestExportCodexWritesPreviewTreeAndLeavesFixtureInputsUnchanged(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "codex-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	for _, path := range []string{
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
		filepath.Join(".agents", "skills", "sample-skill", "SKILL.md"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestExportClaudeWritesPreviewTreeAndLeavesFixtureInputsUnchanged(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "claude-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	for _, path := range []string{
		"CLAUDE.md",
		filepath.Join(".claude", "settings.json"),
		filepath.Join(".claude", "skills", "sample-skill", "SKILL.md"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	if !strings.Contains(stdout.String(), "agent-canon export claude") || !strings.Contains(stdout.String(), "wrote Claude preview") {
		t.Fatalf("stdout missing Claude export summary: %q", stdout.String())
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestExportCodexRejectsExistingNonEmptyPreviewDir(t *testing.T) {
	fixture := fixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	outDir := t.TempDir()
	existing := filepath.Join(outDir, "existing.txt")
	mustWriteFile(t, existing, "keep\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	payload, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(payload) != "keep\n" {
		t.Fatalf("existing file contents = %q, want keep", string(payload))
	}
	assertPathMissing(t, filepath.Join(outDir, "AGENTS.md"))
	assertFilesUnchanged(t, fixture.root, before)
}

func TestSecretFixtureExportDoesNotLeakToCLIOutputsOrGeneratedFiles(t *testing.T) {
	fixture := fixturePathsFor(t, "secrets")
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "codex-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "stdout")
	assertDoesNotContainSecret(t, stderr.String(), "stderr")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	assertGeneratedFilesDoNotContainSecret(t, outDir)
	assertFilesUnchanged(t, fixture.root, before)
}

func TestSecretFixtureExportClaudeDoesNotLeakToCLIOutputsOrGeneratedFiles(t *testing.T) {
	fixture := fixturePathsFor(t, "secrets")
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "claude-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "claude export stdout")
	assertDoesNotContainSecret(t, stderr.String(), "claude export stderr")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	assertGeneratedFilesDoNotContainSecret(t, outDir)
	assertFilesUnchanged(t, fixture.root, before)
}

func TestUnsupportedFixtureExportReportsSkippedResourcesWithoutAutomaticFiles(t *testing.T) {
	fixture := fixturePathsFor(t, "unsupported")
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "codex-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	report := readFileString(t, filepath.Join(outDir, "migration-report.md"))
	for _, want := range []string{
		"hook:global-PreToolUse",
		"session:global-session-history",
		"skipped unsupported resources",
		"review-required resources",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("migration report missing %q in:\n%s", want, report)
		}
	}
	for _, rel := range previewRelativePaths(t, outDir) {
		if strings.Contains(rel, "PreToolUse") || strings.Contains(rel, "session-history") {
			t.Fatalf("unsupported resource generated automatic preview file %q", rel)
		}
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestSecretFixtureTokenDoesNotLeakToCLIOutputs(t *testing.T) {
	fixture := fixturePathsFor(t, "secrets")
	cases := []struct {
		name string
		args []string
	}{
		{
			name: "scan text report",
			args: []string{"scan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		},
		{
			name: "scan json stdout",
			args: []string{"scan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"},
		},
		{
			name: "plan text report",
			args: []string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		},
		{
			name: "plan json stdout",
			args: []string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := app.Run(tc.args, fixture.project, fixture.home, &stdout, &stderr)
			assertDoesNotContainSecret(t, stdout.String(), "stdout")
			assertDoesNotContainSecret(t, stderr.String(), "stderr")
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, redactSecret(stderr.String()))
			}
		})
	}

	t.Run("plan out json and stdout", func(t *testing.T) {
		outPath := filepath.Join(t.TempDir(), "plan.json")
		var stdout, stderr bytes.Buffer
		code := app.Run([]string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outPath}, fixture.project, fixture.home, &stdout, &stderr)
		assertDoesNotContainSecret(t, stdout.String(), "stdout")
		assertDoesNotContainSecret(t, stderr.String(), "stderr")
		if code != 0 {
			t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
		}
		payload, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read plan output: %v", err)
		}
		assertDoesNotContainSecret(t, string(payload), "plan JSON")
	})
}

func TestImportClaudeWritesOnlyWorkspaceBaselineAndReport(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon import claude") {
		t.Fatalf("stdout missing import header: %q", stdout.String())
	}

	allowed := map[string]bool{
		snapshotDirKey(filepath.Join("project", ".agent-canon")):                  true,
		snapshotDirKey(filepath.Join("project", ".agent-canon", "base")):          true,
		snapshotDirKey(filepath.Join("project", ".agent-canon", "imports")):       true,
		filepath.Join("project", ".agent-canon", "manifest.json"):                 true,
		filepath.Join("project", ".agent-canon", "base", "claude.snapshot.json"):  true,
		filepath.Join("project", ".agent-canon", "imports", "claude.import.json"): true,
	}
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	assertFileExists(t, filepath.Join(workspaceRoot, "manifest.json"))
	assertFileExists(t, filepath.Join(workspaceRoot, "base", "claude.snapshot.json"))
	assertFileExists(t, filepath.Join(workspaceRoot, "imports", "claude.import.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "base", "codex.snapshot.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "base", "canon.snapshot.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "imports", "codex.import.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "sync-state.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "rollback"))
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))

	after := snapshotFiles(t, fixture.root)
	for rel, contents := range before {
		if allowed[rel] {
			continue
		}
		if after[rel] != contents {
			t.Fatalf("import claude changed fixture input %s", rel)
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok && !allowed[rel] {
			t.Fatalf("import claude created unexpected file %s", rel)
		}
	}

	payload, err := os.ReadFile(filepath.Join(workspaceRoot, "imports", "claude.import.json"))
	if err != nil {
		t.Fatalf("read import report: %v", err)
	}
	var report model.ImportReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("import report JSON invalid: %v\n%s", err, string(payload))
	}
	if report.Tool != "claude" || report.SnapshotPath != filepath.Join(workspaceRoot, "base", "claude.snapshot.json") {
		t.Fatalf("import report = %#v", report)
	}
}

func TestSecretFixtureImportClaudeDoesNotLeakToCLIOutputsOrState(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "import claude stdout")
	assertDoesNotContainSecret(t, stderr.String(), "import claude stderr")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	for _, rel := range previewRelativePaths(t, filepath.Join(fixture.project, ".agent-canon")) {
		assertDoesNotContainSecret(t, readFileString(t, filepath.Join(fixture.project, ".agent-canon", rel)), filepath.Join(".agent-canon", rel))
	}
}

func TestSyncConflictsResolveRoundTripUsesProjectWorkspaceOnly(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	mustWriteFile(t, filepath.Join(fixture.project, "CLAUDE.md"), "shared base\n")
	mustWriteFile(t, filepath.Join(fixture.project, "AGENTS.md"), "shared base\n")
	claudeBefore := snapshotFiles(t, fixture.claudeHome)
	codexBefore := snapshotFiles(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("initial sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileExists(t, filepath.Join(fixture.project, ".agent-canon", "base", "claude.snapshot.json"))
	assertFileExists(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))

	mustWriteFile(t, filepath.Join(fixture.project, "CLAUDE.md"), "ours changed\n")
	mustWriteFile(t, filepath.Join(fixture.project, "AGENTS.md"), "theirs changed\n")
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Summary: diffs=1 open=1 resolved=0") {
		t.Fatalf("sync stdout missing open conflict summary: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"conflicts", "--project", fixture.project}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("conflicts exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "- conflict-001 ContentConflict instruction:project-claude-md") {
		t.Fatalf("conflicts stdout missing conflict: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"resolve", "conflict-001", "--manual", "merged value", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("resolve exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "resolved conflict-001 with manual as resolution-001") {
		t.Fatalf("resolve stdout missing confirmation: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"conflicts", "--project", fixture.project, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("conflicts json exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var state model.SyncStateReport
	if err := json.Unmarshal(stdout.Bytes(), &state); err != nil {
		t.Fatalf("conflicts JSON invalid: %v\n%s", err, stdout.String())
	}
	if state.Summary.OpenConflicts != 0 || state.Summary.ResolvedConflicts != 1 {
		t.Fatalf("resolved summary = %#v", state.Summary)
	}
	assertFilesUnchanged(t, fixture.claudeHome, claudeBefore)
	assertFilesUnchanged(t, fixture.codexHome, codexBefore)
}

func TestSyncConflictsResolveTheirsThenApplyCodexMergeConfigDryRun(t *testing.T) {
	fixture := tempFixtureWithProjectMCPConfigConflict(t)
	configPath := filepath.Join(fixture.project, ".codex", "config.toml")
	beforeConfig := readFileString(t, configPath)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"resolve", "conflict-001", "--theirs", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("resolve exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "resolved conflict-001 with theirs as resolution-001") {
		t.Fatalf("resolve stdout missing confirmation: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"apply", "codex", "--merge-config", "--dry-run", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply dry-run exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon apply codex: dry-run") || !strings.Contains(stdout.String(), "modify") || !strings.Contains(stdout.String(), configPath) {
		t.Fatalf("apply dry-run stdout missing merge plan: %q", stdout.String())
	}
	if got := readFileString(t, configPath); got != beforeConfig {
		t.Fatalf("dry-run modified config: got %q want %q", got, beforeConfig)
	}
}

func TestSyncConflictsResolveOursThenApplyCodexMergeConfigYes(t *testing.T) {
	fixture := tempFixtureWithProjectMCPConfigConflict(t)
	configPath := filepath.Join(fixture.project, ".codex", "config.toml")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"resolve", "conflict-001", "--ours", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("resolve exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"apply", "codex", "--merge-config", "--yes", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply yes exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	contents := readFileString(t, configPath)
	if !strings.Contains(contents, "command = \"claude-shared\"") || strings.Contains(contents, "command = \"codex-shared\"") {
		t.Fatalf("resolved ours apply did not write replacement config:\n%s", contents)
	}
	if !strings.Contains(stdout.String(), "agent-canon apply codex: applied") {
		t.Fatalf("apply stdout missing applied summary: %q", stdout.String())
	}
	rollbackEntries, err := os.ReadDir(filepath.Join(fixture.project, ".agent-canon", "rollback"))
	if err != nil || len(rollbackEntries) == 0 {
		t.Fatalf("rollback entries = %#v err=%v, want at least one apply manifest", rollbackEntries, err)
	}
}

func TestSecretFixtureSyncDoesNotLeakToCLIOutputsOrState(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "sync stdout")
	assertDoesNotContainSecret(t, stderr.String(), "sync stderr")
	if code != 0 {
		t.Fatalf("sync exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	for _, rel := range previewRelativePaths(t, filepath.Join(fixture.project, ".agent-canon")) {
		assertDoesNotContainSecret(t, readFileString(t, filepath.Join(fixture.project, ".agent-canon", rel)), filepath.Join(".agent-canon", rel))
	}
	after := snapshotFiles(t, fixture.root)
	for rel, beforeContents := range before {
		if strings.HasPrefix(rel, filepath.Join("project", ".agent-canon")+string(filepath.Separator)) {
			continue
		}
		if after[rel] != beforeContents {
			t.Fatalf("sync changed non-workspace fixture file %s", rel)
		}
	}
}

func TestCompileCodexWritesPreviewTreeAndLeavesFixtureRootUnchanged(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("compile exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, path := range []string{
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	if !strings.Contains(stdout.String(), "agent-canon compile codex") || !strings.Contains(stdout.String(), "Summary: files=") {
		t.Fatalf("compile stdout missing summary: %q", stdout.String())
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestCompileClaudeWritesPreviewTreeAndLeavesFixtureRootUnchanged(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	before := snapshotFiles(t, fixture.root)
	outDir := filepath.Join(t.TempDir(), "claude-compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("compile claude exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, path := range []string{
		"CLAUDE.md",
		filepath.Join(".claude", "settings.json"),
		filepath.Join(".claude", "skills", "sample-skill", "SKILL.md"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	if !strings.Contains(stdout.String(), "agent-canon compile claude") || !strings.Contains(stdout.String(), "Summary: files=") {
		t.Fatalf("compile claude stdout missing summary: %q", stdout.String())
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestSecretFixtureCompileDoesNotLeakToCLIOutputsOrGeneratedPreview(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	runSyncCommand(t, fixture)
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "compile stdout")
	assertDoesNotContainSecret(t, stderr.String(), "compile stderr")
	if code != 0 {
		t.Fatalf("compile exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	assertGeneratedFilesDoNotContainSecret(t, outDir)
}

func TestSecretFixtureCompileClaudeDoesNotLeakToCLIOutputsOrGeneratedPreview(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	runSyncCommand(t, fixture)
	outDir := filepath.Join(t.TempDir(), "claude-compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "compile claude stdout")
	assertDoesNotContainSecret(t, stderr.String(), "compile claude stderr")
	if code != 0 {
		t.Fatalf("compile claude exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	assertGeneratedFilesDoNotContainSecret(t, outDir)
}

func TestApplyCodexDryRunAndYesRoundTrip(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	codexHomeBefore := snapshotFiles(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon apply codex: dry-run") || !strings.Contains(stdout.String(), filepath.Join(fixture.project, "AGENTS.md")) {
		t.Fatalf("dry-run stdout missing planned project write: %q", stdout.String())
	}
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileExists(t, filepath.Join(fixture.project, "AGENTS.md"))
	if !reflect.DeepEqual(snapshotFiles(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("apply codex without --global modified Codex home")
	}
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	assertFileExists(t, onlyFileInDir(t, filepath.Join(workspaceRoot, "rollback")))
	state := readSyncStateReport(t, filepath.Join(workspaceRoot, "sync-state.json"))
	if state.Summary.OpenConflicts != 0 || state.Summary.Diffs != 0 {
		t.Fatalf("apply refreshed sync summary = %#v", state.Summary)
	}
	baseCodex := readFileString(t, filepath.Join(workspaceRoot, "base", "codex.snapshot.json"))
	if !strings.Contains(baseCodex, filepath.Join(fixture.project, "AGENTS.md")) {
		t.Fatalf("base codex snapshot missing applied project target: %q", baseCodex)
	}
}

func TestApplyCodexGlobalDryRunOnlyConfigDoesNotWriteHome(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--global", "--dry-run", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{"Filters: only=config exclude=none", filepath.Join(fixture.project, ".codex", "config.toml"), "review-only-config-skipped"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stdout missing %q:\n%s", want, text)
		}
	}
	for _, notWant := range []string{filepath.Join(fixture.codexHome, "AGENTS.md"), filepath.Join(fixture.codexHome, "skills")} {
		if strings.Contains(text, notWant) {
			t.Fatalf("stdout contains filtered global path %q:\n%s", notWant, text)
		}
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestApplyCodexBacksUpExistingProjectTarget(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("initial apply exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	originalAgents := readFileString(t, filepath.Join(fixture.project, "AGENTS.md"))
	mustWriteFile(t, filepath.Join(fixture.project, "CLAUDE.md"), "# Project Instructions\n\nUpdated after initial apply.\n")

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second apply exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	backup := onlyFileInDir(t, filepath.Join(fixture.project, ".agent-canon", "backups", latestDirName(t, filepath.Join(fixture.project, ".agent-canon", "backups")), "project"))
	if readFileString(t, backup) != originalAgents {
		t.Fatalf("backup contents did not preserve original AGENTS.md")
	}
}

func TestApplyClaudeDryRunYesAndRollbackRoundTrip(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	beforeDryRun := snapshotFiles(t, fixture.root)
	projectBefore := snapshotFiles(t, fixture.project)
	claudeHomeBefore := snapshotFiles(t, fixture.claudeHome)
	codexHomeBefore := snapshotFiles(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("dry-run exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon apply claude: dry-run") || !strings.Contains(stdout.String(), filepath.Join(fixture.project, "CLAUDE.md")) {
		t.Fatalf("dry-run stdout missing planned Claude write: %q", stdout.String())
	}
	assertFilesUnchanged(t, fixture.root, beforeDryRun)

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"apply", "claude", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	claude := readFileString(t, filepath.Join(fixture.project, "CLAUDE.md"))
	if !strings.Contains(claude, "# CLAUDE.md preview") || !strings.Contains(claude, "Generated preview for Claude") {
		t.Fatalf("CLAUDE.md missing generated preview: %q", claude)
	}
	assertFileExists(t, filepath.Join(fixture.project, ".claude", "settings.json"))
	assertFileExists(t, filepath.Join(fixture.project, ".claude", "skills", "sample-skill", "SKILL.md"))
	if !reflect.DeepEqual(snapshotFiles(t, fixture.claudeHome), claudeHomeBefore) {
		t.Fatalf("apply claude without --global modified Claude home")
	}
	if !reflect.DeepEqual(snapshotFiles(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("apply claude modified Codex home")
	}

	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	manifestPath := onlyFileInDir(t, filepath.Join(workspaceRoot, "rollback"))
	manifestPayload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read rollback manifest: %v", err)
	}
	var manifest model.RollbackManifestReport
	if err := json.Unmarshal(manifestPayload, &manifest); err != nil {
		t.Fatalf("unmarshal rollback manifest: %v\n%s", err, string(manifestPayload))
	}
	if manifest.Target != "claude" || len(manifest.Changes) == 0 {
		t.Fatalf("rollback manifest = %#v, want Claude target with changes", manifest)
	}

	applyID := strings.TrimSuffix(filepath.Base(manifestPath), ".json")
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"rollback", applyID, "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("rollback exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, rel := range []string{"CLAUDE.md", filepath.Join(".claude", "settings.json"), filepath.Join(".claude", "skills", "sample-skill", "SKILL.md")} {
		got := readFileString(t, filepath.Join(fixture.project, rel))
		if got != projectBefore[rel] {
			t.Fatalf("rollback restored %s = %q, want %q", rel, got, projectBefore[rel])
		}
	}
	if !reflect.DeepEqual(snapshotFiles(t, fixture.claudeHome), claudeHomeBefore) {
		t.Fatalf("rollback claude modified Claude home")
	}
	if !reflect.DeepEqual(snapshotFiles(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("rollback claude modified Codex home")
	}
}

func TestSecretFixtureApplyDoesNotLeakToCLIOutputsOrState(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	runSyncCommand(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "apply stdout")
	assertDoesNotContainSecret(t, stderr.String(), "apply stderr")
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	for _, rel := range previewRelativePaths(t, filepath.Join(fixture.project, ".agent-canon")) {
		assertDoesNotContainSecret(t, readFileString(t, filepath.Join(fixture.project, ".agent-canon", rel)), filepath.Join(".agent-canon", rel))
	}
}

func TestSecretFixtureApplyClaudeDoesNotLeakToCLIOutputsOrState(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	runSyncCommand(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "apply claude stdout")
	assertDoesNotContainSecret(t, stderr.String(), "apply claude stderr")
	if code != 0 {
		t.Fatalf("apply claude exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout.String()), redactSecret(stderr.String()))
	}
	for _, root := range []string{fixture.project, filepath.Join(fixture.project, ".agent-canon")} {
		for _, rel := range previewRelativePaths(t, root) {
			assertDoesNotContainSecret(t, readFileString(t, filepath.Join(root, rel)), filepath.Join(root, rel))
		}
	}
}

func TestVerifyCodexAfterApplyRoundTrip(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	runSyncCommand(t, fixture)
	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify codex exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon verify codex") || !strings.Contains(stdout.String(), "fail=0") {
		t.Fatalf("verify codex stdout missing success summary: %q", stdout.String())
	}
}

func TestVerifyCodexJSONReportsSchemaAndIsReadOnly(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify codex json exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report model.VerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("verify JSON invalid: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.VerifySchemaVersion || report.Target != "codex" {
		t.Fatalf("verify report metadata = %#v", report)
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestVerifyCodexMalformedConfigReturnsExitOne(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	mustWriteFile(t, filepath.Join(fixture.project, ".codex", "config.toml"), "[mcp_servers.github\ncommand = \"gh\"\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("verify malformed config exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "fail codex-config-project") {
		t.Fatalf("verify malformed config stdout missing failed check: %q", stdout.String())
	}
}

func TestVerifyCodexOpenConflictsReturnsExitOne(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	mustWriteFile(t, filepath.Join(fixture.project, "CLAUDE.md"), "shared base\n")
	mustWriteFile(t, filepath.Join(fixture.project, "AGENTS.md"), "shared base\n")
	runSyncCommand(t, fixture)
	mustWriteFile(t, filepath.Join(fixture.project, "CLAUDE.md"), "ours changed\n")
	mustWriteFile(t, filepath.Join(fixture.project, "AGENTS.md"), "theirs changed\n")
	runSyncCommand(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("verify open conflicts exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "fail sync-conflicts") || !strings.Contains(stdout.String(), "open conflicts") {
		t.Fatalf("verify open conflicts stdout missing failed check: %q", stdout.String())
	}
}

func TestVerifyClaudeIsReadOnly(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("verify claude exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon verify claude") {
		t.Fatalf("verify claude stdout missing header: %q", stdout.String())
	}
	assertFilesUnchanged(t, fixture.root, before)
}

func TestSecretFixtureVerifyDoesNotLeakToCLIOutputsOrState(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")
	before := snapshotFiles(t, fixture.root)
	var stdout, stderr bytes.Buffer

	_ = app.Run([]string{"verify", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	assertDoesNotContainSecret(t, stdout.String(), "verify stdout")
	assertDoesNotContainSecret(t, stderr.String(), "verify stderr")
	assertFilesUnchanged(t, fixture.root, before)
}

func TestDogfoodSafeWorkflowUsesOnlyWorkspaceAndPreviewOutputs(t *testing.T) {
	fixture := tempFixturePathsFor(t, "basic")
	beforeReadOnly := snapshotFiles(t, fixture.root)

	runDogfoodAppCommand(t, fixture, "scan", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeReadOnly)
	runDogfoodAppCommand(t, fixture, "plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeReadOnly)

	exportPreview := filepath.Join(t.TempDir(), "export-codex-preview")
	runDogfoodAppCommand(t, fixture, "export", "codex", "--out", exportPreview, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	if len(previewRelativePaths(t, exportPreview)) == 0 {
		t.Fatalf("export preview %s has no files", exportPreview)
	}
	assertFilesUnchanged(t, fixture.root, beforeReadOnly)

	beforeSync := snapshotFiles(t, fixture.root)
	runDogfoodAppCommand(t, fixture, "sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	afterSync := snapshotFiles(t, fixture.root)
	for rel, beforeContents := range beforeSync {
		if strings.HasPrefix(rel, filepath.Join("project", ".agent-canon")+string(filepath.Separator)) {
			continue
		}
		if afterSync[rel] != beforeContents {
			t.Fatalf("sync changed non-workspace fixture file %s", rel)
		}
	}
	for rel := range afterSync {
		if _, ok := beforeSync[rel]; ok {
			continue
		}
		if strings.HasPrefix(rel, filepath.Join("project", ".agent-canon")+string(filepath.Separator)) || rel == snapshotDirKey(filepath.Join("project", ".agent-canon")) {
			continue
		}
		t.Fatalf("sync created non-workspace fixture file %s", rel)
	}

	codexCompilePreview := filepath.Join(t.TempDir(), "compile-codex-preview")
	beforeCompileCodex := snapshotFiles(t, fixture.root)
	runDogfoodAppCommand(t, fixture, "compile", "codex", "--out", codexCompilePreview, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	if len(previewRelativePaths(t, codexCompilePreview)) == 0 {
		t.Fatalf("compile codex preview %s has no files", codexCompilePreview)
	}
	assertFilesUnchanged(t, fixture.root, beforeCompileCodex)

	claudeCompilePreview := filepath.Join(t.TempDir(), "compile-claude-preview")
	beforeCompileClaude := snapshotFiles(t, fixture.root)
	runDogfoodAppCommand(t, fixture, "compile", "claude", "--out", claudeCompilePreview, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	if len(previewRelativePaths(t, claudeCompilePreview)) == 0 {
		t.Fatalf("compile claude preview %s has no files", claudeCompilePreview)
	}
	assertFilesUnchanged(t, fixture.root, beforeCompileClaude)

	beforeDryRun := snapshotFiles(t, fixture.root)
	runDogfoodAppCommand(t, fixture, "apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeDryRun)
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))

	claudeHomeBeforeGlobalDryRun := snapshotFiles(t, fixture.claudeHome)
	codexHomeBeforeGlobalDryRun := snapshotFiles(t, fixture.codexHome)
	beforeGlobalDryRun := snapshotFiles(t, fixture.root)
	runDogfoodAppCommand(t, fixture, "apply", "codex", "--global", "--dry-run", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeGlobalDryRun)
	assertFilesUnchanged(t, fixture.claudeHome, claudeHomeBeforeGlobalDryRun)
	assertFilesUnchanged(t, fixture.codexHome, codexHomeBeforeGlobalDryRun)
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))

	beforeVerify := snapshotFiles(t, fixture.root)
	stdout, stderr, code := runDogfoodAppCommandAllowingFailure(t, fixture, "verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeVerify)
	if code != 0 && !strings.Contains(stdout, "agent-canon verify codex") {
		t.Fatalf("verify codex exit code = %d without controlled report; stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestDogfoodSafeWorkflowDoesNotLeakSecrets(t *testing.T) {
	fixture := tempFixturePathsFor(t, "secrets")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "scan", args: []string{"scan", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}},
		{name: "plan", args: []string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runDogfoodAppCommandAllowingFailure(t, fixture, tc.args...)
			assertDoesNotContainSecret(t, stdout, tc.name+" stdout")
			assertDoesNotContainSecret(t, stderr, tc.name+" stderr")
			if code != 0 {
				t.Fatalf("%s exit code = %d, want 0; stdout=%q stderr=%q", tc.name, code, redactSecret(stdout), redactSecret(stderr))
			}
		})
	}

	stdout, stderr, code := runDogfoodAppCommandAllowingFailure(t, fixture, "sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertDoesNotContainSecret(t, stdout, "sync stdout")
	assertDoesNotContainSecret(t, stderr, "sync stderr")
	if code != 0 {
		t.Fatalf("sync exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout), redactSecret(stderr))
	}
	assertGeneratedFilesDoNotContainSecret(t, filepath.Join(fixture.project, ".agent-canon"))

	compilePreview := filepath.Join(t.TempDir(), "compile-codex-preview")
	stdout, stderr, code = runDogfoodAppCommandAllowingFailure(t, fixture, "compile", "codex", "--out", compilePreview, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertDoesNotContainSecret(t, stdout, "compile stdout")
	assertDoesNotContainSecret(t, stderr, "compile stderr")
	if code != 0 {
		t.Fatalf("compile exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout), redactSecret(stderr))
	}
	assertGeneratedFilesDoNotContainSecret(t, compilePreview)

	stdout, stderr, code = runDogfoodAppCommandAllowingFailure(t, fixture, "apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertDoesNotContainSecret(t, stdout, "apply stdout")
	assertDoesNotContainSecret(t, stderr, "apply stderr")
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stdout=%q stderr=%q", code, redactSecret(stdout), redactSecret(stderr))
	}
	assertGeneratedFilesDoNotContainSecret(t, filepath.Join(fixture.project, ".agent-canon"))

	stdout, stderr, code = runDogfoodAppCommandAllowingFailure(t, fixture, "verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertDoesNotContainSecret(t, stdout, "verify stdout")
	assertDoesNotContainSecret(t, stderr, "verify stderr")
	if code != 0 && !strings.Contains(stdout, "agent-canon verify codex") {
		t.Fatalf("verify exit code = %d without controlled report; stdout=%q stderr=%q", code, redactSecret(stdout), redactSecret(stderr))
	}
}

type fixturePaths struct {
	home       string
	root       string
	project    string
	claudeHome string
	codexHome  string
}

func fixturePathsFor(t *testing.T, name string) fixturePaths {
	t.Helper()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	root := filepath.Join(repoRoot, "testdata", name)
	return fixturePaths{
		home:       root,
		root:       root,
		project:    filepath.Join(root, "project"),
		claudeHome: filepath.Join(root, "claude-home"),
		codexHome:  filepath.Join(root, "codex-home"),
	}
}

func tempFixturePathsFor(t *testing.T, name string) fixturePaths {
	t.Helper()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	source := filepath.Join(repoRoot, "testdata", name)
	root := t.TempDir()
	copyDir(t, source, root)
	return fixturePaths{
		home:       root,
		root:       root,
		project:    filepath.Join(root, "project"),
		claudeHome: filepath.Join(root, "claude-home"),
		codexHome:  filepath.Join(root, "codex-home"),
	}
}

func copyDir(t *testing.T, source string, target string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, payload, 0o644)
	}); err != nil {
		t.Fatalf("copy fixture %s to %s: %v", source, target, err)
	}
}

func runSyncCommand(t *testing.T, fixture fixturePaths) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func runDogfoodAppCommand(t *testing.T, fixture fixturePaths, args ...string) string {
	t.Helper()
	stdout, stderr, code := runDogfoodAppCommandAllowingFailure(t, fixture, args...)
	if code != 0 {
		t.Fatalf("%s exit code = %d, want 0; stdout=%q stderr=%q", strings.Join(args, " "), code, stdout, stderr)
	}
	return stdout
}

func runDogfoodAppCommandAllowingFailure(t *testing.T, fixture fixturePaths, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := app.Run(args, fixture.project, fixture.home, &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func tempFixtureWithProjectMCPConfigConflict(t *testing.T) fixturePaths {
	t.Helper()
	fixture := tempFixturePathsFor(t, "basic")
	mustWriteFile(t, filepath.Join(fixture.project, ".claude", "settings.json"), `{
  "mcpServers": {
    "shared": {
      "command": "claude-shared",
      "args": ["--from", "claude"]
    }
  }
}
`)
	mustWriteFile(t, filepath.Join(fixture.project, ".codex", "config.toml"), `[mcp_servers.shared]
command = "codex-shared"
args = ["--from", "codex"]
`)
	runSyncCommand(t, fixture)
	runSyncCommand(t, fixture)
	state := readSyncStateReport(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))
	if state.Summary.OpenConflicts != 1 || len(state.Conflicts) != 1 || state.Conflicts[0].Kind != model.ConflictKindConfigMerge {
		t.Fatalf("setup state = %#v summary=%#v, want one config merge conflict", state.Conflicts, state.Summary)
	}
	return fixture
}

func readSyncStateReport(t *testing.T, path string) model.SyncStateReport {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sync state: %v", err)
	}
	var state model.SyncStateReport
	if err := json.Unmarshal(payload, &state); err != nil {
		t.Fatalf("unmarshal sync state: %v\n%s", err, string(payload))
	}
	return state
}

func latestDirName(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read directory %s: %v", root, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatalf("%s has no directories", root)
	}
	return names[len(names)-1]
}

func onlyFileInDir(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read directory %s: %v", root, err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(root, entry.Name()))
		}
	}
	if len(files) != 1 {
		t.Fatalf("%s file count = %d, want 1", root, len(files))
	}
	return files[0]
}

const snapshotDirectoryValue = "<DIR>"

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel != "." {
				files[snapshotDirKey(rel)] = snapshotDirectoryValue
			}
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = string(contents)
		return nil
	}); err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return files
}

func snapshotDirKey(rel string) string {
	return rel + string(os.PathSeparator)
}

func assertFilesUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := snapshotFiles(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("fixture tree changed under %s\nbefore: %#v\nafter: %#v", root, before, after)
	}
}

func assertDoesNotContainSecret(t *testing.T, value string, label string) {
	t.Helper()
	if strings.Contains(value, fixtureSecret) {
		t.Fatalf("%s leaked fixture secret", label)
	}
}

func assertNoUnsafePublicMarkers(t *testing.T, value string, label string) {
	t.Helper()
	for _, marker := range []string{"ghp_", "sk-", "BEGIN PRIVATE KEY", "/Users/"} {
		if strings.Contains(value, marker) {
			t.Fatalf("%s contains unsafe public marker %q", label, marker)
		}
	}
}

func assertGeneratedFilesDoNotContainSecret(t *testing.T, root string) {
	t.Helper()
	for _, rel := range previewRelativePaths(t, root) {
		assertDoesNotContainSecret(t, readFileString(t, filepath.Join(root, rel)), rel)
	}
}

func previewRelativePaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	}); err != nil {
		t.Fatalf("walk preview root %s: %v", root, err)
	}
	sort.Strings(paths)
	return paths
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory, want file", path)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists unexpectedly", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func redactSecret(value string) string {
	return strings.ReplaceAll(value, fixtureSecret, "<REDACTED-FIXTURE-SECRET>")
}

func mustWriteFile(t *testing.T, path string, contents string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
