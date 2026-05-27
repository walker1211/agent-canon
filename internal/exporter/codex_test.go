package exporter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/exporter"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
)

const fixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestBuildCodexPreviewBasicFixtureGeneratesExpectedFiles(t *testing.T) {
	preview := buildPreview(t, "basic")

	requireFile(t, preview, "AGENTS.md")
	requireFile(t, preview, ".codex/config.toml")
	requireFile(t, preview, ".agents/skills/sample-skill/SKILL.md")
	requireFile(t, preview, "migration-report.md")
}

func TestBuildCodexPreviewAgentsContainsInstructionsAndRulesWithoutAbsolutePaths(t *testing.T) {
	preview := buildPreview(t, "basic")
	agents := string(requireFile(t, preview, "AGENTS.md").Contents)

	for _, want := range []string{
		"# Global Claude Instructions",
		"Prefer concise answers and explain tradeoffs when changing configuration.",
		"# Project Instructions",
		"This fixture project is read-only input for scanner tests.",
		"# Language",
		"Use English for public project files unless a user asks otherwise.",
	} {
		if !strings.Contains(agents, want) {
			t.Fatalf("AGENTS.md missing %q in:\n%s", want, agents)
		}
	}

	for _, resource := range scanFixture(t, "basic").Resources {
		if resource.SourcePath != "" && strings.Contains(agents, resource.SourcePath) {
			t.Fatalf("AGENTS.md contains absolute source path %q:\n%s", resource.SourcePath, agents)
		}
	}
}

func TestBuildCodexPreviewUnsupportedFixtureReportsSkippedReviewResourcesWithoutGeneratingThem(t *testing.T) {
	preview := buildPreview(t, "unsupported")
	report := string(requireFile(t, preview, "migration-report.md").Contents)

	for _, want := range []string{
		"hook:global-PreToolUse",
		"session:global-session-history",
		"skipped unsupported resources",
		"review-required resources",
		"no real Codex configuration was written",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("migration report missing %q in:\n%s", want, report)
		}
	}

	for _, file := range preview.Files {
		if strings.Contains(file.Path, "PreToolUse") || strings.Contains(file.Path, "session-history") {
			t.Fatalf("unsupported resource unexpectedly generated preview file %q", file.Path)
		}
	}
}

func TestBuildCodexPreviewSecretsFixtureRedactsSecrets(t *testing.T) {
	preview := buildPreview(t, "secrets")

	for _, file := range preview.Files {
		if strings.Contains(string(file.Contents), fixtureSecret) {
			t.Fatalf("%s leaked fixture secret", file.Path)
		}
	}

	config := string(requireFile(t, preview, ".codex/config.toml").Contents)
	report := string(requireFile(t, preview, "migration-report.md").Contents)
	if !strings.Contains(config, "<REDACTED>") && !strings.Contains(config, "secret-redacted") {
		t.Fatalf("config missing redaction marker or warning:\n%s", config)
	}
	if !strings.Contains(report, "<REDACTED>") && !strings.Contains(report, "secret-redacted") {
		t.Fatalf("report missing redaction marker or warning:\n%s", report)
	}
}

func TestBuildCodexPreviewDoesNotGenerateUnsupportedSkillPreview(t *testing.T) {
	sourcePath := writeTempFile(t, filepath.Join("blocked-skill", "SKILL.md"), "# Blocked Skill\n\nDo not convert automatically.\n")
	preview := buildSyntheticPreview(t, model.Resource{
		ID:         "skill:global-blocked-skill",
		Kind:       model.KindSkill,
		Scope:      model.ScopeGlobal,
		SourcePath: sourcePath,
		Status:     model.StatusUnsupported,
		Strategy:   "skip-unsupported-skill",
	})

	assertNoFile(t, preview, ".agents/skills/blocked-skill/SKILL.md")
	report := string(requireFile(t, preview, "migration-report.md").Contents)
	if !strings.Contains(report, "skill:global-blocked-skill") {
		t.Fatalf("migration report missing unsupported skill resource:\n%s", report)
	}
}

