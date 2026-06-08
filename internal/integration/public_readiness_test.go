package integration_test

import (
	"fmt"
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
	for _, rel := range []string{"README.md", "README.zh-CN.md"} {
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		assertSingleLanguageSwitch(t, rel, contents)
		if strings.Contains(contents, "./docs/") {
			t.Fatalf("%s links to docs excluded from public release package", rel)
		}
	}

	for _, rel := range []string{"README.md", "README.zh-CN.md"} {
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		if !strings.Contains(contents, "Quick Start") {
			t.Fatalf("%s missing Quick Start section", rel)
		}
		if !strings.Contains(contents, "./agent-canon --help") {
			t.Fatalf("%s missing local help command example", rel)
		}
		for _, command := range []string{"./agent-canon scan", "./agent-canon sync claude codex", "./agent-canon apply codex --dry-run"} {
			if !strings.Contains(contents, command) {
				t.Fatalf("%s missing local golden path command %q", rel, command)
			}
		}
	}
}

func TestPublicReadinessReadmesDocumentScenarioExamples(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, tc := range []struct {
		rel     string
		markers []string
	}{
		{
			rel: "README.md",
			markers: []string{
				"## Scenario Examples",
				"Preview a migration without writing targets",
				"Review and resolve conflicts before applying",
				"Inspect global-home changes safely",
				"./agent-canon compile codex --out <preview-dir>",
				"./agent-canon apply codex --global --dry-run --only config",
				"Only replace `--dry-run` with `--yes` after reviewing the output",
				"Do not use `--global --yes` unless you intentionally want to write selected global home targets",
			},
		},
		{
			rel: "README.zh-CN.md",
			markers: []string{
				"## 场景示例",
				"只预览迁移结果",
				"解决冲突",
				"安全检查 global home",
				"./agent-canon compile codex --out <preview-dir>",
				"./agent-canon apply codex --global --dry-run --only config",
				"只有审查输出后，才把 `--dry-run` 换成 `--yes`",
				"否则不要使用 `--global --yes`",
			},
		},
	} {
		contents := readFileString(t, filepath.Join(repoRoot, tc.rel))
		for _, marker := range tc.markers {
			if !strings.Contains(contents, marker) {
				t.Fatalf("%s scenario examples missing marker %q", tc.rel, marker)
			}
		}
	}
}

func TestPublicReadinessReadmesDocumentReleaseInstallPath(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, tc := range []struct {
		rel            string
		installHeading string
		currentScope   string
	}{
		{rel: "README.zh-CN.md", installHeading: "## 安装与 Release 归档", currentScope: "当前范围"},
		{rel: "README.md", installHeading: "## Install and Release Archives", currentScope: "Current Scope"},
	} {
		contents := readFileString(t, filepath.Join(repoRoot, tc.rel))
		quickStartIndex := strings.Index(contents, "## Quick Start")
		installHeadingIndex := strings.Index(contents, tc.installHeading)
		currentScopeIndex := strings.Index(contents, tc.currentScope)
		if quickStartIndex < 0 || installHeadingIndex < 0 || currentScopeIndex < 0 || !(quickStartIndex < installHeadingIndex && installHeadingIndex < currentScopeIndex) {
			t.Fatalf("%s must document release archive install guidance after Quick Start and before Current Scope", tc.rel)
		}
		installSection := contents[installHeadingIndex:currentScopeIndex]
		for _, want := range []string{"agent-canon_vX.Y.Z_<goos>_<goarch>.tar.gz", "checksums.txt", "README.zh-CN.md", "LICENSE", "agent-canon --help"} {
			if !strings.Contains(installSection, want) {
				t.Fatalf("%s install section missing release install guidance marker %q", tc.rel, want)
			}
		}
	}
}

