package verifier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

const verifierFixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestVerifyRejectsUnsupportedTargetBeforeInspectingPaths(t *testing.T) {
	root := t.TempDir()
	_, err := Verify(Options{
		Target:     "unknown",
		Project:    filepath.Join(root, "missing-project"),
		ClaudeHome: filepath.Join(root, "missing-claude-home"),
		CodexHome:  filepath.Join(root, "missing-codex-home"),
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported verify target "unknown"`) {
		t.Fatalf("Verify error = %v, want unsupported target", err)
	}
}

func TestVerifyCodexPassesGeneratedProjectTargets(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, "AGENTS.md"), "# AGENTS.md preview\n\nGenerated preview for Codex.\n")
	writeFile(t, filepath.Join(paths.project, ".codex", "config.toml"), "[mcp_servers.github]\ncommand = \"gh\"\n")
	writeFile(t, filepath.Join(paths.project, ".agents", "skills", "review", "SKILL.md"), "# Review\n")
	writeFile(t, filepath.Join(paths.project, ".codex", "agents", "reviewer.toml"), "name = \"reviewer\"\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-instructions-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-config-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-mcp-list", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-skills-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-agents-project", model.VerifyStatusPass)
	if report.Summary.Fail != 0 {
		t.Fatalf("Summary.Fail = %d, want 0; checks=%#v", report.Summary.Fail, report.Checks)
	}
}

func TestVerifyCodexPassesGlobalFallbackTargets(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.codexHome, "AGENTS.md"), "# Global AGENTS\n")
	writeFile(t, filepath.Join(paths.codexHome, "config.toml"), "[mcp_servers.github]\ncommand = \"gh\"\n")
	writeFile(t, filepath.Join(paths.codexHome, "skills", "review", "SKILL.md"), "# Review\n")
	writeFile(t, filepath.Join(paths.codexHome, "agents", "reviewer.toml"), "name = \"reviewer\"\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-instructions-project", model.VerifyStatusPass)
	assertCheckPath(t, report, "codex-instructions-project", filepath.Join(paths.codexHome, "AGENTS.md"))
	assertCheckStatus(t, report, "codex-config-project", model.VerifyStatusPass)
	assertCheckPath(t, report, "codex-config-project", filepath.Join(paths.codexHome, "config.toml"))
	assertCheckStatus(t, report, "codex-mcp-list", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-skills-project", model.VerifyStatusPass)
	assertCheckPath(t, report, "codex-skills-project", filepath.Join(paths.codexHome, "skills"))
	assertCheckStatus(t, report, "codex-agents-project", model.VerifyStatusPass)
	assertCheckPath(t, report, "codex-agents-project", filepath.Join(paths.codexHome, "agents"))
	if report.Summary.Fail != 0 {
		t.Fatalf("Summary.Fail = %d, want 0; checks=%#v", report.Summary.Fail, report.Checks)
	}
}

func TestVerifyCodexWarnsForMissingOptionalTargets(t *testing.T) {
	paths := newVerifyFixture(t)

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-instructions-project", model.VerifyStatusWarn)
	assertCheckStatus(t, report, "codex-config-project", model.VerifyStatusWarn)
	assertCheckStatus(t, report, "codex-mcp-list", model.VerifyStatusWarn)
	assertCheckStatus(t, report, "codex-skills-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-agents-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "sync-conflicts", model.VerifyStatusWarn)
	if report.Summary.Fail != 0 {
		t.Fatalf("Summary.Fail = %d, want 0; checks=%#v", report.Summary.Fail, report.Checks)
	}
}

func TestVerifyCodexWarnsWhenExpectedAgentsTargetIsMissing(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.claudeHome, "agents", "reviewer.md"), "agent instructions\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-agents-project", model.VerifyStatusWarn)
}

func TestVerifyCodexFailsMalformedProjectConfig(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, ".codex", "config.toml"), "[mcp_servers.github\ncommand = \"gh\"\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-config-project", model.VerifyStatusFail)
	if report.Summary.Fail == 0 {
		t.Fatalf("Summary.Fail = 0, want failure; checks=%#v", report.Checks)
	}
}

func TestVerifyCodexFailsMalformedGlobalFallbackConfig(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.codexHome, "config.toml"), "[mcp_servers.github\ncommand = \"gh\"\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-config-project", model.VerifyStatusFail)
	assertCheckPath(t, report, "codex-config-project", filepath.Join(paths.codexHome, "config.toml"))
	if report.Summary.Fail == 0 {
		t.Fatalf("Summary.Fail = 0, want failure; checks=%#v", report.Checks)
	}
}

func TestVerifyCodexWarnsWhenMCPEntriesAreMissing(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, ".codex", "config.toml"), "[profile.default]\nmodel = \"claude\"\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-config-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "codex-mcp-list", model.VerifyStatusWarn)
}

func TestVerifyCodexFailsSkillsPathCollision(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, ".agents", "skills"), "not a directory\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "codex-skills-project", model.VerifyStatusFail)
}

func TestVerifyClaudePassesProjectInstructionAndSettings(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, "CLAUDE.md"), "# Project instructions\n")
	writeFile(t, filepath.Join(paths.project, ".claude", "settings.json"), "{\"permissions\": {}}\n")

	report, err := Verify(Options{Target: "claude", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "claude-instructions-project", model.VerifyStatusPass)
	assertCheckStatus(t, report, "claude-settings-project", model.VerifyStatusPass)
	if report.Summary.Fail != 0 {
		t.Fatalf("Summary.Fail = %d, want 0; checks=%#v", report.Summary.Fail, report.Checks)
	}
}

func TestVerifyClaudeFailsMalformedProjectSettings(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, ".claude", "settings.json"), "{not-json\n")

	report, err := Verify(Options{Target: "claude", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "claude-settings-project", model.VerifyStatusFail)
}

func TestVerifyFailsOpenSyncConflicts(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, ".agent-canon", "sync-state.json"), `{"schemaVersion":"agent-canon.sync-state.v1","conflicts":[{"id":"conflict-1","status":"open"}]}`+"\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}

	assertCheckStatus(t, report, "sync-conflicts", model.VerifyStatusFail)
}

func TestVerifyReportDoesNotLeakRawSecretText(t *testing.T) {
	paths := newVerifyFixture(t)
	writeFile(t, filepath.Join(paths.project, ".codex", "config.toml"), "[mcp_servers.github]\ncommand = \"gh\"\nargs = [\""+verifierFixtureSecret+"\"]\n")

	report, err := Verify(Options{Target: "codex", Project: paths.project, ClaudeHome: paths.claudeHome, CodexHome: paths.codexHome})
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal report: %v", err)
	}
	if strings.Contains(string(payload), verifierFixtureSecret) {
		t.Fatalf("Verify report leaked fixture secret: %s", payload)
	}
}

type verifyFixture struct {
	project    string
	claudeHome string
	codexHome  string
}

func newVerifyFixture(t *testing.T) verifyFixture {
	t.Helper()
	root := t.TempDir()
	paths := verifyFixture{
		project:    filepath.Join(root, "project"),
		claudeHome: filepath.Join(root, "claude-home"),
		codexHome:  filepath.Join(root, "codex-home"),
	}
	for _, dir := range []string{paths.project, paths.claudeHome, paths.codexHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create fixture dir %s: %v", dir, err)
		}
	}
	return paths
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertCheckStatus(t *testing.T, report model.VerifyReport, id string, want model.VerifyStatus) {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID != id {
			continue
		}
		if check.Status != want {
			t.Fatalf("check %s status = %q, want %q; check=%#v", id, check.Status, want, check)
		}
		return
	}
	t.Fatalf("missing check %s in %#v", id, report.Checks)
}

func assertCheckPath(t *testing.T, report model.VerifyReport, id string, want string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID != id {
			continue
		}
		if check.Path != want {
			t.Fatalf("check %s path = %q, want %q; check=%#v", id, check.Path, want, check)
		}
		return
	}
	t.Fatalf("missing check %s in %#v", id, report.Checks)
}
