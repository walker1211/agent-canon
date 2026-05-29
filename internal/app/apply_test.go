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

func TestRunApplyCodexRequiresSyncState(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "run \"agent-canon sync claude codex\" first") {
		t.Fatalf("stderr missing sync guidance: %q", stderr.String())
	}
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))
}

func TestRunApplyCodexBlocksOpenConflicts(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")
	runInitialSync(t, fixture)
	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")
	runInitialSync(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "open conflicts") || !strings.Contains(stderr.String(), "agent-canon conflicts") {
		t.Fatalf("stderr missing conflict guidance: %q", stderr.String())
	}
	assertFileContents(t, codexPath, "theirs changed\n")
}

func TestRunApplyCodexDryRunWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	codexHomeBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))
	if !strings.Contains(stdout.String(), "agent-canon apply codex: dry-run") || !strings.Contains(stdout.String(), "Changed files:") || !strings.Contains(stdout.String(), filepath.Join(fixture.project, "AGENTS.md")) {
		t.Fatalf("stdout missing dry-run changed files: %q", stdout.String())
	}
	for _, want := range []string{
		"Global boundary: global Claude/Codex home writes are intentionally excluded unless --global is used.",
		"global-skipped",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("dry-run modified Codex home")
	}
}

func TestRunApplyCodexGlobalDryRunWritesNothingAndExplainsRealHomeTargets(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	codexHomeBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--global", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	want := "Global boundary: listed global paths point at real Claude/Codex homes, but dry-run does not write them."
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing %q: %q", want, stdout.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("global dry-run modified Codex home")
	}
}

func TestRunApplyCodexGlobalDryRunOnlyConfigFiltersOutputAndWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	codexHomeBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--global", "--dry-run", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{
		"Filters: only=config exclude=none",
		filepath.Join(fixture.project, ".codex", "config.toml"),
		"Global groups:",
		"- config: 0",
		"review-only-config-skipped",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("stdout missing %q:\n%s", want, text)
		}
	}
	for _, notWant := range []string{
		filepath.Join(fixture.codexHome, "AGENTS.md"),
		filepath.Join(fixture.codexHome, "skills"),
	} {
		if strings.Contains(text, notWant) {
			t.Fatalf("stdout contains filtered path %q:\n%s", notWant, text)
		}
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("filtered global dry-run modified Codex home")
	}
}

func TestRunApplyCodexGlobalYesOnlyConfigSkipsExistingUserConfig(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	configPath := filepath.Join(fixture.codexHome, "config.toml")
	beforeConfig := readFileText(t, configPath)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--global", "--yes", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got := readFileText(t, configPath); got != beforeConfig {
		t.Fatalf("config was overwritten; got %q want %q", got, beforeConfig)
	}
	for _, want := range []string{filepath.Join(fixture.project, ".codex", "config.toml"), "review-only-config-skipped", "selective-apply-baseline-not-refreshed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunApplyCodexConfirmationNoRejectsWithoutWrites(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.RunWithIO([]string{"apply", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, strings.NewReader("n\n"), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "apply cancelled") {
		t.Fatalf("stderr missing cancellation: %q", stderr.String())
	}
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))
}

