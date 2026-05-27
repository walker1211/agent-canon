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

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
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

func assertFilesUnchanged(t *testing.T, root string, before map[string]string) {
	t.Helper()
	after := snapshotFiles(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("fixture files changed under %s\nbefore: %#v\nafter: %#v", root, before, after)
	}
}

func assertDoesNotContainSecret(t *testing.T, value string, label string) {
	t.Helper()
	if strings.Contains(value, fixtureSecret) {
		t.Fatalf("%s leaked fixture secret", label)
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