func TestBuildCodexPreviewDoesNotGenerateDangerousCommandOrLeakSourceSecret(t *testing.T) {
	sourcePath := writeTempFile(t, "dangerous-command.md", "# Dangerous Command\n\nToken: "+fixtureSecret+"\n")
	preview := buildSyntheticPreview(t, model.Resource{
		ID:         "command:global-dangerous-command",
		Kind:       model.KindCommand,
		Scope:      model.ScopeGlobal,
		SourcePath: sourcePath,
		Status:     model.StatusDangerous,
		Strategy:   "manual-redacted-command-secret",
		Warnings:   []model.Warning{{Code: "secret-redacted", Message: "command source contains a redacted secret"}},
	})

	assertNoFile(t, preview, ".agents/skills/dangerous-command/SKILL.md")
	for _, file := range preview.Files {
		if strings.Contains(string(file.Contents), fixtureSecret) {
			t.Fatalf("%s leaked fixture secret", file.Path)
		}
	}
	report := string(requireFile(t, preview, "migration-report.md").Contents)
	if !strings.Contains(report, "command:global-dangerous-command") || !strings.Contains(report, "secret-redacted") {
		t.Fatalf("migration report missing dangerous command details:\n%s", report)
	}
}

func TestBuildCodexPreviewGeneratesPartialCommandPreview(t *testing.T) {
	sourcePath := writeTempFile(t, "deploy.md", "# Deploy\n\nRun deployment steps.\n")
	preview := buildSyntheticPreview(t, model.Resource{
		ID:         "command:project-deploy",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: sourcePath,
		Status:     model.StatusPartial,
		Strategy:   "convert-command-to-skill-or-workflow",
	})

	file := requireFile(t, preview, ".agents/skills/deploy/SKILL.md")
	contents := string(file.Contents)
	if !strings.Contains(contents, "Lossy command-to-skill conversion") || !strings.Contains(contents, "Run deployment steps.") {
		t.Fatalf("command preview missing conversion header or source content:\n%s", contents)
	}
}

func TestBuildCodexPreviewGeneratesPartialAgentPreview(t *testing.T) {
	sourcePath := writeTempFile(t, "reviewer.md", "# Reviewer\n\nReview code.\n")
	preview := buildSyntheticPreview(t, model.Resource{
		ID:         "agent:project-reviewer",
		Kind:       model.KindAgent,
		Scope:      model.ScopeProject,
		SourcePath: sourcePath,
		Status:     model.StatusPartial,
		Strategy:   "rewrite-agent-schema",
	})

	file := requireFile(t, preview, ".codex/agents/reviewer.toml")
	contents := string(file.Contents)
	if !strings.Contains(contents, "Review required") || !strings.Contains(contents, "agent:project-reviewer") {
		t.Fatalf("agent preview missing conservative review text:\n%s", contents)
	}
}

func TestBuildCodexPreviewReturnsErrorForDuplicatePreviewPaths(t *testing.T) {
	skillPath := writeTempFile(t, filepath.Join("foo", "SKILL.md"), "# Foo Skill\n")
	commandPath := writeTempFile(t, "foo.md", "# Foo Command\n")
	_, err := buildSyntheticPreviewResult(t, model.Resource{
		ID:         "skill:project-foo",
		Kind:       model.KindSkill,
		Scope:      model.ScopeProject,
		SourcePath: skillPath,
		Status:     model.StatusPartial,
		Strategy:   "convert-skill-with-review",
	}, model.Resource{
		ID:         "command:project-foo",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: commandPath,
		Status:     model.StatusPartial,
		Strategy:   "convert-command-to-skill-or-workflow",
	})
	if err == nil {
		t.Fatalf("BuildCodexPreview returned nil error for duplicate preview paths")
	}
	if !strings.Contains(err.Error(), ".agents/skills/foo/SKILL.md") {
		t.Fatalf("duplicate path error missing preview path: %v", err)
	}
}

func TestBuildCodexPreviewScrubsAbsolutePathsFromWarnings(t *testing.T) {
	absolutePath := filepath.Join(t.TempDir(), "codex", "AGENTS.md")
	preview := buildSyntheticPreviewWithWarnings(t, []model.Warning{{Code: "existing-codex-target", Message: "existing target at " + absolutePath + " requires review"}})

	for _, path := range []string{".codex/config.toml", "migration-report.md"} {
		contents := string(requireFile(t, preview, path).Contents)
		if strings.Contains(contents, absolutePath) {
			t.Fatalf("%s contains absolute warning path %q:\n%s", path, absolutePath, contents)
		}
		if !strings.Contains(contents, "AGENTS.md") && !strings.Contains(contents, "<path>") {
			t.Fatalf("%s missing scrubbed warning path marker or basename:\n%s", path, contents)
		}
	}
}

