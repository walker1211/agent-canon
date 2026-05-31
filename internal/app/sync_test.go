package app_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestRunSyncInitializesWorkspaceAndBaseSnapshots(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	statePath := filepath.Join(workspaceRoot, "sync-state.json")
	assertFileExists(t, filepath.Join(workspaceRoot, "base", "claude.snapshot.json"))
	assertFileExists(t, filepath.Join(workspaceRoot, "base", "codex.snapshot.json"))
	assertFileExists(t, filepath.Join(workspaceRoot, "base", "canon.snapshot.json"))
	assertFileExists(t, statePath)

	state := readSyncState(t, statePath)
	if state.SchemaVersion != model.SyncStateSchemaVersion || state.Source != "claude" || state.Target != "codex" {
		t.Fatalf("sync state metadata = %#v", state)
	}
	if state.Summary.Diffs != 0 || state.Summary.OpenConflicts != 0 || state.Summary.ResolvedConflicts != 0 {
		t.Fatalf("initial summary = %#v, want no diffs or conflicts", state.Summary)
	}
	if !hasWarning(state.Warnings, "base-snapshots-initialized") {
		t.Fatalf("initial warnings = %#v, want base-snapshots-initialized", state.Warnings)
	}
	for _, want := range []string{
		"agent-canon sync: claude -> codex",
		"Project: " + fixture.project,
		"Workspace: " + workspaceRoot,
		"State: " + statePath,
		"Summary: diffs=0 open=0 resolved=0 warnings=1",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q in %q", want, stdout.String())
		}
	}
	if !reflect.DeepEqual(directorySnapshot(t, fixture.claudeHome), claudeBefore) {
		t.Fatalf("sync modified Claude home")
	}
	if !reflect.DeepEqual(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("sync modified Codex home")
	}
}

func TestRunSyncSecondRunGeneratesContentConflict(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")

	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("initial sync exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second sync exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	state := readSyncState(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))
	if state.Summary.OpenConflicts != 1 || len(state.Conflicts) != 1 {
		t.Fatalf("conflicts = %#v summary=%#v, want one open conflict", state.Conflicts, state.Summary)
	}
	conflict := state.Conflicts[0]
	if conflict.ID != "conflict-001" || conflict.Kind != model.ConflictKindContent || conflict.ResourceID != "instruction:project-claude-md" || conflict.Status != model.ConflictStatusOpen {
		t.Fatalf("conflict = %#v", conflict)
	}
	if !strings.Contains(stdout.String(), "Summary: diffs=1 open=1 resolved=0") {
		t.Fatalf("stdout missing conflict summary: %q", stdout.String())
	}
}

func TestRunSyncSecondRunDetectsCodexMCPConfigMergeConflict(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	writeFile(t, filepath.Join(fixture.project, ".claude", "settings.json"), `{
  "mcpServers": {
    "shared": {
      "command": "claude-shared",
      "args": ["--from", "claude"]
    }
  }
}
`)
	writeFile(t, filepath.Join(fixture.project, ".codex", "config.toml"), `[mcp_servers.shared]
command = "codex-shared"
args = ["--from", "codex"]
`)
	runInitialSync(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	state := readSyncState(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))
	conflict := requireConfigMergeConflict(t, state.Conflicts, "shared")
	if conflict.ID != "conflict-001" || conflict.Status != model.ConflictStatusOpen || !conflict.RequiresUserDecision {
		t.Fatalf("config conflict = %#v, want first open conflict requiring decision", conflict)
	}
	if conflict.Scope != model.ScopeProject || conflict.ResourceID != "mcp:project-shared" {
		t.Fatalf("config conflict identity = %#v", conflict)
	}
	if state.Summary.OpenConflicts != 1 || state.Summary.ResolvedConflicts != 0 {
		t.Fatalf("summary = %#v, want one open config conflict", state.Summary)
	}
	if !strings.Contains(stdout.String(), "Summary: diffs=0 open=1 resolved=0") {
		t.Fatalf("stdout missing config conflict summary: %q", stdout.String())
	}
}

func TestRunSyncRealWorldConflictFixtureKeepsConfigConflictIdentityStable(t *testing.T) {
	fixture := copiedFixture(t, "real-world-conflict")
	runInitialSync(t, fixture)
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")

	state := readSyncState(t, syncStatePath)
	conflict := requireConfigMergeConflict(t, state.Conflicts, "shared")
	if conflict.ID != "conflict-001" || conflict.Kind != model.ConflictKindConfigMerge || conflict.ResourceID != "mcp:project-shared" || conflict.Scope != model.ScopeProject || conflict.Status != model.ConflictStatusOpen || !conflict.RequiresUserDecision {
		t.Fatalf("config conflict = %#v", conflict)
	}
	if state.Summary.OpenConflicts != 1 || state.Summary.ResolvedConflicts != 0 {
		t.Fatalf("summary = %#v, want one open config conflict", state.Summary)
	}

	runInitialSync(t, fixture)
	updated := readSyncState(t, syncStatePath)
	preserved := requireConfigMergeConflict(t, updated.Conflicts, "shared")
	if preserved.ID != conflict.ID || preserved.Fingerprint != conflict.Fingerprint {
		t.Fatalf("preserved config conflict = %#v, want ID %q fingerprint %q", preserved, conflict.ID, conflict.Fingerprint)
	}
}

func TestRunSyncIgnoresLocalCodexMCPConfigMergeConflict(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	writeFile(t, filepath.Join(fixture.project, ".claude", "settings.local.json"), `{
  "mcpServers": {
    "shared": {
      "command": "claude-local-shared",
      "args": ["--from", "claude-local"]
    }
  }
}
`)
	writeFile(t, filepath.Join(fixture.project, ".codex", "config.toml"), `[mcp_servers.shared]
command = "codex-shared"
args = ["--from", "codex"]
`)
	runInitialSync(t, fixture)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	state := readSyncState(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))
	if got := countConfigMergeConflicts(state.Conflicts); got != 0 {
		t.Fatalf("local MCP config merge conflicts = %d, want 0; conflicts=%#v", got, state.Conflicts)
	}
	if state.Summary.OpenConflicts != 0 || state.Summary.ResolvedConflicts != 0 {
		t.Fatalf("summary = %#v, want no local config merge conflict", state.Summary)
	}
}

