package app_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
)

const lifecycleFixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestRunInitStatusDiffDoNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	for _, args := range [][]string{
		{"init", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		{"init", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
	} {
		var stdout, stderr bytes.Buffer
		code := app.Run(args, fixture.project, fixture.home, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v exit code = %d, want 0; stderr=%q", args, code, stderr.String())
		}
		assertNoLifecycleSecret(t, stdout.String(), stderr.String())
	}

	runInitialSync(t, fixture)
	for _, args := range [][]string{
		{"status", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		{"status", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		{"diff", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
		{"diff", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome},
	} {
		var stdout, stderr bytes.Buffer
		code := app.Run(args, fixture.project, fixture.home, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("%v exit code = %d, want 0; stderr=%q", args, code, stderr.String())
		}
		assertNoLifecycleSecret(t, stdout.String(), stderr.String())
	}
	if strings.Contains(readTreeText(t, filepath.Join(fixture.project, ".agent-canon")), lifecycleFixtureSecret) {
		t.Fatalf("workspace state leaked fixture secret")
	}
}

func TestRunDiffDoesNotWriteRollbackBackupsOrLearnedResolutions(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	syncStatePath := filepath.Join(fixture.project, ".agent-canon", "sync-state.json")
	beforeState := readFileText(t, syncStatePath)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"diff", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("diff exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if readFileText(t, syncStatePath) != beforeState {
		t.Fatalf("diff updated sync state")
	}
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon", "rollback"))
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon", "backups"))
	assertPathMissing(t, filepath.Join(fixture.project, ".agent-canon", "resolutions", "learned-resolutions.json"))
}

func assertNoLifecycleSecret(t *testing.T, values ...string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(value, lifecycleFixtureSecret) {
			t.Fatalf("output leaked fixture secret: %q", value)
		}
	}
}
