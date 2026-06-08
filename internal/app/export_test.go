package app_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
)

func TestRunExportCodexWritesPreviewAndPrintsShortSummary(t *testing.T) {
	fixture := basicFixture(t)
	outDir := filepath.Join(t.TempDir(), "preview")
	mustMkdir(t, outDir)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	for _, path := range []string{
		"AGENTS.md",
		filepath.Join(".codex", "config.toml"),
		filepath.Join(".agents", "skills", "sample-skill", "SKILL.md"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	project, err := filepath.Abs(fixture.project)
	if err != nil {
		t.Fatalf("resolve fixture project: %v", err)
	}
	for _, want := range []string{
		"agent-canon export: claude -> codex",
		"Project: " + project,
		"wrote Codex preview to " + outDir + " (4 files)",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q in %q", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "# AGENTS.md preview") || strings.Contains(stdout.String(), "# Migration report") {
		t.Fatalf("stdout contains full preview contents, want short summary only: %q", stdout.String())
	}
}

func TestRunExportClaudeWritesPreviewAndPrintsShortSummary(t *testing.T) {
	fixture := basicFixture(t)
	outDir := filepath.Join(t.TempDir(), "claude-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	for _, path := range []string{
		"CLAUDE.md",
		filepath.Join(".claude", "settings.json"),
		filepath.Join(".claude", "skills", "sample-skill", "SKILL.md"),
		"migration-report.md",
	} {
		assertFileExists(t, filepath.Join(outDir, path))
	}
	project, err := filepath.Abs(fixture.project)
	if err != nil {
		t.Fatalf("resolve fixture project: %v", err)
	}
	for _, want := range []string{
		"agent-canon export claude",
		"Project: " + project,
		"wrote Claude preview to " + outDir,
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q in %q", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "# CLAUDE.md preview") || strings.Contains(stdout.String(), "# Migration report") {
		t.Fatalf("stdout contains full preview contents, want short summary only: %q", stdout.String())
	}
}

func TestRunExportCodexAppliesProjectSkipConfigBeforePreview(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	outDir := filepath.Join(root, "preview")

	writeFile(t, filepath.Join(claudeHome, "commands", "ccs"), "# CCS command wrapper\n")
	writeFile(t, filepath.Join(claudeHome, "commands", "ccs.md"), "# CCS slash command\n")
	writeFile(t, filepath.Join(project, ".agent-canon", "config.toml"), `[skip]
resources = ["command:global-ccs"]
`)
	mustMkdir(t, codexHome)

	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"export", "codex", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome, "--out", outDir}, project, root, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	assertFileExists(t, filepath.Join(outDir, "AGENTS.md"))
	assertFileExists(t, filepath.Join(outDir, ".codex", "config.toml"))
	assertFileExists(t, filepath.Join(outDir, "migration-report.md"))
	if _, err := os.Stat(filepath.Join(outDir, ".agents", "skills", "ccs", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("skipped CCS preview exists or stat failed unexpectedly: %v", err)
	}
}

func TestRunExportMalformedSettingsJSONReturnsExitTwo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)
	outDir := filepath.Join(root, "preview")

	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"export", "codex", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome, "--out", outDir}, project, root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunExportNonEmptyOutputDirReturnsExitOne(t *testing.T) {
	fixture := basicFixture(t)
	outDir := t.TempDir()
	existing := filepath.Join(outDir, "existing.txt")
	writeFile(t, existing, "keep\n")
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
}

func TestRunExportEmptyOutputPathReturnsExitOne(t *testing.T) {
	fixture := basicFixture(t)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", "   "}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "preview root is empty") {
		t.Fatalf("stderr missing WritePreview error, got %q", stderr.String())
	}
}

func TestRunExportRejectsSymlinkedOutputInsideClaudeOrCodexHome(t *testing.T) {
	cases := []struct {
		name   string
		target func(fixturePaths) string
	}{
		{
			name: "claude home descendant",
			target: func(f fixturePaths) string {
				return filepath.Join(f.claudeHome, "preview-target")
			},
		},
		{
			name: "codex home descendant",
			target: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview-target")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fixture := fixturePaths{
				home:       root,
				project:    filepath.Join(root, "project"),
				claudeHome: filepath.Join(root, "claude-home"),
				codexHome:  filepath.Join(root, "codex-home"),
			}
			mustMkdir(t, fixture.project)
			mustMkdir(t, fixture.claudeHome)
			mustMkdir(t, fixture.codexHome)
			target := tc.target(fixture)
			mustMkdir(t, target)
			link := filepath.Join(root, "out-link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatalf("create symlink: %v", err)
			}
			var stdout, stderr bytes.Buffer

			code := app.Run([]string{"export", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", link}, fixture.project, fixture.home, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "Claude or Codex home") {
				t.Fatalf("stderr missing home boundary context: %q", stderr.String())
			}
			assertPathMissing(t, filepath.Join(target, "AGENTS.md"))
		})
	}
}

func TestRunExportRejectsOutputInsideClaudeOrCodexHome(t *testing.T) {
	cases := []struct {
		name       string
		target     string
		out        func(fixturePaths) string
		writeCheck func(fixturePaths) string
	}{
		{
			name:   "codex to claude home",
			target: "codex",
			out: func(f fixturePaths) string {
				return f.claudeHome
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.claudeHome, "AGENTS.md")
			},
		},
		{
			name:   "codex to codex home",
			target: "codex",
			out: func(f fixturePaths) string {
				return f.codexHome
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "AGENTS.md")
			},
		},
		{
			name:   "codex inside codex home",
			target: "codex",
			out: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview")
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview", "AGENTS.md")
			},
		},
		{
			name:   "claude to claude home",
			target: "claude",
			out: func(f fixturePaths) string {
				return f.claudeHome
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.claudeHome, "CLAUDE.md")
			},
		},
		{
			name:   "claude inside codex home",
			target: "claude",
			out: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview")
			},
			writeCheck: func(f fixturePaths) string {
				return filepath.Join(f.codexHome, "preview", "CLAUDE.md")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			fixture := fixturePaths{
				home:       root,
				project:    filepath.Join(root, "project"),
				claudeHome: filepath.Join(root, "claude-home"),
				codexHome:  filepath.Join(root, "codex-home"),
			}
			mustMkdir(t, fixture.project)
			mustMkdir(t, fixture.claudeHome)
			mustMkdir(t, fixture.codexHome)
			var stdout, stderr bytes.Buffer

			code := app.Run([]string{"export", tc.target, "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", tc.out(fixture)}, fixture.project, fixture.home, &stdout, &stderr)
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

func TestRunExportClaudeDoesNotLeakSecrets(t *testing.T) {
	fixture := copiedFixture(t, "secrets")
	const secret = "ghp_agent_canon_fixture_secret_must_not_leak"
	outDir := filepath.Join(t.TempDir(), "claude-preview")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"export", "claude", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", outDir}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatalf("export output leaked secret; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	previewText := readTreeText(t, outDir)
	if strings.Contains(previewText, secret) {
		t.Fatalf("export preview leaked secret")
	}
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