func TestRunApplyCodexConfirmationYesWritesProjectFiles(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.RunWithIO([]string{"apply", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, strings.NewReader("yes\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	agents := readFileText(t, filepath.Join(fixture.project, "AGENTS.md"))
	if !strings.Contains(agents, "# AGENTS.md preview") || !strings.Contains(agents, "# Project Instructions") || !strings.Contains(agents, "This fixture project is read-only input for scanner tests.") {
		t.Fatalf("AGENTS.md missing generated project instructions: %q", agents)
	}
	if !strings.Contains(stdout.String(), "Apply these changes? [y/N]:") || !strings.Contains(stdout.String(), "agent-canon apply codex: applied") {
		t.Fatalf("stdout missing prompt or applied summary: %q", stdout.String())
	}
}

func TestRunApplyCodexYesWritesWithoutPromptAndSkipsGlobalByDefault(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	codexHomeBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	assertFileExists(t, filepath.Join(fixture.project, "AGENTS.md"))
	if strings.Contains(stdout.String(), "Apply these changes?") {
		t.Fatalf("stdout contains prompt despite --yes: %q", stdout.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexHomeBefore) {
		t.Fatalf("apply codex without --global modified Codex home")
	}
}

func TestRunApplyClaudeRequiresSyncState(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	projectBefore := directorySnapshot(t, fixture.project)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "run \"agent-canon sync claude codex\" first") {
		t.Fatalf("stderr missing sync guidance: %q", stderr.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) {
		t.Fatalf("apply claude without sync state modified project")
	}
}

func TestRunApplyClaudeDryRunWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon apply claude: dry-run") || !strings.Contains(stdout.String(), "Changed files:") || !strings.Contains(stdout.String(), filepath.Join(fixture.project, "CLAUDE.md")) {
		t.Fatalf("stdout missing dry-run changed files: %q", stdout.String())
	}
	for _, want := range []string{
		"Global boundary: global Claude/Codex home writes are intentionally excluded unless --global is used.",
		"global-skipped",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("dry-run modified files")
	}
}

func TestRunApplyClaudeGlobalDryRunWritesNothingAndExplainsRealHomeTargets(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--global", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	want := "Global boundary: listed global paths point at real Claude/Codex homes, but dry-run does not write them."
	if !strings.Contains(stdout.String(), want) {
		t.Fatalf("stdout missing %q: %q", want, stdout.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("global dry-run modified files")
	}
}

func TestRunApplyClaudeConfirmationNoRejectsWithoutWrites(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	projectBefore := directorySnapshot(t, fixture.project)
	var stdout, stderr bytes.Buffer

	code := app.RunWithIO([]string{"apply", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, strings.NewReader("n\n"), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "apply cancelled") {
		t.Fatalf("stderr missing cancellation: %q", stderr.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) {
		t.Fatalf("cancelled apply claude modified project")
	}
}

func TestRunApplyClaudeYesWritesProjectFilesAndSkipsGlobalByDefault(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	claude := readFileText(t, filepath.Join(fixture.project, "CLAUDE.md"))
	if !strings.Contains(claude, "# CLAUDE.md preview") || !strings.Contains(claude, "Generated preview for Claude") {
		t.Fatalf("CLAUDE.md missing generated preview: %q", claude)
	}
	assertFileExists(t, filepath.Join(fixture.project, ".claude", "settings.json"))
	assertFileExists(t, filepath.Join(fixture.project, ".claude", "skills", "sample-skill", "SKILL.md"))
	if strings.Contains(stdout.String(), "Apply these changes?") || !strings.Contains(stdout.String(), "agent-canon apply claude: applied") {
		t.Fatalf("stdout has prompt or missing applied summary: %q", stdout.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) {
		t.Fatalf("apply claude without --global modified Claude home")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("apply claude modified Codex home")
	}
}

func TestRunApplyCodexSelectiveApplyDoesNotRefreshCleanBaseline(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	writeFile(t, filepath.Join(fixture.project, "CLAUDE.md"), "# Project Instructions\n\nChanged but not selected.\n")
	runInitialSync(t, fixture)
	beforeState := readSyncState(t, filepath.Join(workspaceRoot, "sync-state.json"))
	if !hasSyncDiff(beforeState, "instruction:project-claude-md") {
		t.Fatalf("setup sync state missing unselected instruction diff: %#v", beforeState.Summary)
	}
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--yes", "--only", "config", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	assertFileExists(t, filepath.Join(fixture.project, ".codex", "config.toml"))
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))
	afterState := readSyncState(t, filepath.Join(workspaceRoot, "sync-state.json"))
	if afterState.Summary.Diffs == 0 || !hasSyncDiff(afterState, "instruction:project-claude-md") {
		t.Fatalf("selective apply swallowed unselected diff: %#v", afterState.Summary)
	}
	if !strings.Contains(stdout.String(), "selective-apply-baseline-not-refreshed") {
		t.Fatalf("stdout missing selective baseline warning: %q", stdout.String())
	}
	manifest := readOnlyRollbackManifest(t, filepath.Join(workspaceRoot, "rollback"))
	if !hasWarning(manifest.Warnings, "selective-apply-baseline-not-refreshed") {
		t.Fatalf("rollback manifest missing selective baseline warning: %#v", manifest.Warnings)
	}
}

func TestRunApplyClaudeYesAdvancesBaseStateAndWritesRollbackManifest(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	baseClaudePath := filepath.Join(workspaceRoot, "base", "claude.snapshot.json")
	beforeBase := readFileText(t, baseClaudePath)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	afterBase := readFileText(t, baseClaudePath)
	if afterBase == beforeBase || !strings.Contains(afterBase, filepath.Join(fixture.project, "CLAUDE.md")) {
		t.Fatalf("claude base snapshot was not advanced; before=%q after=%q", beforeBase, afterBase)
	}
	state := readSyncState(t, filepath.Join(workspaceRoot, "sync-state.json"))
	if state.Summary.OpenConflicts != 0 || state.Summary.Diffs != 0 {
		t.Fatalf("refreshed sync state summary = %#v, want clean baseline", state.Summary)
	}
	manifest := readOnlyRollbackManifest(t, filepath.Join(workspaceRoot, "rollback"))
	if manifest.SchemaVersion != model.RollbackManifestSchemaVersion || manifest.Target != "claude" || manifest.Project != fixture.project {
		t.Fatalf("rollback manifest metadata = %#v", manifest)
	}
	if manifest.BackupDir == "" || manifest.BaseSnapshots["claude"] != baseClaudePath || len(manifest.Changes) == 0 {
		t.Fatalf("rollback manifest missing backup/base/change context: %#v", manifest)
	}
	for _, change := range manifest.Changes {
		if !change.Verified {
			t.Fatalf("manifest change not verified: %#v", change)
		}
	}
}

func TestRunApplyClaudeDoesNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	runInitialSync(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "claude", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("apply claude output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, root := range []string{fixture.project, filepath.Join(fixture.project, ".agent-canon")} {
		if strings.Contains(readTreeText(t, root), secret) {
			t.Fatalf("apply claude leaked secret under %s", root)
		}
	}
}

func TestRunApplyCodexYesAdvancesBaseStateAndWritesRollbackManifest(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	baseCodexPath := filepath.Join(workspaceRoot, "base", "codex.snapshot.json")
	beforeBase := readFileText(t, baseCodexPath)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"apply", "codex", "--yes", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	afterBase := readFileText(t, baseCodexPath)
	if afterBase == beforeBase || !strings.Contains(afterBase, filepath.Join(fixture.project, "AGENTS.md")) {
		t.Fatalf("codex base snapshot was not advanced; before=%q after=%q", beforeBase, afterBase)
	}
	state := readSyncState(t, filepath.Join(workspaceRoot, "sync-state.json"))
	if state.Summary.OpenConflicts != 0 || state.Summary.Diffs != 0 {
		t.Fatalf("refreshed sync state summary = %#v, want clean baseline", state.Summary)
	}
	manifest := readOnlyRollbackManifest(t, filepath.Join(workspaceRoot, "rollback"))
	if manifest.SchemaVersion != model.RollbackManifestSchemaVersion || manifest.Target != "codex" || manifest.Project != fixture.project {
		t.Fatalf("rollback manifest metadata = %#v", manifest)
	}
	if manifest.BackupDir == "" || manifest.BaseSnapshots["codex"] != baseCodexPath || len(manifest.Changes) == 0 {
		t.Fatalf("rollback manifest missing backup/base/change context: %#v", manifest)
	}
	for _, change := range manifest.Changes {
		if !change.Verified {
			t.Fatalf("manifest change not verified: %#v", change)
		}
	}
}

func runInitialSync(t *testing.T, fixture fixturePaths) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func assertFileContents(t *testing.T, path string, want string) {
	t.Helper()
	got := readFileText(t, path)
	if got != want {
		t.Fatalf("%s contents = %q, want %q", path, got, want)
	}
}

func readFileText(t *testing.T, path string) string {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(got)
}

func readOnlyRollbackManifest(t *testing.T, rollbackDir string) model.RollbackManifestReport {
	t.Helper()
	entries, err := os.ReadDir(rollbackDir)
	if err != nil {
		t.Fatalf("read rollback directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("rollback manifest count = %d, want 1", len(entries))
	}
	payload, err := os.ReadFile(filepath.Join(rollbackDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read rollback manifest: %v", err)
	}
	var manifest model.RollbackManifestReport
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("unmarshal rollback manifest: %v\n%s", err, string(payload))
	}
	return manifest
}

func equalStringMaps(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if right[key] != leftValue {
			return false
		}
	}
	return true
}

func hasSyncDiff(state model.SyncStateReport, resourceID string) bool {
	for _, diff := range state.Diffs {
		if diff.ResourceID == resourceID {
			return true
		}
	}
	return false
}
