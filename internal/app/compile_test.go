package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
)

func TestRunCompileCodexRequiresCanonBaseline(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "canon baseline") || !strings.Contains(stderr.String(), "run \"agent-canon sync claude codex\" first") {
		t.Fatalf("stderr missing baseline guidance: %q", stderr.String())
	}
	assertPathMissing(t, outDir)
}

func TestRunCompileCodexRequiresSyncState(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	if err := os.Remove(filepath.Join(fixture.project, ".agent-canon", "sync-state.json")); err != nil {
		t.Fatalf("remove sync state: %v", err)
	}
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "sync state") || !strings.Contains(stderr.String(), "run \"agent-canon sync claude codex\" first") {
		t.Fatalf("stderr missing sync guidance: %q", stderr.String())
	}
	assertPathMissing(t, outDir)
}

func TestRunCompileCodexBlocksOpenConflicts(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")
	runInitialSync(t, fixture)
	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")
	runInitialSync(t, fixture)
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "open conflicts") || !strings.Contains(stderr.String(), "agent-canon conflicts") || !strings.Contains(stderr.String(), "agent-canon resolve") {
		t.Fatalf("stderr missing conflict guidance: %q", stderr.String())
	}
	assertPathMissing(t, outDir)
}

func TestRunCompileCodexRejectsOutputInsideClaudeOrCodexHome(t *testing.T) {
	cases := []struct {
		name       string
		out        func(fixturePaths) string
		writeCheck func(fixturePaths) string
	}{
		{
			name: "claude home",
			out: func(f fixturePaths) string {
				return f.claudeHome
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.claudeHome, "migration-report.md")
			},
		},
		{
			name: "codex home",
			out: func(f fixturePaths) string {
				return f.codexHome
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "migration-report.md")
			},
		},
		{
			name: "inside codex home",
			out: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview")
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview", "AGENTS.md")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := copiedFixture(t, "basic")
			runInitialSync(t, fixture)
			var stdout, stderr bytes.Buffer

			code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", tc.out(fixture)}, fixture.project, fixture.home, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "Claude or Codex home") {
				t.Fatalf("stderr missing home boundary context: %q", stderr.String())
			}
			assertPathMissing(t, tc.writeCheck(fixture))
		})
	}
}

func TestRunCompileCodexWritesPreviewAndDoesNotModifyInputs(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	runInitialSync(t, fixture)
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	canonPath := filepath.Join(workspaceRoot, "base", "canon.snapshot.json")
	syncStatePath := filepath.Join(workspaceRoot, "sync-state.json")
	workspaceBefore := directorySnapshot(t, workspaceRoot)
	projectBefore := directorySnapshot(t, fixture.project)
	claudeBefore := directorySnapshot(t, fixture.claudeHome)
	codexBefore := directorySnapshot(t, fixture.codexHome)
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	for _, path := range []string{
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	for _, want := range []string{
		"agent-canon compile codex",
		"Project: " + fixture.project,
		"Workspace: " + workspaceRoot,
		"Canon snapshot: " + canonPath,
		"Sync state: " + syncStatePath,
		"Output: " + outDir,
		"Summary: files=4",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
	if !equalStringMaps(directorySnapshot(t, workspaceRoot), workspaceBefore) {
		t.Fatalf("compile modified .agent-canon")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.project), projectBefore) {
		t.Fatalf("compile modified project files")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.claudeHome), claudeBefore) {
		t.Fatalf("compile modified Claude home")
	}
	if !equalStringMaps(directorySnapshot(t, fixture.codexHome), codexBefore) {
		t.Fatalf("compile modified Codex home")
	}
}

func TestRunCompileCodexDoesNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	runInitialSync(t, fixture)
	outDir := filepath.Join(t.TempDir(), "compiled")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"compile", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("compile output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	previewText := readTreeText(t, outDir)
	if strings.Contains(previewText, secret) {
		t.Fatalf("compile preview leaked secret")
	}
}
