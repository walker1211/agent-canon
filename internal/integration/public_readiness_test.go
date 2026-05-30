package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicReadinessFilesExist(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, rel := range publicReadinessFiles() {
		assertPublicFileExists(t, filepath.Join(repoRoot, rel))
	}
}

func TestPublicReadinessFilesAreTrackable(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, rel := range publicReadinessFiles() {
		cmd := exec.Command("git", "-C", repoRoot, "check-ignore", "-q", "--", rel)
		err := cmd.Run()
		if err == nil {
			t.Fatalf("%s is ignored by git", rel)
		}
		if exitError, ok := err.(*exec.ExitError); !ok || exitError.ExitCode() != 1 {
			t.Fatalf("check git ignore status for %s: %v", rel, err)
		}
	}
}

func TestPublicReadinessReadmesFollowLanguageAndQuickStartRules(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, rel := range []string{"README.md", "README.zh-CN.md", "README.en.md"} {
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		assertSingleLanguageSwitch(t, rel, contents)
		if strings.Contains(contents, "./docs/") {
			t.Fatalf("%s links to docs excluded from public release package", rel)
		}
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

func TestPublicReadinessContributingDocumentsReleasePackaging(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	contributing := readFileString(t, filepath.Join(repoRoot, "CONTRIBUTING.md"))
	for _, want := range []string{
		"scripts/package-release.sh vX.Y.Z",
		"scripts/ci-local.sh clean",
		"scripts/tag-release.sh vX.Y.Z",
		"`agent-canon`",
		"`LICENSE`",
		"`README.md`",
		"`README.zh-CN.md`",
		"`README.en.md`",
		"local config",
		".env",
		"databases",
		"logs",
		"generated workspace state",
		"private assets",
	} {
		if !strings.Contains(contributing, want) {
			t.Fatalf("CONTRIBUTING.md missing release packaging guidance %q", want)
		}
	}
}

func TestPublicReadinessContributingDocumentsReadinessExecution(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	contributing := readFileString(t, filepath.Join(repoRoot, "CONTRIBUTING.md"))
	for _, want := range []string{
		"scripts/github-readiness.sh --repo OWNER/REPO",
		"scripts/github-readiness.sh --repo OWNER/REPO --strict",
		"read-only",
		"does not change GitHub settings",
		"release preflight",
	} {
		if !strings.Contains(contributing, want) {
			t.Fatalf("CONTRIBUTING.md missing readiness execution guidance %q", want)
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
		"build.sh",
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "release.yml"),
		filepath.Join("scripts", "github-readiness.sh"),
		filepath.Join("scripts", "package-release.sh"),
		filepath.Join("scripts", "secret-scan.sh"),
		filepath.Join("scripts", "ci-local.sh"),
		filepath.Join("scripts", "tag-release.sh"),
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

func TestPublicReadinessScriptSupportsStrictReadinessAudit(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	contents := readFileString(t, filepath.Join(repoRoot, "scripts", "github-readiness.sh"))
	for _, want := range []string{
		"command -v gh",
		"command -v jq",
		"--strict",
		"--report-only",
		"default_branch",
		"repos/$repo/branches/$branch/protection",
		"required_status_checks",
		"Settings > Code security and analysis",
		"exit 1",
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("github-readiness.sh missing strict audit support %q", want)
		}
	}
}

func TestPublicReadinessScriptTreatsFailedGitHubAPIResponsesAsUnavailable(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	fakeBin := t.TempDir()
	fakeGH := filepath.Join(fakeBin, "gh")
	fakeGHScript := `#!/bin/sh
if [ "$1" = "api" ]; then
  case "$2" in
    repos/OWNER/REPO)
      printf '{"default_branch":"main","security_and_analysis":null}\n'
      exit 0
      ;;
    repos/OWNER/REPO/private-vulnerability-reporting|repos/OWNER/REPO/branches/main/protection|repos/OWNER/REPO/rulesets|repos/OWNER/REPO/code-scanning/alerts?state=open\&per_page=1)
      printf '{"message":"unavailable","status":"403"}\n'
      exit 1
      ;;
  esac
fi
exit 1
`
	if err := os.WriteFile(fakeGH, []byte(fakeGHScript), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}

	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "github-readiness.sh"), "--repo", "OWNER/REPO", "--report-only")
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("github-readiness.sh returned error: %v\n%s", err, output)
	}
	contents := string(output)
	for _, want := range []string{
		"private vulnerability reporting: unavailable",
		"default branch protection or rulesets: missing or unavailable",
		"CodeQL / code-scanning: unavailable",
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("github-readiness.sh output missing %q:\n%s", want, contents)
		}
	}
	for _, notWant := range []string{
		"private vulnerability reporting: false",
		"default branch protection or rulesets: rulesets present",
		"CodeQL / code-scanning: available",
	} {
		if strings.Contains(contents, notWant) {
			t.Fatalf("github-readiness.sh output contains %q:\n%s", notWant, contents)
		}
	}
}

func TestPublicReadinessCIAndReleaseWorkflowContracts(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	ci := readFileString(t, filepath.Join(repoRoot, ".github", "workflows", "ci.yml"))
	for _, want := range []string{"go-version-file: go.mod", "cache: false", "scripts/secret-scan.sh", "scripts/package-release.sh v0.0.0-ci"} {
		if !strings.Contains(ci, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
	release := readFileString(t, filepath.Join(repoRoot, ".github", "workflows", "release.yml"))
	for _, want := range []string{
		"tags:",
		"v*",
		"security-events: read",
		"fetch-depth: 0",
		"scripts/secret-scan.sh --history",
		"GH_TOKEN: ${{ github.token }}",
		"scripts/github-readiness.sh --repo \"${GITHUB_REPOSITORY}\" --strict",
		"linux",
		"darwin",
		"windows",
		"amd64",
		"arm64",
		"checksums.txt",
		"gh release",
	} {
		if !strings.Contains(release, want) {
			t.Fatalf("release.yml missing %q", want)
		}
	}
}

func publicReadinessFiles() []string {
	return []string{
		"README.md",
		"README.zh-CN.md",
		"README.en.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		"build.sh",
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "release.yml"),
		filepath.Join("scripts", "github-readiness.sh"),
		filepath.Join("scripts", "package-release.sh"),
		filepath.Join("scripts", "secret-scan.sh"),
		filepath.Join("scripts", "ci-local.sh"),
		filepath.Join("scripts", "tag-release.sh"),
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
