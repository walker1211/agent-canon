package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
)

func TestRunRollbackMissingManifestExitsOneAndWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	projectBefore := directorySnapshot(t, fixture.project)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", "apply-missing", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "rollback manifest") || !strings.Contains(stderr.String(), "apply-missing") {
		t.Fatalf("stderr missing rollback manifest context: %q", stderr.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) {
		t.Fatalf("rollback missing manifest modified project files")
	}
}

func TestRunRollbackDryRunAfterApplyWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture)
	projectBefore := directorySnapshot(t, fixture.project)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", applyID, "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"agent-canon rollback codex: dry-run", "Rollback changes:", "delete", filepath.Join(fixture.project, "AGENTS.md")} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("rollback dry-run modified files")
	}
}

func TestRunRollbackCreateRemovesGeneratedFileAndRefreshesState(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture)
	agentsPath := filepath.Join(fixture.project, "AGENTS.md")
	assertFileExists(t, agentsPath)
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", applyID, "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon rollback codex: applied") {
		t.Fatalf("stdout missing applied summary: %q", stdout.String())
	}
	assertPathMissing(t, agentsPath)
	state := readSyncState(t, filepath.Join(workspaceRoot, "sync-state.json"))
	if state.Summary.OpenConflicts != 0 || state.Summary.Diffs != 0 {
		t.Fatalf("refreshed sync state summary = %#v, want clean baseline", state.Summary)
	}
	baseCodex := readFileText(t, filepath.Join(workspaceRoot, "base", "codex.snapshot.json"))
	if strings.Contains(baseCodex, agentsPath) {
		t.Fatalf("codex base snapshot still includes rolled-back project AGENTS.md: %q", baseCodex)
	}
}

func TestRunRollbackModifyRestoresBackupContents(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	agentsPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, agentsPath, "old agents\n")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture)
	if got := readFileText(t, agentsPath); got == "old agents\n" {
		t.Fatalf("apply did not modify AGENTS.md")
	}
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", applyID, "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileContents(t, agentsPath, "old agents\n")
}

func TestRunRollbackConfirmationNoCancelsWithoutWrites(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture)
	agentsPath := filepath.Join(fixture.project, "AGENTS.md")
	before := readFileText(t, agentsPath)
	var stdout, stderr bytes.Buffer

	code := app.RunWithIO([]string{"rollback", applyID, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, strings.NewReader("n\n"), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "rollback cancelled") {
		t.Fatalf("stderr missing cancellation: %q", stderr.String())
	}
	assertFileContents(t, agentsPath, before)
}

func TestRunRollbackBlocksTargetDrift(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture)
	agentsPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, agentsPath, "drift\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", applyID, "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "hash") {
		t.Fatalf("stderr missing hash mismatch context: %q", stderr.String())
	}
	assertFileContents(t, agentsPath, "drift\n")
}

func TestRunRollbackMalformedManifestExitsOneAndWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture)
	agentsPath := filepath.Join(fixture.project, "AGENTS.md")
	before := readFileText(t, agentsPath)
	manifestPath := filepath.Join(fixture.project, ".agent-canon", "rollback", applyID+".json")
	writeFile(t, manifestPath, "{not json\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", applyID, "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "rollback manifest") {
		t.Fatalf("stderr missing rollback manifest context: %q", stderr.String())
	}
	assertFileContents(t, agentsPath, before)
}

func TestRunRollbackGlobalEntriesRequireGlobalFlag(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	applyID := runApplyCodexYes(t, fixture, "--global")
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"rollback", applyID, "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "global") {
		t.Fatalf("stderr missing global scope context: %q", stderr.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("rollback without --global modified Codex home")
	}
}

func runApplyCodexYes(t *testing.T, fixture fixturePaths, extra ...string) string {
	t.Helper()
	args := []string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	code := app.Run(args, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	return onlyRollbackApplyID(t, filepath.Join(fixture.project, ".agent-canon", "rollback"))
}

func onlyRollbackApplyID(t *testing.T, rollbackDir string) string {
	t.Helper()
	entries, err := os.ReadDir(rollbackDir)
	if err != nil {
		t.Fatalf("read rollback directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("rollback manifest count = %d, want 1", len(entries))
	}
	name := entries[0].Name()
	if !strings.HasSuffix(name, ".json") {
		t.Fatalf("rollback manifest name = %q, want .json suffix", name)
	}
	return strings.TrimSuffix(name, ".json")
}
