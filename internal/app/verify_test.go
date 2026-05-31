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

func TestRunVerifyCodexTextWarnOnlyExitsZeroAndWritesNothing(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"agent-canon verify codex", "Summary:", "Checks:", "warn"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %q", want, stdout.String())
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) || !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) || !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("verify codex modified fixture files")
	}
}

func TestRunVerifyCodexFormatJSONPrintsValidReport(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var report model.VerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid verify JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.VerifySchemaVersion || report.Target != "codex" {
		t.Fatalf("verify JSON metadata = %#v", report)
	}
}

func TestRunVerifyCodexMalformedConfigExitsOne(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	writeFile(t, filepath.Join(fixture.project, ".codex", "config.toml"), "[mcp_servers.github\ncommand = \"gh\"\n")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "fail codex-config-project") {
		t.Fatalf("stdout missing failed config check: %q", stdout.String())
	}
	want := "verify codex failed with 1 failed checks; see failed checks above, fix the target files or conflicts, then rerun verify"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr missing failure summary %q: %q", want, stderr.String())
	}
}

func TestRunVerifyCodexOpenConflictsExitOne(t *testing.T) {
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

	code := app.Run([]string{"verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "fail sync-conflicts") || !strings.Contains(stdout.String(), "open conflicts") {
		t.Fatalf("stdout missing open conflict failure: %q", stdout.String())
	}
}

func TestRunVerifyClaudeValidatesClaudeSide(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"verify", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "agent-canon verify claude") || !strings.Contains(stdout.String(), "claude-instructions-project") {
		t.Fatalf("stdout missing Claude verify checks: %q", stdout.String())
	}
}

func TestRunVerifyDoesNotLeakFixtureSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	var stdout, stderr bytes.Buffer

	_ = app.Run([]string{"verify", "codex", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("verify output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
