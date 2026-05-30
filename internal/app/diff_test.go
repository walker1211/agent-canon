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

func TestRunDiffBeforeBaseSnapshotsExitsOneAndWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"diff", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "agent-canon sync claude codex") {
		t.Fatalf("stderr missing sync guidance: %q", stderr.String())
	}
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon"))
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("diff modified files without base snapshots")
	}
}

func TestRunDiffAfterInitialSyncReportsNoChangesAndDoesNotPersist(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	beforeState := readFileText(t, syncStatePath)
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"diff", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var report model.DiffReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid diff JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.DiffSchemaVersion || report.Target != "codex" || report.Summary.Diffs != 0 || report.Summary.OpenConflicts != 0 {
		t.Fatalf("diff report = %#v", report)
	}
	if readFileText(t, syncStatePath) != beforeState {
		t.Fatalf("diff updated sync state")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("diff modified files after initial sync")
	}
}

func TestRunDiffReportsOpenConflictWithoutPersistingIt(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	beforeState := readFileText(t, syncStatePath)
	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")
	projectBefore := directorySnapshot(t, fixture.project)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"diff", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Summary: diffs=1 open=1 resolved=0") || !strings.Contains(stdout.String(), "conflict-001") {
		t.Fatalf("stdout missing conflict summary: %q", stdout.String())
	}
	if readFileText(t, syncStatePath) != beforeState {
		t.Fatalf("diff persisted conflict to sync state")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) {
		t.Fatalf("diff modified project files")
	}
}

func TestRunDiffReportsCodexMCPConfigMergeConflictWithoutPersistingIt(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	writeFile(t, filepath.Join(fixture.project, ".claude", "settings.json"), `{
  "mcpServers": {
    "shared": {"command": "claude-shared"}
  }
}
`)
	writeFile(t, filepath.Join(fixture.project, ".codex", "config.toml"), `[mcp_servers.shared]
command = "codex-shared"
`)
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	beforeState := readFileText(t, syncStatePath)
	projectBefore := directorySnapshot(t, fixture.project)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"diff", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report model.DiffReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid diff JSON: %v\n%s", err, stdout.String())
	}
	conflict := requireConfigMergeConflict(t, report.Conflicts, "shared")
	if conflict.ID != "conflict-001" || conflict.Status != model.ConflictStatusOpen || conflict.Scope != model.ScopeProject {
		t.Fatalf("config conflict = %#v", conflict)
	}
	if report.Summary.Diffs != 0 || report.Summary.OpenConflicts != 1 || report.Summary.ResolvedConflicts != 0 {
		t.Fatalf("diff summary = %#v, want one open config conflict and no semantic diffs", report.Summary)
	}
	if readFileText(t, syncStatePath) != beforeState {
		t.Fatalf("diff persisted config conflict to sync state")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) {
		t.Fatalf("diff modified project files")
	}
}

func TestRunDiffDefaultTargetMatchesCodexTarget(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	var stdoutDefault, stderrDefault, stdoutCodex, stderrCodex bytes.Buffer

	defaultCode := app.Run([]string{"diff", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdoutDefault, &stderrDefault)
	codexCode := app.Run([]string{"diff", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdoutCodex, &stderrCodex)
	if defaultCode != 0 || codexCode != 0 {
		t.Fatalf("diff codes = %d/%d; stderr=%q/%q", defaultCode, codexCode, stderrDefault.String(), stderrCodex.String())
	}
	if stdoutDefault.String() != stdoutCodex.String() {
		t.Fatalf("diff default output != diff codex output:\ndefault=%s\ncodex=%s", stdoutDefault.String(), stdoutCodex.String())
	}
}

func TestRunDiffMalformedSettingsJSONReturnsExitTwoAfterBaseSnapshots(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	writeFile(t, filepath.Join(fixture.claudeHome, "settings.json"), "{")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"diff", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