func TestBuildCodexPreviewRedactsSecretLikeSourceLines(t *testing.T) {
	tests := []struct {
		name        string
		sourceLine  string
		wantLine    string
		previewPath string
	}{
		{
			name:        "colon assignment",
			sourceLine:  "GITHUB_TOKEN: " + fixtureSecret,
			wantLine:    "GITHUB_TOKEN: <REDACTED>",
			previewPath: ".agents/skills/token-command-colon/SKILL.md",
		},
		{
			name:        "equals assignment",
			sourceLine:  "GITHUB_TOKEN=" + fixtureSecret,
			wantLine:    "GITHUB_TOKEN=<REDACTED>",
			previewPath: ".agents/skills/token-command-equals/SKILL.md",
		},
		{
			name:        "export equals assignment",
			sourceLine:  "export GITHUB_TOKEN=" + fixtureSecret,
			wantLine:    "export GITHUB_TOKEN=<REDACTED>",
			previewPath: ".agents/skills/token-command-export/SKILL.md",
		},
		{
			name:        "export multiple assignments",
			sourceLine:  "export FOO=bar GITHUB_TOKEN=" + fixtureSecret,
			wantLine:    "export FOO=bar GITHUB_TOKEN=<REDACTED>",
			previewPath: ".agents/skills/token-command-export-multiple/SKILL.md",
		},
		{
			name:        "command environment assignment",
			sourceLine:  "FOO=bar GITHUB_TOKEN=" + fixtureSecret + " command",
			wantLine:    "FOO=bar GITHUB_TOKEN=<REDACTED> command",
			previewPath: ".agents/skills/token-command-env/SKILL.md",
		},
		{
			name:        "inline github token",
			sourceLine:  "Use token " + fixtureSecret + " for tests.",
			wantLine:    "Use token <REDACTED> for tests.",
			previewPath: ".agents/skills/token-command-inline/SKILL.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourcePath := writeTempFile(t, strings.TrimSuffix(strings.TrimPrefix(tt.previewPath, ".agents/skills/"), "/SKILL.md")+".md", "# Token Command\n\n"+tt.sourceLine+"\nKeep this safe.\n")
			preview := buildSyntheticPreview(t, model.Resource{
				ID:         "command:project-" + strings.TrimSuffix(strings.TrimPrefix(tt.previewPath, ".agents/skills/"), "/SKILL.md"),
				Kind:       model.KindCommand,
				Scope:      model.ScopeProject,
				SourcePath: sourcePath,
				Status:     model.StatusPartial,
				Strategy:   "convert-command-to-skill-or-workflow",
			})

			for _, file := range preview.Files {
				contents := string(file.Contents)
				if strings.Contains(contents, fixtureSecret) {
					t.Fatalf("%s leaked fixture secret", file.Path)
				}
			}
			command := string(requireFile(t, preview, tt.previewPath).Contents)
			if !strings.Contains(command, tt.wantLine) {
				t.Fatalf("command preview missing redacted source line %q:\n%s", tt.wantLine, command)
			}
		})
	}
}

func TestBuildCodexPreviewRedactsPrivateKeyBlocks(t *testing.T) {
	sourcePath := writeTempFile(t, "private-key-command.md", "# Private Key Command\n\n-----BEGIN PRIVATE KEY-----\nfixture-secret-key-material\n-----END PRIVATE KEY-----\n")
	preview := buildSyntheticPreview(t, model.Resource{
		ID:         "command:project-private-key-command",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: sourcePath,
		Status:     model.StatusPartial,
		Strategy:   "convert-command-to-skill-or-workflow",
	})

	command := string(requireFile(t, preview, ".agents/skills/private-key-command/SKILL.md").Contents)
	if strings.Contains(command, "fixture-secret-key-material") || strings.Contains(command, "BEGIN PRIVATE KEY") {
		t.Fatalf("command preview leaked private key block:\n%s", command)
	}
	if !strings.Contains(command, "<REDACTED>") {
		t.Fatalf("command preview missing redaction marker:\n%s", command)
	}
}

