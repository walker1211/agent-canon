package app_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestRunInitTextCreatesManifestAndWritesOnlyWorkspaceManifest(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"init", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	manifestPath := filepath.Join(fixture.project, ".agent-canon", "manifest.json")
	manifest := readWorkspaceManifest(t, manifestPath)
	if manifest.SchemaVersion != model.WorkspaceManifestSchemaVersion || manifest.Project != fixture.project || manifest.Source != "claude" || manifest.Target != "codex" {
		t.Fatalf("manifest metadata = %#v", manifest)
	}
	if !strings.Contains(stdout.String(), "agent-canon init: claude -> codex") || !strings.Contains(stdout.String(), manifestPath) {
		t.Fatalf("stdout missing init summary: %q", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	afterProject := directorySnapshot(t, fixture.project)
	delete(afterProject, filepath.Join(".agent-canon", "manifest.json"))
	if !equalStringMaps(afterProject, projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("init modified files beyond workspace manifest")
	}
}

func TestRunInitFormatJSONPrintsManifestReport(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"init", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var report model.WorkspaceManifestReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid manifest JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.WorkspaceManifestSchemaVersion || report.WorkspaceRoot != filepath.Join(fixture.project, ".agent-canon") {
		t.Fatalf("init JSON report = %#v", report)
	}
}

func TestRunInitTwicePreservesCreatedAt(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	manifestPath := filepath.Join(fixture.project, ".agent-canon", "manifest.json")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"init", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("first init exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	first := readWorkspaceManifest(t, manifestPath)
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"init", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second init exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	second := readWorkspaceManifest(t, manifestPath)
	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("CreatedAt = %q, want preserved %q", second.CreatedAt, first.CreatedAt)
	}
	if second.UpdatedAt == "" {
		t.Fatalf("UpdatedAt is empty after second init: %#v", second)
	}
}

func TestRunInitMalformedManifestExitsOneAndDoesNotOverwrite(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	manifestPath := filepath.Join(fixture.project, ".agent-canon", "manifest.json")
	writeFile(t, manifestPath, "{not json\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"init", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "manifest") {
		t.Fatalf("stderr missing manifest context: %q", stderr.String())
	}
	if got := readFileText(t, manifestPath); got != "{not json\n" {
		t.Fatalf("malformed manifest was overwritten: %q", got)
	}
}

func readWorkspaceManifest(t *testing.T, path string) model.WorkspaceManifestReport {
	t.Helper()
	payload := readFileText(t, path)
	var report model.WorkspaceManifestReport
	if err := json.Unmarshal([]byte(payload), &report); err != nil {
		t.Fatalf("unmarshal workspace manifest: %v\n%s", err, payload)
	}
	return report
}
