package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicReadinessFilesExist(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, rel := range []string{
		"README.md",
		"README.zh-CN.md",
		"README.en.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join("scripts", "github-readiness.sh"),
	} {
		assertPublicFileExists(t, filepath.Join(repoRoot, rel))
	}
}

func TestPublicReadinessReadmesFollowLanguageAndQuickStartRules(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, rel := range []string{"README.md", "README.zh-CN.md", "README.en.md"} {
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		assertSingleLanguageSwitch(t, rel, contents)
	}

	for _, rel := range []string{"README.zh-CN.md", "README.en.md"} {
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		if !strings.Contains(contents, "Quick Start") {
			t.Fatalf("%s missing Quick Start section", rel)
		}
		if !strings.Contains(contents, "agent-canon --help") {
			t.Fatalf("%s missing help command example", rel)
		}
		for _, command := range []string{"agent-canon scan", "agent-canon sync claude codex", "agent-canon apply codex --dry-run"} {
			if !strings.Contains(contents, command) {
				t.Fatalf("%s missing golden path command %q", rel, command)
			}
		}
	}
}

func TestPublicReadinessSecurityAndContributingContracts(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	security := readFileString(t, filepath.Join(repoRoot, "SECURITY.md"))
	for _, want := range []string{"GitHub Security", "private vulnerability reporting", "Do not", "secret", "exploit"} {
		if !strings.Contains(security, want) {
			t.Fatalf("SECURITY.md missing %q", want)
		}
	}

	contributing := strings.ToLower(readFileString(t, filepath.Join(repoRoot, "CONTRIBUTING.md")))
	for _, want := range []string{"build", "test", "local ci", "secret scan", "pull request", "commit message", "release", "tag"} {
		if !strings.Contains(contributing, want) {
			t.Fatalf("CONTRIBUTING.md missing %q", want)
		}
	}
}

func TestPublicReadinessFilesDoNotExposePrivateContent(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, rel := range []string{
		"README.md",
		"README.zh-CN.md",
		"README.en.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join("scripts", "github-readiness.sh"),
	} {
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		for _, forbidden := range []string{"/Users/", "ghp_", fixtureSecret, "GITHUB_TOKEN=", "ANTHROPIC_API_KEY="} {
			if strings.Contains(contents, forbidden) {
				t.Fatalf("%s contains forbidden public content %q", rel, forbidden)
			}
		}
	}
}

func TestPublicReadinessScriptChecksPrivateVulnerabilityReportingCorrectly(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	contents := readFileString(t, filepath.Join(repoRoot, "scripts", "github-readiness.sh"))
	if !strings.Contains(contents, "/private-vulnerability-reporting") {
		t.Fatalf("github-readiness.sh must use the private vulnerability reporting endpoint")
	}
	if !strings.Contains(contents, ".enabled") {
		t.Fatalf("github-readiness.sh must check the private vulnerability reporting enabled field")
	}
	for _, want := range []string{"security_and_analysis", "secret_scanning", "secret_scanning_push_protection", "code-scanning", "rulesets"} {
		if !strings.Contains(contents, want) {
			t.Fatalf("github-readiness.sh missing readiness check marker %q", want)
		}
	}
}

func publicReadinessRepoRoot() string {
	return filepath.Clean(filepath.Join("..", ".."))
}

func assertPublicFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("%s is a directory, want file", path)
	}
}

func assertSingleLanguageSwitch(t *testing.T, label string, contents string) {
	t.Helper()
	const languageSwitch = "[中文](./README.zh-CN.md) | [English](./README.en.md)"
	if !strings.HasPrefix(contents, languageSwitch+"\n\n") {
		t.Fatalf("%s must start with the language switch", label)
	}
	if strings.Count(contents, languageSwitch) != 1 {
		t.Fatalf("%s language switch count = %d, want 1", label, strings.Count(contents, languageSwitch))
	}
}
