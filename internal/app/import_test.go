package app_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestRunImportCodexWritesSnapshotAndReportOnly(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	projectInstruction := readFileText(t, filepath.Join(fixture.project, "CLAUDE.md"))
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	snapshotPath := filepath.Join(workspaceRoot, "base", "codex.snapshot.json")
	reportPath := filepath.Join(workspaceRoot, "imports", "codex.import.json")
	manifestPath := filepath.Join(workspaceRoot, "manifest.json")
	assertFileExists(t, snapshotPath)
	assertFileExists(t, reportPath)
	assertFileExists(t, manifestPath)
	assertPathMissing(t, filepath.Join(workspaceRoot, "base", "claude.snapshot.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "base", "canon.snapshot.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "sync-state.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "rollback"))
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))
	assertFileContents(t, filepath.Join(fixture.project, "CLAUDE.md"), projectInstruction)
	if !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) {
		t.Fatalf("import codex modified Claude home")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("import codex modified Codex home")
	}

	report := readImportReport(t, reportPath)
	snapshot := readSnapshotReport(t, snapshotPath)
	if report.SchemaVersion != model.ImportSchemaVersion || report.Tool != "codex" || report.Project != fixture.project {
		t.Fatalf("import report metadata = %#v", report)
	}
	if report.WorkspaceRoot != workspaceRoot || report.SnapshotPath != snapshotPath || report.ReportPath != reportPath {
		t.Fatalf("import report paths = %#v", report)
	}
	if report.Summary.Resources != len(snapshot.Resources) || report.Summary.Warnings != len(report.Warnings) {
		t.Fatalf("import summary = %#v, resources=%d warnings=%d", report.Summary, len(snapshot.Resources), len(report.Warnings))
	}
	for _, want := range []string{
		"agent-canon import codex",
		"Project: " + fixture.project,
		"Workspace: " + workspaceRoot,
		"Snapshot: " + snapshotPath,
		"Report: " + reportPath,
		"Summary: resources=",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunImportCodexFormatJSONPrintsReport(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report model.ImportReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid import JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.ImportSchemaVersion || report.Tool != "codex" || report.ReportPath == "" || report.SnapshotPath == "" {
		t.Fatalf("import JSON report = %#v", report)
	}
}

func TestRunImportCodexIncludeMemorySucceeds(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "codex", "--include-memory", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	report := readImportReport(t, filepath.Join(fixture.project, ".agent-canon", "imports", "codex.import.json"))
	if report.Tool != "codex" {
		t.Fatalf("import report = %#v", report)
	}
}

func TestRunImportCodexMalformedSettingsJSONReturnsExitTwo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "codex", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunImportCodexDoesNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("import output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	workspaceText := readTreeText(t, filepath.Join(fixture.project, ".agent-canon"))
	if strings.Contains(workspaceText, secret) {
		t.Fatalf("import workspace state leaked secret")
	}
}

func TestRunImportClaudeWritesSnapshotAndReportOnly(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	projectInstruction := readFileText(t, filepath.Join(fixture.project, "CLAUDE.md"))
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	snapshotPath := filepath.Join(workspaceRoot, "base", "claude.snapshot.json")
	reportPath := filepath.Join(workspaceRoot, "imports", "claude.import.json")
	manifestPath := filepath.Join(workspaceRoot, "manifest.json")
	assertFileExists(t, snapshotPath)
	assertFileExists(t, reportPath)
	assertFileExists(t, manifestPath)
	assertPathMissing(t, filepath.Join(workspaceRoot, "base", "codex.snapshot.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "base", "canon.snapshot.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "imports", "codex.import.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "sync-state.json"))
	assertPathMissing(t, filepath.Join(workspaceRoot, "rollback"))
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))
	assertFileContents(t, filepath.Join(fixture.project, "CLAUDE.md"), projectInstruction)
	if !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) {
		t.Fatalf("import claude modified Claude home")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("import claude modified Codex home")
	}

	report := readImportReport(t, reportPath)
	snapshot := readSnapshotReport(t, snapshotPath)
	if report.SchemaVersion != model.ImportSchemaVersion || report.Tool != "claude" || report.Project != fixture.project {
		t.Fatalf("import report metadata = %#v", report)
	}
	if report.WorkspaceRoot != workspaceRoot || report.SnapshotPath != snapshotPath || report.ReportPath != reportPath {
		t.Fatalf("import report paths = %#v", report)
	}
	if report.Summary.Resources != len(snapshot.Resources) || report.Summary.Warnings != len(report.Warnings) {
		t.Fatalf("import summary = %#v, resources=%d warnings=%d", report.Summary, len(snapshot.Resources), len(report.Warnings))
	}
	for _, want := range []string{
		"agent-canon import claude",
		"Project: " + fixture.project,
		"Workspace: " + workspaceRoot,
		"Snapshot: " + snapshotPath,
		"Report: " + reportPath,
		"Summary: resources=",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunImportClaudeFormatJSONPrintsReport(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report model.ImportReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid import JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.ImportSchemaVersion || report.Tool != "claude" || report.ReportPath == "" || report.SnapshotPath == "" {
		t.Fatalf("import JSON report = %#v", report)
	}
}

func TestRunImportClaudeIncludeMemorySucceeds(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--include-memory", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	report := readImportReport(t, filepath.Join(fixture.project, ".agent-canon", "imports", "claude.import.json"))
	if report.Tool != "claude" {
		t.Fatalf("import report = %#v", report)
	}
}

func TestRunImportClaudeMalformedSettingsJSONReturnsExitTwo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunImportClaudeDoesNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"import", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("import output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	workspaceText := readTreeText(t, filepath.Join(fixture.project, ".agent-canon"))
	if strings.Contains(workspaceText, secret) {
		t.Fatalf("import workspace state leaked secret")
	}
}

func readImportReport(t *testing.T, path string) model.ImportReport {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read import report: %v", err)
	}
	var report model.ImportReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("unmarshal import report: %v\n%s", err, string(payload))
	}
	return report
}

func readSnapshotReport(t *testing.T, path string) model.SnapshotReport {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot report: %v", err)
	}
	var report model.SnapshotReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("unmarshal snapshot report: %v\n%s", err, string(payload))
	}
	return report
}