func TestRunSyncPreservesResolvedCodexMCPConfigMergeConflict(t *testing.T) {
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
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	state := readSyncState(t, syncStatePath)
	conflict := requireConfigMergeConflict(t, state.Conflicts, "shared")
	for i := range state.Conflicts {
		if state.Conflicts[i].Fingerprint == conflict.Fingerprint {
			state.Conflicts[i].Status = model.ConflictStatusResolved
			state.Conflicts[i].RequiresUserDecision = false
			state.Conflicts[i].ResolutionID = "resolution-config-shared"
		}
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal sync state: %v", err)
	}
	writeFile(t, syncStatePath, string(payload)+"\n")

	runInitialSync(t, fixture)
	updated := readSyncState(t, syncStatePath)
	preserved := requireConfigMergeConflict(t, updated.Conflicts, "shared")
	if preserved.Status != model.ConflictStatusResolved || preserved.RequiresUserDecision || preserved.ResolutionID != "resolution-config-shared" {
		t.Fatalf("preserved config conflict = %#v", preserved)
	}
	if updated.Summary.OpenConflicts != 0 || updated.Summary.ResolvedConflicts != 1 {
		t.Fatalf("summary = %#v, want resolved config conflict preserved", updated.Summary)
	}
}

func TestRunSyncKeepsResolvedConfigConflictIDWhenNewSemanticConflictAppears(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")
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
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	state := readSyncState(t, syncStatePath)
	configConflict := requireConfigMergeConflict(t, state.Conflicts, "shared")
	if configConflict.ID != "conflict-001" {
		t.Fatalf("initial config conflict ID = %q, want conflict-001", configConflict.ID)
	}
	for i := range state.Conflicts {
		if state.Conflicts[i].Fingerprint == configConflict.Fingerprint {
			state.Conflicts[i].Status = model.ConflictStatusResolved
			state.Conflicts[i].RequiresUserDecision = false
			state.Conflicts[i].ResolutionID = "resolution-config-shared"
		}
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal sync state: %v", err)
	}
	writeFile(t, syncStatePath, string(payload)+"\n")
	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")

	runInitialSync(t, fixture)
	updated := readSyncState(t, syncStatePath)
	preserved := requireConfigMergeConflict(t, updated.Conflicts, "shared")
	if preserved.ID != "conflict-001" || preserved.Status != model.ConflictStatusResolved || preserved.RequiresUserDecision || preserved.ResolutionID != "resolution-config-shared" {
		t.Fatalf("preserved config conflict = %#v", preserved)
	}
	assertUniqueConflictIDs(t, updated.Conflicts)
	if updated.Summary.OpenConflicts != 1 || updated.Summary.ResolvedConflicts != 1 {
		t.Fatalf("summary = %#v, want one open semantic conflict and one resolved config conflict", updated.Summary)
	}
}

