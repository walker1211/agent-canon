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

func TestRunStatusBeforeInitReportsUninitializedAndWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"status", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var report model.StatusReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid status JSON: %v\n%s", err, stdout.String())
	}
	if report.Initialized || report.Summary.HasManifest || report.ManifestPath != "" {
		t.Fatalf("status report = %#v, want uninitialized without manifest", report)
	}
	if report.SchemaVersion != model.StatusSchemaVersion || report.WorkspaceRoot != filepath.Join(fixture.project, ".agent-canon") {
		t.Fatalf("status metadata = %#v", report)
	}
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon"))
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("status modified files before init")
	}
}

func TestRunStatusAfterInitPrintsInitializedWorkspace(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"init", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("init exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	code = app.Run([]string{"status", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	manifestPath := filepath.Join(fixture.project, ".agent-canon", "manifest.json")
	for _, want := range []string{
		"agent-canon status",
		"Initialized: true",
		"Manifest: " + manifestPath,
		"Summary: manifest=true syncState=false baseClaude=false baseCodex=false baseCanon=false open=0 resolved=0 warnings=0",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunStatusAfterSyncReportsBaseSnapshotsAndConflictsReadOnly(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")
	runInitialSync(t, fixture)
	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")
	runInitialSync(t, fixture)
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"status", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("status exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var report model.StatusReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid status JSON: %v\n%s", err, stdout.String())
	}
	if !report.Summary.HasSyncState || !report.Summary.HasBaseClaude || !report.Summary.HasBaseCodex || !report.Summary.HasBaseCanon || report.Summary.OpenConflicts != 1 {
		t.Fatalf("status summary = %#v", report.Summary)
	}
	if !report.BaseSnapshots["claude"] || !report.BaseSnapshots["codex"] || !report.BaseSnapshots["canon"] {
		t.Fatalf("base snapshots = %#v", report.BaseSnapshots)
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("status modified files after sync")
	}
}

func TestRunStatusMalformedManifestExitsOne(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	manifestPath := filepath.Join(fixture.project, ".agent-canon", "manifest.json")
	writeFile(t, manifestPath, "{not json\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"status", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
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

func TestRunStatusMalformedSyncStateExitsOne(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	writeFile(t, syncStatePath, "{not json\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"status", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "sync state") {
		t.Fatalf("stderr missing sync state context: %q", stderr.String())
	}
	if got := readFileText(t, syncStatePath); got != "{not json\n" {
		t.Fatalf("malformed sync state was overwritten: %q", got)
	}
}