func TestPublicReadinessReadmesDistinguishReleaseAndLocalBuildCommands(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	for _, tc := range []struct {
		rel              string
		installHeading   string
		currentScope     string
		localBuildMarker string
	}{
		{rel: "README.zh-CN.md", installHeading: "## 安装与 Release 归档", currentScope: "当前范围", localBuildMarker: "源码构建"},
		{rel: "README.md", installHeading: "## Install and Release Archives", currentScope: "Current Scope", localBuildMarker: "source build"},
	} {
		contents := readFileString(t, filepath.Join(repoRoot, tc.rel))
		installHeadingIndex := strings.Index(contents, tc.installHeading)
		currentScopeIndex := strings.Index(contents, tc.currentScope)
		if installHeadingIndex < 0 || currentScopeIndex < 0 || installHeadingIndex >= currentScopeIndex {
			t.Fatalf("%s install section is not bounded by expected headings", tc.rel)
		}
		installSection := contents[installHeadingIndex:currentScopeIndex]
		for _, want := range []string{"agent-canon --help", "./build.sh", "./agent-canon --help", tc.localBuildMarker} {
			if !strings.Contains(installSection, want) {
				t.Fatalf("%s install section missing local/release command marker %q", tc.rel, want)
			}
		}
		if strings.Contains(installSection, "././agent-canon") {
			t.Fatalf("%s install section contains duplicated local binary prefix", tc.rel)
		}
	}
}

func TestDocsAgentCanonShellExamplesFollowCLIUsage(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	docsDir := filepath.Join(repoRoot, "docs")
	entries, err := os.ReadDir(docsDir)
	if os.IsNotExist(err) {
		t.Skip("docs directory is not part of the public repository checkout")
	}
	if err != nil {
		t.Fatalf("read docs directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		rel := filepath.Join("docs", entry.Name())
		contents := readFileString(t, filepath.Join(repoRoot, rel))
		assertDocumentedAgentCanonCommandsFollowCLIUsage(t, rel, contents)
	}
}

func assertDocumentedAgentCanonCommandsFollowCLIUsage(t *testing.T, rel string, contents string) {
	t.Helper()
	inShellFence := false
	for index, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence, ok := strings.CutPrefix(trimmed, "```"); ok {
			fence = strings.TrimSpace(fence)
			inShellFence = fence == "sh" || fence == "bash"
			continue
		}
		if !inShellFence || !strings.HasPrefix(trimmed, "agent-canon ") {
			continue
		}
		if err := validateDocumentedAgentCanonCommand(trimmed); err != nil {
			t.Fatalf("%s:%d documents invalid agent-canon command %q: %v", rel, index+1, trimmed, err)
		}
	}
}

func validateDocumentedAgentCanonCommand(command string) error {
	fields := strings.Fields(command)
	if len(fields) < 2 || fields[0] != "agent-canon" {
		return nil
	}
	known := map[string]bool{
		"init": true, "scan": true, "status": true, "diff": true, "plan": true,
		"export": true, "import": true, "compile": true, "sync": true,
		"conflicts": true, "resolve": true, "apply": true, "rollback": true, "verify": true,
	}
	cmd := fields[1]
	if !known[cmd] {
		return fmt.Errorf("unknown command %q", cmd)
	}
	switch cmd {
	case "export", "compile":
		if len(fields) < 3 || !isToolTarget(fields[2]) {
			return fmt.Errorf("%s requires target claude or codex", cmd)
		}
		if !hasFlagWithValue(fields, "--out") {
			return fmt.Errorf("%s %s requires --out <dir>", cmd, fields[2])
		}
	case "import", "verify":
		if len(fields) < 3 || !isToolTarget(fields[2]) {
			return fmt.Errorf("%s requires target claude or codex", cmd)
		}
	case "sync":
		if len(fields) < 4 || fields[2] != "claude" || fields[3] != "codex" {
			return fmt.Errorf("sync requires direction claude codex")
		}
	case "resolve":
		if len(fields) < 4 || strings.HasPrefix(fields[2], "-") {
			return fmt.Errorf("resolve requires <conflict-id> and a decision")
		}
		if !hasResolveDecision(fields[3:]) {
			return fmt.Errorf("resolve requires --ours, --theirs, --accept-suggestion, or --manual <value>")
		}
	case "apply":
		if len(fields) < 3 || !isToolTarget(fields[2]) {
			return fmt.Errorf("apply requires target claude or codex")
		}
	case "rollback":
		if len(fields) < 3 || strings.HasPrefix(fields[2], "-") {
			return fmt.Errorf("rollback requires <apply-id>")
		}
	case "diff":
		if len(fields) >= 3 && fields[2] != "codex" && !strings.HasPrefix(fields[2], "-") {
			return fmt.Errorf("diff supports only optional codex target")
		}
	}
	return nil
}