func TestBuildCodexPreviewReportGeneratedFilesAreSortedLikePreviewPaths(t *testing.T) {
	preview := buildSyntheticPreview(t, model.Resource{
		ID:         "agent:project-zed",
		Kind:       model.KindAgent,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, "zed.md", "# Zed\n"),
		Status:     model.StatusPartial,
		Strategy:   "rewrite-agent-schema",
	}, model.Resource{
		ID:         "command:project-alpha",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, "alpha.md", "# Alpha\n"),
		Status:     model.StatusPartial,
		Strategy:   "convert-command-to-skill-or-workflow",
	})

	var previewPaths []string
	for _, file := range preview.Files {
		previewPaths = append(previewPaths, file.Path)
	}
	reportPaths := generatedFilePathsFromReport(t, string(requireFile(t, preview, "migration-report.md").Contents))
	if strings.Join(reportPaths, "\n") != strings.Join(previewPaths, "\n") {
		t.Fatalf("report generated files = %#v, want sorted preview paths %#v", reportPaths, previewPaths)
	}
}

func buildPreview(t *testing.T, name string) exporter.CodexPreview {
	t.Helper()
	scan := scanFixture(t, name)
	plan := planner.Build(scan)
	preview, err := exporter.BuildCodexPreview(scan, plan)
	if err != nil {
		t.Fatalf("BuildCodexPreview returned error: %v", err)
	}
	return preview
}

func scanFixture(t *testing.T, name string) model.ScanReport {
	t.Helper()
	fixture := filepath.Join(repoRoot(), "testdata", name)
	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	return report
}

func repoRoot() string {
	return filepath.Clean(filepath.Join("..", ".."))
}

func buildSyntheticPreview(t *testing.T, resources ...model.Resource) exporter.CodexPreview {
	t.Helper()
	preview, err := buildSyntheticPreviewResult(t, resources...)
	if err != nil {
		t.Fatalf("BuildCodexPreview returned error: %v", err)
	}
	return preview
}

func buildSyntheticPreviewWithWarnings(t *testing.T, warnings []model.Warning, resources ...model.Resource) exporter.CodexPreview {
	t.Helper()
	preview, err := buildSyntheticPreviewResultWithWarnings(t, warnings, resources...)
	if err != nil {
		t.Fatalf("BuildCodexPreview returned error: %v", err)
	}
	return preview
}

func buildSyntheticPreviewResult(t *testing.T, resources ...model.Resource) (exporter.CodexPreview, error) {
	t.Helper()
	return buildSyntheticPreviewResultWithWarnings(t, nil, resources...)
}

func buildSyntheticPreviewResultWithWarnings(t *testing.T, warnings []model.Warning, resources ...model.Resource) (exporter.CodexPreview, error) {
	t.Helper()
	scan := model.ScanReport{
		SchemaVersion: model.ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       t.TempDir(),
		ClaudeHome:    t.TempDir(),
		CodexHome:     t.TempDir(),
		Resources:     resources,
		Warnings:      warnings,
	}
	plan := planner.Build(scan)
	return exporter.BuildCodexPreview(scan, plan)
}

func writeTempFile(t *testing.T, relativePath string, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), relativePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func requireFile(t *testing.T, preview exporter.CodexPreview, path string) exporter.PreviewFile {
	t.Helper()
	for _, file := range preview.Files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("preview file %q not found in %#v", path, preview.Files)
	return exporter.PreviewFile{}
}

func assertNoFile(t *testing.T, preview exporter.CodexPreview, path string) {
	t.Helper()
	for _, file := range preview.Files {
		if file.Path == path {
			t.Fatalf("preview file %q was generated unexpectedly", path)
		}
	}
}

func generatedFilePathsFromReport(t *testing.T, report string) []string {
	t.Helper()
	var paths []string
	inSection := false
	for _, line := range strings.Split(report, "\n") {
		if line == "## generated files" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if !inSection || !strings.HasPrefix(line, "- `") || !strings.HasSuffix(line, "`") {
			continue
		}
		paths = append(paths, strings.TrimSuffix(strings.TrimPrefix(line, "- `"), "`"))
	}
	if len(paths) == 0 {
		t.Fatalf("no generated file paths found in report:\n%s", report)
	}
	return paths
}