func TestRunSyncInitialBaseSnapshotSkipsCodexMCPConfigConflictDetection(t *testing.T) {
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
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("initial sync exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	state := readSyncState(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))
	if got := countConfigMergeConflicts(state.Conflicts); got != 0 {
		t.Fatalf("initial sync config merge conflicts = %d, want 0; conflicts=%#v", got, state.Conflicts)
	}
	if state.Summary.OpenConflicts != 0 || !hasWarning(state.Warnings, "base-snapshots-initialized") {
		t.Fatalf("initial state summary=%#v warnings=%#v", state.Summary, state.Warnings)
	}
}

func TestRunSyncFormatJSONPrintsSyncState(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	var state model.SyncStateReport
	if err := json.Unmarshal(stdout.Bytes(), &state); err != nil {
		t.Fatalf("stdout is not valid sync JSON: %v\n%s", err, stdout.String())
	}
	if state.SchemaVersion != model.SyncStateSchemaVersion || state.Summary.OpenConflicts != 0 {
		t.Fatalf("sync JSON state = %#v", state)
	}
}

func TestRunSyncMalformedSettingsJSONReturnsExitTwo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunSyncRejectsSymlinkedWorkspaceEscape(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	outside := filepath.Join(root, "outside")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{}")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)
	mustMkdir(t, outside)
	if err := os.Symlink(outside, filepath.Join(project, ".agent-canon")); err != nil {
		t.Fatalf("create workspace symlink: %v", err)
	}
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "outside project") {
		t.Fatalf("stderr missing workspace boundary context: %q", stderr.String())
	}
	assertPathMissing(t, filepath.Join(outside, "sync-state.json"))
}

func TestRunSyncDoesNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("sync output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	workspaceText := readTreeText(t, filepath.Join(fixture.project, ".agent-canon"))
	if strings.Contains(workspaceText, secret) {
		t.Fatalf("workspace state leaked secret")
	}
}

func copiedFixture(t *testing.T, name string) fixturePaths {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	copyDir(t, filepath.Join(repoRoot, "testdata", name), root)
	return fixturePaths{
		home:       root,
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

func readSyncState(t *testing.T, path string) model.SyncStateReport {
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

func hasWarning(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func requireConfigMergeConflict(t *testing.T, conflicts []model.Conflict, serverName string) model.Conflict {
	t.Helper()
	for _, conflict := range conflicts {
		if conflict.Kind == model.ConflictKindConfigMerge && conflict.Details["serverName"] == serverName {
			return conflict
		}
	}
	t.Fatalf("missing ConfigMergeConflict for MCP server %q in %#v", serverName, conflicts)
	return model.Conflict{}
}

func countConfigMergeConflicts(conflicts []model.Conflict) int {
	count := 0
	for _, conflict := range conflicts {
		if conflict.Kind == model.ConflictKindConfigMerge {
			count++
		}
	}
	return count
}

func assertUniqueConflictIDs(t *testing.T, conflicts []model.Conflict) {
	t.Helper()
	seen := map[string]bool{}
	for _, conflict := range conflicts {
		if conflict.ID == "" {
			t.Fatalf("conflict has empty ID: %#v", conflict)
		}
		if seen[conflict.ID] {
			t.Fatalf("duplicate conflict ID %q in %#v", conflict.ID, conflicts)
		}
		seen[conflict.ID] = true
	}
}

func directorySnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
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
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(payload)
		return nil
	}); err != nil {
		t.Fatalf("snapshot directory %s: %v", root, err)
	}
	return out
}

func readTreeText(t *testing.T, root string) string {
	t.Helper()
	var builder strings.Builder
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		builder.Write(payload)
		builder.WriteByte('\n')
		return nil
	}); err != nil {
		t.Fatalf("read tree %s: %v", root, err)
	}
	return builder.String()
}