func isToolTarget(value string) bool {
	return value == "claude" || value == "codex"
}

func hasFlagWithValue(fields []string, flag string) bool {
	for index, field := range fields {
		if field == flag && index+1 < len(fields) && !strings.HasPrefix(fields[index+1], "-") {
			return true
		}
	}
	return false
}

func hasResolveDecision(fields []string) bool {
	for index, field := range fields {
		switch field {
		case "--ours", "--theirs", "--accept-suggestion":
			return true
		case "--manual":
			return index+1 < len(fields) && !strings.HasPrefix(fields[index+1], "-")
		}
	}
	return false
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
		"SECURITY.md",
		"CONTRIBUTING.md",
		"build.sh",
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "codeql.yml"),
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
	for _, want := range []string{"actions/checkout@v6", "actions/setup-go@v6", "go-version-file: go.mod", "cache: false", "scripts/secret-scan.sh", "scripts/package-release.sh v0.0.0-ci"} {
		if !strings.Contains(ci, want) {
			t.Fatalf("ci.yml missing %q", want)
		}
	}
	codeql := readFileString(t, filepath.Join(repoRoot, ".github", "workflows", "codeql.yml"))
	for _, want := range []string{
		"name: CodeQL",
		"security-events: write",
		"github.event.repository.private == false",
		"actions/checkout@v6",
		"github/codeql-action/init@v4",
		"languages: go",
		"build-mode: manual",
		"actions/setup-go@v6",
		"go-version-file: go.mod",
		"github/codeql-action/analyze@v4",
	} {
		if !strings.Contains(codeql, want) {
			t.Fatalf("codeql.yml missing %q", want)
		}
	}
	release := readFileString(t, filepath.Join(repoRoot, ".github", "workflows", "release.yml"))
	for _, want := range []string{
		"tags:",
		"v*",
		"security-events: read",
		"actions/checkout@v6",
		"actions/setup-go@v6",
		"actions/upload-artifact@v7",
		"actions/download-artifact@v8",
		"fetch-depth: 0",
		"scripts/secret-scan.sh --history",
		"GH_TOKEN: ${{ github.token }}",
		"scripts/github-readiness.sh --repo \"${GITHUB_REPOSITORY}\" --report-only",
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
	for _, want := range []string{
		"release-notes.md",
		"agent-canon_vX.Y.Z_<goos>_<goarch>.tar.gz",
		"Verify the downloaded archive with `checksums.txt`",
		"agent-canon --help",
		"README.zh-CN.md",
		"gh release edit",
		"--notes-file dist/release-notes.md",
		"--repo \"${GITHUB_REPOSITORY}\"",
	} {
		if !strings.Contains(release, want) {
			t.Fatalf("release.yml missing release note marker %q", want)
		}
	}
	if strings.Contains(release, "--notes \"Release ${GITHUB_REF_NAME}\"") {
		t.Fatalf("release.yml still uses placeholder release notes")
	}
}

func publicReadinessFiles() []string {
	return []string{
		"README.md",
		"README.zh-CN.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		"build.sh",
		filepath.Join(".github", "ISSUE_TEMPLATE", "bug_report.yml"),
		filepath.Join(".github", "ISSUE_TEMPLATE", "feature_request.yml"),
		filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join(".github", "workflows", "codeql.yml"),
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
	want := "[中文](./README.zh-CN.md)"
	if label == "README.zh-CN.md" {
		want = "[English](./README.md)"
	}
	if !strings.HasPrefix(contents, want+"\n\n") {
		t.Fatalf("%s must start with the language switch %q", label, want)
	}
	if strings.Count(contents, want) != 1 {
		t.Fatalf("%s language switch count = %d, want 1", label, strings.Count(contents, want))
	}
	for _, forbidden := range []string{"README.en.md", "[中文](./README.zh-CN.md) | [English]"} {
		if strings.Contains(contents, forbidden) {
			t.Fatalf("%s contains obsolete language switch marker %q", label, forbidden)
		}
	}
}
