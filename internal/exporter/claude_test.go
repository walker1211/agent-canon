package exporter_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/exporter"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
)

func TestBuildClaudePreviewBasicFixtureGeneratesExpectedFiles(t *testing.T) {
	preview := buildClaudePreview(t, "basic")

	requireFile(t, preview, "CLAUDE.md")
	requireFile(t, preview, ".claude/settings.json")
	requireFile(t, preview, filepath.ToSlash(filepath.Join(".claude", "skills", "sample-skill", "SKILL.md")))
	requireFile(t, preview, "migration-report.md")
}

func TestBuildClaudePreviewClaudeMDContainsInstructionsAndRulesWithoutAbsolutePaths(t *testing.T) {
	preview := buildClaudePreview(t, "basic")
	claude := string(requireFile(t, preview, "CLAUDE.md").Contents)

	for _, want := range []string{
		"# Global Claude Instructions",
		"Prefer concise answers and explain tradeoffs when changing configuration.",
		"# Project Instructions",
		"This fixture project is read-only input for scanner tests.",
		"# Language",
		"Use English for public project files unless a user asks otherwise.",
	} {
		if !strings.Contains(claude, want) {
			t.Fatalf("CLAUDE.md missing %q in:\n%s", want, claude)
		}
	}

	for _, resource := range scanFixture(t, "basic").Resources {
		if resource.SourcePath != "" && strings.Contains(claude, resource.SourcePath) {
			t.Fatalf("CLAUDE.md contains absolute source path %q:\n%s", resource.SourcePath, claude)
		}
	}
}

func TestBuildClaudePreviewUnsupportedFixtureReportsSkippedReviewResourcesWithoutGeneratingThem(t *testing.T) {
	preview := buildClaudePreview(t, "unsupported")
	report := string(requireFile(t, preview, "migration-report.md").Contents)

	for _, want := range []string{
		"hook:global-PreToolUse",
		"session:global-session-history",
		"skipped unsupported resources",
		"review-required resources",
		"no real Claude configuration was written",
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

func TestBuildClaudePreviewSecretsFixtureRedactsSecrets(t *testing.T) {
	preview := buildClaudePreview(t, "secrets")

	for _, file := range preview.Files {
		if strings.Contains(string(file.Contents), fixtureSecret) {
			t.Fatalf("%s leaked fixture secret", file.Path)
		}
	}

	settings := string(requireFile(t, preview, ".claude/settings.json").Contents)
	report := string(requireFile(t, preview, "migration-report.md").Contents)
	if !strings.Contains(settings, "<REDACTED>") && !strings.Contains(settings, "secret-redacted") {
		t.Fatalf("settings missing redaction marker or warning:\n%s", settings)
	}
	if !strings.Contains(report, "<REDACTED>") && !strings.Contains(report, "secret-redacted") {
		t.Fatalf("report missing redaction marker or warning:\n%s", report)
	}
}

func TestBuildClaudePreviewGeneratesPartialCommandAndAgentPreviews(t *testing.T) {
	preview := buildSyntheticClaudePreview(t, model.Resource{
		ID:         "command:project-deploy",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, "deploy.md", "# Deploy\n\nRun deployment steps.\n"),
		Status:     model.StatusPartial,
		Strategy:   "convert-command-with-review",
	}, model.Resource{
		ID:         "agent:project-reviewer",
		Kind:       model.KindAgent,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, "reviewer.md", "# Reviewer\n\nReview code.\n"),
		Status:     model.StatusPartial,
		Strategy:   "convert-agent-with-review",
	})

	command := string(requireFile(t, preview, filepath.ToSlash(filepath.Join(".claude", "commands", "deploy.md"))).Contents)
	if !strings.Contains(command, "Review required") || !strings.Contains(command, "Run deployment steps.") {
		t.Fatalf("command preview missing review text or source content:\n%s", command)
	}
	agent := string(requireFile(t, preview, filepath.ToSlash(filepath.Join(".claude", "agents", "reviewer.md"))).Contents)
	if !strings.Contains(agent, "Review required") || !strings.Contains(agent, "Review code.") {
		t.Fatalf("agent preview missing review text or source content:\n%s", agent)
	}
}

func TestBuildClaudePreviewReturnsErrorForDuplicatePreviewPaths(t *testing.T) {
	_, err := buildSyntheticClaudePreviewResult(t, model.Resource{
		ID:         "skill:project-first-foo",
		Kind:       model.KindSkill,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, filepath.Join("first", "foo", "SKILL.md"), "# First Foo\n"),
		Status:     model.StatusPartial,
		Strategy:   "copy-skill-with-review",
	}, model.Resource{
		ID:         "skill:project-second-foo",
		Kind:       model.KindSkill,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, filepath.Join("second", "foo", "SKILL.md"), "# Second Foo\n"),
		Status:     model.StatusPartial,
		Strategy:   "copy-skill-with-review",
	})
	if err == nil {
		t.Fatalf("BuildClaudePreview returned nil error for duplicate preview paths")
	}
	if !strings.Contains(err.Error(), filepath.ToSlash(filepath.Join(".claude", "skills", "foo", "SKILL.md"))) {
		t.Fatalf("duplicate path error missing preview path: %v", err)
	}
}

func TestBuildClaudePreviewReportGeneratedFilesAreSortedLikePreviewPaths(t *testing.T) {
	preview := buildSyntheticClaudePreview(t, model.Resource{
		ID:         "agent:project-zed",
		Kind:       model.KindAgent,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, "zed.md", "# Zed\n"),
		Status:     model.StatusPartial,
		Strategy:   "convert-agent-with-review",
	}, model.Resource{
		ID:         "command:project-alpha",
		Kind:       model.KindCommand,
		Scope:      model.ScopeProject,
		SourcePath: writeTempFile(t, "alpha.md", "# Alpha\n"),
		Status:     model.StatusPartial,
		Strategy:   "convert-command-with-review",
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

func buildClaudePreview(t *testing.T, name string) exporter.CodexPreview {
	t.Helper()
	scan := scanFixture(t, name)
	plan := planner.Build(scan)
	preview, err := exporter.BuildClaudePreview(scan, plan)
	if err != nil {
		t.Fatalf("BuildClaudePreview returned error: %v", err)
	}
	return preview
}

func buildSyntheticClaudePreview(t *testing.T, resources ...model.Resource) exporter.CodexPreview {
	t.Helper()
	preview, err := buildSyntheticClaudePreviewResult(t, resources...)
	if err != nil {
		t.Fatalf("BuildClaudePreview returned error: %v", err)
	}
	return preview
}

func buildSyntheticClaudePreviewResult(t *testing.T, resources ...model.Resource) (exporter.CodexPreview, error) {
	t.Helper()
	scan := model.ScanReport{
		SchemaVersion: model.ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       t.TempDir(),
		ClaudeHome:    t.TempDir(),
		CodexHome:     t.TempDir(),
		Resources:     resources,
	}
	plan := planner.Build(scan)
	return exporter.BuildClaudePreview(scan, plan)
}
