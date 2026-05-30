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

func TestRunResolveDecisionsUpdateStateAndLearnedResolutions(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		prepare   func(t *testing.T, fixture fixturePaths)
		decision  model.ResolutionDecision
		wantValue string
	}{
		{
			name:      "ours",
			args:      []string{"--ours"},
			decision:  model.ResolutionDecisionOurs,
			wantValue: "ours changed",
		},
		{
			name:      "theirs",
			args:      []string{"--theirs"},
			decision:  model.ResolutionDecisionTheirs,
			wantValue: "theirs changed",
		},
		{
			name:      "suggestion",
			args:      []string{"--accept-suggestion"},
			prepare:   addSuggestionToOpenConflict,
			decision:  model.ResolutionDecisionSuggestion,
			wantValue: "merged suggestion",
		},
		{
			name:      "manual",
			args:      []string{"--manual", "manual merged value"},
			decision:  model.ResolutionDecisionManual,
			wantValue: "manual merged value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := fixtureWithOpenConflict(t)
			if tt.prepare != nil {
				tt.prepare(t, fixture)
			}
			claudeBefore := directorySnapshot(t, fixture.claudeHome)
			codexBefore := directorySnapshot(t, fixture.codexHome)
			var stdout, stderr bytes.Buffer
			args := append([]string{"resolve", "conflict-001"}, tt.args...)
			args = append(args, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)

			code := app.Run(args, fixture.project, fixture.home, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
			}

			state := readSyncState(t, filepath.Join(fixture.project, ".agent-canon", "sync-state.json"))
			if state.Summary.OpenConflicts != 0 || state.Summary.ResolvedConflicts != 1 {
				t.Fatalf("summary = %#v", state.Summary)
			}
			resolved := state.Conflicts[0]
			if resolved.Status != model.ConflictStatusResolved || resolved.RequiresUserDecision || resolved.ResolutionID != "resolution-001" {
				t.Fatalf("resolved conflict = %#v", resolved)
			}
			learned := readLearnedResolutions(t, filepath.Join(fixture.project, ".agent-canon", "resolutions", "learned-resolutions.json"))
			if len(learned.Resolutions) != 1 {
				t.Fatalf("learned resolutions = %#v", learned.Resolutions)
			}
			resolution := learned.Resolutions[0]
			if resolution.ID != "resolution-001" || resolution.Decision != tt.decision || resolution.Value != tt.wantValue || resolution.ConflictFingerprint != resolved.Fingerprint {
				t.Fatalf("learned resolution = %#v, conflict=%#v", resolution, resolved)
			}
			if !strings.Contains(stdout.String(), "resolved conflict-001 with "+string(tt.decision)+" as resolution-001") {
				t.Fatalf("stdout missing confirmation: %q", stdout.String())
			}
			if strings.Contains(stdout.String(), tt.wantValue) {
				t.Fatalf("stdout leaked resolved value: %q", stdout.String())
			}
			if !reflect.DeepEqual(directorySnapshot(t, fixture.claudeHome), claudeBefore) {
				t.Fatalf("resolve modified Claude home")
			}
			if !reflect.DeepEqual(directorySnapshot(t, fixture.codexHome), codexBefore) {
				t.Fatalf("resolve modified Codex home")
			}
		})
	}
}

func TestRunResolveManualDoesNotPersistSecretValue(t *testing.T) {
	fixture := fixtureWithOpenConflict(t)
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"resolve", "conflict-001", "--manual", "GITHUB_TOKEN=" + secret, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("resolve output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	workspaceText := readTreeText(t, filepath.Join(fixture.project, ".agent-canon"))
	if strings.Contains(workspaceText, secret) {
		t.Fatalf("resolve workspace state leaked secret")
	}
	if !strings.Contains(workspaceText, "REDACTED") {
		t.Fatalf("resolve workspace state missing redaction marker")
	}
}

func TestRunResolveMissingSyncStateReturnsExitOne(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	mustMkdir(t, project)
	mustMkdir(t, claudeHome)
	mustMkdir(t, codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"resolve", "conflict-001", "--ours", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "no sync state found; run \"agent-canon sync claude codex\" first") {
		t.Fatalf("stderr missing missing-state guidance: %q", stderr.String())
	}
}

func TestRunResolveAcceptSuggestionRequiresSuggestion(t *testing.T) {
	fixture := fixtureWithOpenConflict(t)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"resolve", "conflict-001", "--accept-suggestion", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "suggestion") {
		t.Fatalf("stderr missing suggestion context: %q", stderr.String())
	}
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon", "resolutions", "learned-resolutions.json"))
}

func TestRunResolveConfigMergeManualReturnsClearErrorWithoutLearnedResolutions(t *testing.T) {
	fixture := fixtureWithOpenConflict(t)
	setOpenConflictAsConfigMerge(t, fixture)
	workspaceBefore := directorySnapshot(t, filepath.Join(fixture.project, ".agent-canon"))
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"resolve", "conflict-001", "--manual", "[mcp_servers.example]\ncommand = \"example\"", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	const want = "manual TOML resolution is not supported for Codex MCP config merge conflicts"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want to contain %q", stderr.String(), want)
	}
	if !reflect.DeepEqual(directorySnapshot(t, filepath.Join(fixture.project, ".agent-canon")), workspaceBefore) {
		t.Fatalf("resolve mutated workspace state after rejected config merge manual resolution")
	}
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon", "resolutions", "learned-resolutions.json"))
}

func addSuggestionToOpenConflict(t *testing.T, fixture fixturePaths) {
	t.Helper()
	path := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	state := readSyncState(t, path)
	state.Conflicts[0].Suggestion = "merged suggestion"
	state.Conflicts[0].SuggestionConfidence = 1
	writeSyncState(t, path, state)
}

func setOpenConflictAsConfigMerge(t *testing.T, fixture fixturePaths) {
	t.Helper()
	path := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	state := readSyncState(t, path)
	state.Conflicts[0].Kind = model.ConflictKindConfigMerge
	state.Conflicts[0].ResourceID = "mcp:project-example"
	state.Conflicts[0].ResourceKind = model.KindMCPServer
	state.Conflicts[0].Ours = &model.ResourceState{ID: "mcp:project-example", Kind: model.KindMCPServer, Scope: model.ScopeProject, Tool: "claude", ContentHash: "ours", NormalizedText: "Codex MCP ours summary"}
	state.Conflicts[0].Theirs = &model.ResourceState{ID: "mcp:project-example", Kind: model.KindMCPServer, Scope: model.ScopeProject, Tool: "codex", ContentHash: "theirs", NormalizedText: "Codex MCP theirs summary"}
	state.Conflicts[0].Details = map[string]string{
		"serverName": "example",
		"targetPath": filepath.Join(fixture.project, ".codex", "config.toml"),
		"reason":     "same-name Codex MCP server exists with different normalized configuration",
	}
	writeSyncState(t, path, state)
}

func writeSyncState(t *testing.T, path string, state model.SyncStateReport) {
	t.Helper()
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal sync state: %v", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write sync state: %v", err)
	}
}

func readLearnedResolutions(t *testing.T, path string) model.LearnedResolutionReport {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read learned resolutions: %v", err)
	}
	var report model.LearnedResolutionReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("unmarshal learned resolutions: %v\n%s", err, string(payload))
	}
	return report
}
