package snapshot

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
)

func TestBuildCreatesClaudeCodexAndCanonSnapshots(t *testing.T) {
	project := t.TempDir()
	claudePath := writeSnapshotFile(t, project, "CLAUDE.md", "Hello Claude  \r\n\r\n")
	codexPath := writeSnapshotFile(t, project, "AGENTS.md", "Hello Codex\n")
	report := model.ScanReport{
		Source:  "claude",
		Target:  "codex",
		Project: project,
		Resources: []model.Resource{{
			ID:             "instruction:project-claude-md",
			Kind:           model.KindInstruction,
			Scope:          model.ScopeProject,
			SourcePath:     claudePath,
			TargetPathHint: codexPath,
			Status:         model.StatusCompatible,
			Strategy:       "merge-section-into-agents-md",
		}},
	}

	set, err := Build(report)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if set.Claude.SchemaVersion != model.SnapshotSchemaVersion || set.Claude.Tool != "claude" {
		t.Fatalf("Claude snapshot metadata = %#v", set.Claude)
	}
	if set.Codex.SchemaVersion != model.SnapshotSchemaVersion || set.Codex.Tool != "codex" {
		t.Fatalf("Codex snapshot metadata = %#v", set.Codex)
	}
	if set.Canon.SchemaVersion != model.CanonSnapshotSchemaVersion {
		t.Fatalf("Canon snapshot schema = %q", set.Canon.SchemaVersion)
	}
	if len(set.Claude.Resources) != 1 || len(set.Codex.Resources) != 1 || len(set.Canon.Resources) != 1 {
		t.Fatalf("resource counts claude=%d codex=%d canon=%d", len(set.Claude.Resources), len(set.Codex.Resources), len(set.Canon.Resources))
	}
	if got := set.Claude.Resources[0].NormalizedText; got != "Hello Claude" {
		t.Fatalf("Claude normalized text = %q", got)
	}
	if got := set.Codex.Resources[0].NormalizedText; got != "Hello Codex" {
		t.Fatalf("Codex normalized text = %q", got)
	}
	if set.Claude.Resources[0].ContentHash == "" || set.Codex.Resources[0].ContentHash == "" {
		t.Fatalf("snapshots missing content hashes: %#v %#v", set.Claude.Resources[0], set.Codex.Resources[0])
	}
	if set.Canon.Resources[0].Tool != "canon" || set.Canon.Resources[0].TargetPathHint != "" || set.Canon.Resources[0].ContentHash == "" {
		t.Fatalf("Canon resource = %#v", set.Canon.Resources[0])
	}
}

func TestBuildSkipsMissingCodexTargets(t *testing.T) {
	project := t.TempDir()
	claudePath := writeSnapshotFile(t, project, "CLAUDE.md", "Hello\n")
	report := model.ScanReport{Project: project, Resources: []model.Resource{{
		ID:             "instruction:project-claude-md",
		Kind:           model.KindInstruction,
		Scope:          model.ScopeProject,
		SourcePath:     claudePath,
		TargetPathHint: filepath.Join(project, "AGENTS.md"),
		Status:         model.StatusCompatible,
		Strategy:       "merge-section-into-agents-md",
	}}}

	set, err := Build(report)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(set.Claude.Resources) != 1 {
		t.Fatalf("Claude resources = %d, want 1", len(set.Claude.Resources))
	}
	if len(set.Codex.Resources) != 0 {
		t.Fatalf("Codex resources = %#v, want none for missing target", set.Codex.Resources)
	}
}

func TestBuildRedactsDangerousResourceContent(t *testing.T) {
	project := t.TempDir()
	secret := "github_pat_11ABCDEFG0abcdefghijklmnopqrstuvwxyz_1234567890ABCDE"
	commandPath := writeSnapshotFile(t, project, "dangerous.md", "GITHUB_TOKEN="+secret+"\nrun deploy\n")
	report := model.ScanReport{Project: project, Resources: []model.Resource{{
		ID:         "command:project-dangerous",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: commandPath,
		Status:     model.StatusDangerous,
		Strategy:   "manual-redacted-command-secret",
	}}}

	set, err := Build(report)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	state := set.Claude.Resources[0]
	if strings.Contains(state.NormalizedText, secret) {
		t.Fatalf("snapshot leaked secret in %#v", state)
	}
	if !strings.Contains(state.NormalizedText, "<REDACTED>") {
		t.Fatalf("snapshot missing redaction marker in %#v", state)
	}
	if !hasWarning(state.Warnings, "secret-redacted") {
		t.Fatalf("snapshot warnings = %#v, want secret-redacted", state.Warnings)
	}
}

func TestBuildKeepsMemoryItemsMetadataOnly(t *testing.T) {
	project := t.TempDir()
	memoryPath := writeSnapshotFile(t, project, "memory.md", "remember GITHUB_TOKEN=fixture-secret\n")
	report := model.ScanReport{Project: project, Resources: []model.Resource{{
		ID:         "memory:project-example",
		Kind:       model.KindMemoryItem,
		Scope:      model.ScopeProject,
		SourcePath: memoryPath,
		Status:     model.StatusPartial,
		Strategy:   "review-memory-candidate",
	}}}

	set, err := Build(report)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	state := set.Claude.Resources[0]
	if state.NormalizedText != "" || state.ContentHash != "" {
		t.Fatalf("memory state stored content: %#v", state)
	}
	if state.Path != memoryPath || state.Tool != "claude" {
		t.Fatalf("memory metadata missing: %#v", state)
	}
}

func TestBuildCapturesExistingCodexFixtureTargets(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "basic")
	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	set, err := Build(report)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if !hasStateWithBase(set.Codex.Resources, "AGENTS.md") {
		t.Fatalf("Codex snapshot missing existing AGENTS.md target: %#v", set.Codex.Resources)
	}
	if !hasStateWithBase(set.Codex.Resources, "config.toml") {
		t.Fatalf("Codex snapshot missing existing config.toml target: %#v", set.Codex.Resources)
	}
}

func TestBuildCapturesCodexTargetsImpliedByWarnings(t *testing.T) {
	project := t.TempDir()
	codexHome := t.TempDir()
	claudePath := writeSnapshotFile(t, project, "CLAUDE.md", "Project instructions\n")
	globalAgents := writeSnapshotFile(t, codexHome, "AGENTS.md", "Global Codex instructions\n")
	report := model.ScanReport{Project: project, CodexHome: codexHome, Resources: []model.Resource{{
		ID:             "instruction:project-claude-md",
		Kind:           model.KindInstruction,
		Scope:          model.ScopeProject,
		SourcePath:     claudePath,
		TargetPathHint: filepath.Join(project, "AGENTS.md"),
		Status:         model.StatusCompatible,
		Strategy:       "merge-section-into-agents-md",
		Warnings: []model.Warning{{
			Code:    "existing-codex-target",
			Message: "Codex target already exists and should be reviewed before merging: " + globalAgents,
		}},
	}}}

	set, err := Build(report)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !hasStateWithPath(set.Codex.Resources, globalAgents) {
		t.Fatalf("Codex snapshot missing warning-implied target %s: %#v", globalAgents, set.Codex.Resources)
	}
}

func TestBuildCanonResourcesAreDeterministicIdentities(t *testing.T) {
	first := canonFixtureReport(t)
	second := canonFixtureReport(t)

	firstSet, err := Build(first)
	if err != nil {
		t.Fatalf("Build first returned error: %v", err)
	}
	secondSet, err := Build(second)
	if err != nil {
		t.Fatalf("Build second returned error: %v", err)
	}
	firstState := firstSet.Canon.Resources[0]
	secondState := secondSet.Canon.Resources[0]
	if firstState.Path != "" || firstState.TargetPathHint != "" || firstState.NormalizedText != "" {
		t.Fatalf("canon state contains path or content: %#v", firstState)
	}
	if firstState.ContentHash == "" {
		t.Fatalf("canon state missing identity hash: %#v", firstState)
	}
	if !reflect.DeepEqual(firstState, secondState) {
		t.Fatalf("canon states differ across directories:\nfirst=%#v\nsecond=%#v", firstState, secondState)
	}
}

func canonFixtureReport(t *testing.T) model.ScanReport {
	t.Helper()
	project := t.TempDir()
	source := writeSnapshotFile(t, project, "CLAUDE.md", "same content\n")
	return model.ScanReport{Project: project, Resources: []model.Resource{{
		ID:             "instruction:project-claude-md",
		Kind:           model.KindInstruction,
		Scope:          model.ScopeProject,
		SourcePath:     source,
		TargetPathHint: filepath.Join(project, "AGENTS.md"),
		Status:         model.StatusCompatible,
		Strategy:       "merge-section-into-agents-md",
	}}}
}

func writeSnapshotFile(t *testing.T, root string, relative string, contents string) string {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

func hasWarning(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func hasStateWithBase(states []model.ResourceState, base string) bool {
	for _, state := range states {
		if filepath.Base(state.Path) == base && state.ContentHash != "" {
			return true
		}
	}
	return false
}

func hasStateWithPath(states []model.ResourceState, path string) bool {
	for _, state := range states {
		if state.Path == path && state.ContentHash != "" {
			return true
		}
	}
	return false
}
