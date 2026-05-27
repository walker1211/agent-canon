package scanner_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
)

func TestScanBasicFixtureDiscoversClaudeResources(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "basic")

	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	globalInstruction := requireResource(t, report.Resources, "instruction:global-claude-md")
	assertResource(t, globalInstruction, model.KindInstruction, model.ScopeGlobal, model.StatusCompatible)
	if globalInstruction.TargetPathHint == "" {
		t.Fatalf("global instruction target hint is empty")
	}
	if len(globalInstruction.Warnings) == 0 {
		t.Fatalf("global instruction should warn about existing Codex AGENTS.md")
	}

	projectInstruction := requireResource(t, report.Resources, "instruction:project-claude-md")
	assertResource(t, projectInstruction, model.KindInstruction, model.ScopeProject, model.StatusCompatible)
	if projectInstruction.TargetPathHint == "" {
		t.Fatalf("project instruction target hint is empty")
	}
	if len(projectInstruction.Warnings) == 0 {
		t.Fatalf("project instruction should warn about existing Codex AGENTS.md")
	}

	rule := requireResource(t, report.Resources, "rule:global-language")
	assertResource(t, rule, model.KindRule, model.ScopeGlobal, model.StatusCompatible)

	skill := requireResource(t, report.Resources, "skill:project-sample-skill")
	assertResource(t, skill, model.KindSkill, model.ScopeProject, model.StatusPartial)
	if want := filepath.Join(report.Project, ".agents", "skills", "sample-skill", "SKILL.md"); skill.TargetPathHint != want {
		t.Fatalf("skill target hint = %q, want %q", skill.TargetPathHint, want)
	}

	if report.Summary.Compatible < 3 {
		t.Fatalf("compatible summary = %d, want at least 3", report.Summary.Compatible)
	}
	if report.Summary.Partial < 1 {
		t.Fatalf("partial summary = %d, want at least 1", report.Summary.Partial)
	}
	if report.Summary.Dangerous != 0 {
		t.Fatalf("dangerous summary = %d, want 0", report.Summary.Dangerous)
	}
}

func TestScanWarnsWhenBasicCodexConfigTargetExists(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "basic")

	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	resource := requireResource(t, report.Resources, "config:global-settings")
	if !hasWarningCode(resource.Warnings, "existing-codex-target") {
		t.Fatalf("global settings warnings missing existing-codex-target: %#v", resource.Warnings)
	}
}

func TestScanWarnsWhenCodexTargetsExistForPartialResources(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")

	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{}")
	writeFile(t, filepath.Join(project, ".claude", "settings.json"), "{}")
	writeFile(t, filepath.Join(claudeHome, "commands", "global-build.md"), "global build")
	writeFile(t, filepath.Join(project, ".claude", "commands", "deploy.md"), "deploy")
	writeFile(t, filepath.Join(claudeHome, "skills", "global-skill", "SKILL.md"), "skill")
	writeFile(t, filepath.Join(project, ".claude", "skills", "project-skill", "SKILL.md"), "skill")
	writeFile(t, filepath.Join(claudeHome, "agents", "global-reviewer.md"), "global reviewer")
	writeFile(t, filepath.Join(project, ".claude", "agents", "reviewer.md"), "reviewer")
	writeFile(t, filepath.Join(claudeHome, "plugins", "sample-plugin", "plugin.json"), "{}")

	writeFile(t, filepath.Join(codexHome, "config.toml"), "")
	writeFile(t, filepath.Join(project, ".codex", "config.toml"), "")
	writeFile(t, filepath.Join(codexHome, "skills", "global-build", "SKILL.md"), "")
	writeFile(t, filepath.Join(project, ".agents", "skills", "deploy", "SKILL.md"), "")
	writeFile(t, filepath.Join(codexHome, "skills", "global-skill", "SKILL.md"), "")
	writeFile(t, filepath.Join(project, ".agents", "skills", "project-skill", "SKILL.md"), "")
	writeFile(t, filepath.Join(codexHome, "agents", "global-reviewer.toml"), "")
	writeFile(t, filepath.Join(project, ".codex", "agents", "reviewer.toml"), "")
	writeFile(t, filepath.Join(codexHome, "plugins", "sample-plugin", "plugin.json"), "{}")
	writeFile(t, filepath.Join(codexHome, "memories", "legacy.md"), "legacy")

	report, err := scanner.Scan(scanner.Options{Project: project, ClaudeHome: claudeHome, CodexHome: codexHome})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	for _, id := range []string{
		"config:global-settings",
		"config:project-settings",
		"command:global-global-build",
		"command:project-deploy",
		"skill:global-global-skill",
		"skill:project-project-skill",
		"agent:global-global-reviewer",
		"agent:project-reviewer",
		"plugin:global-sample-plugin",
	} {
		resource := requireResource(t, report.Resources, id)
		if !hasWarningCode(resource.Warnings, "existing-codex-target") {
			t.Fatalf("%s warnings missing existing-codex-target: %#v", id, resource.Warnings)
		}
	}
	if !hasWarningCode(report.Warnings, "existing-codex-memories") {
		t.Fatalf("report warnings missing existing-codex-memories: %#v", report.Warnings)
	}
}

func TestScanPartialResourcesUseTargetHintsAndStableStrategies(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")

	writeFile(t, filepath.Join(project, ".claude", "commands", "deploy.md"), "deploy")
	writeFile(t, filepath.Join(project, ".claude", "agents", "reviewer.md"), "reviewer")
	writeFile(t, filepath.Join(claudeHome, "commands", "global-build.md"), "global build")
	writeFile(t, filepath.Join(claudeHome, "agents", "global-reviewer.md"), "global reviewer")
	writeFile(t, filepath.Join(claudeHome, "plugins", "sample-plugin", "plugin.json"), "{}")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{}")
	writeFile(t, filepath.Join(project, ".claude", "settings.json"), "{}")
	writeFile(t, filepath.Join(claudeHome, "skills", "global-skill", "SKILL.md"), "skill")
	writeFile(t, filepath.Join(project, ".claude", "skills", "project-skill", "SKILL.md"), "skill")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("create codex home: %v", err)
	}

	report, err := scanner.Scan(scanner.Options{Project: project, ClaudeHome: claudeHome, CodexHome: codexHome})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	tests := []struct {
		id             string
		targetPathHint string
		strategy       string
	}{
		{id: "command:project-deploy", targetPathHint: filepath.Join(report.Project, ".agents", "skills", "deploy", "SKILL.md"), strategy: "convert-command-to-skill-or-workflow"},
		{id: "agent:project-reviewer", targetPathHint: filepath.Join(report.Project, ".codex", "agents", "reviewer.toml"), strategy: "rewrite-agent-schema"},
		{id: "command:global-global-build", targetPathHint: filepath.Join(report.CodexHome, "skills", "global-build", "SKILL.md"), strategy: "convert-command-to-skill-or-workflow"},
		{id: "agent:global-global-reviewer", targetPathHint: filepath.Join(report.CodexHome, "agents", "global-reviewer.toml"), strategy: "rewrite-agent-schema"},
		{id: "plugin:global-sample-plugin", targetPathHint: filepath.Join(report.CodexHome, "plugins", "sample-plugin"), strategy: "review-plugin-adaptation"},
		{id: "config:global-settings", targetPathHint: "", strategy: "review-settings-config"},
		{id: "config:project-settings", targetPathHint: "", strategy: "review-settings-config"},
		{id: "skill:global-global-skill", targetPathHint: filepath.Join(report.CodexHome, "skills", "global-skill", "SKILL.md"), strategy: "convert-skill-with-review"},
		{id: "skill:project-project-skill", targetPathHint: filepath.Join(report.Project, ".agents", "skills", "project-skill", "SKILL.md"), strategy: "convert-skill-with-review"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			resource := requireResource(t, report.Resources, tt.id)
			if resource.TargetPathHint != tt.targetPathHint {
				t.Fatalf("target hint = %q, want %q", resource.TargetPathHint, tt.targetPathHint)
			}
			if resource.Strategy != tt.strategy {
				t.Fatalf("strategy = %q, want %q", resource.Strategy, tt.strategy)
			}
		})
	}
}

func TestScanIncludesMemoryOnlyWhenRequested(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "basic")
	opts := scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	}

	defaultReport, err := scanner.Scan(opts)
	if err != nil {
		t.Fatalf("Scan default returned error: %v", err)
	}
	if hasResource(defaultReport.Resources, "memory:project-demo-MEMORY") || hasResource(defaultReport.Resources, "memory:project-demo-notes") {
		t.Fatalf("memory resources present by default: %#v", defaultReport.Resources)
	}

	opts.IncludeMemory = true
	includedReport, err := scanner.Scan(opts)
	if err != nil {
		t.Fatalf("Scan with IncludeMemory returned error: %v", err)
	}

	memory := requireResource(t, includedReport.Resources, "memory:project-demo-MEMORY")
	assertResource(t, memory, model.KindMemoryItem, model.ScopeProject, model.StatusPartial)
	if memory.Strategy != "review-memory-candidate" {
		t.Fatalf("memory strategy = %q, want review-memory-candidate", memory.Strategy)
	}
	if memory.TargetPathHint != "" {
		t.Fatalf("memory target hint = %q, want empty", memory.TargetPathHint)
	}
	notes := requireResource(t, includedReport.Resources, "memory:project-demo-notes")
	assertResource(t, notes, model.KindMemoryItem, model.ScopeProject, model.StatusPartial)

	count := 0
	for _, resource := range includedReport.Resources {
		if resource.ID == "memory:project-demo-MEMORY" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("MEMORY.md resource count = %d, want 1", count)
	}
}

func TestScanRedactsMCPEnvSecrets(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "secrets")

	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report) error = %v", err)
	}
	if strings.Contains(string(payload), "ghp_agent_canon_fixture_secret_must_not_leak") {
		t.Fatalf("marshaled scan JSON leaked fixture secret")
	}

	resource := requireResource(t, report.Resources, "mcp:global-fixture-github")
	assertResource(t, resource, model.KindMCPServer, model.ScopeGlobal, model.StatusDangerous)
	if !hasWarningCode(resource.Warnings, "secret-redacted") {
		t.Fatalf("MCP resource warnings missing secret-redacted: %#v", resource.Warnings)
	}
	if !hasWarningCode(report.Warnings, "secret-redacted") {
		t.Fatalf("report warnings missing secret-redacted: %#v", report.Warnings)
	}
}

func TestScanDetectsUnsupportedHooks(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "unsupported")

	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	resource := requireResource(t, report.Resources, "hook:global-PreToolUse")
	if resource.Kind != model.KindHook {
		t.Fatalf("hook kind = %q, want %q", resource.Kind, model.KindHook)
	}
	if resource.Status != model.StatusUnsupported && resource.Status != model.StatusDangerous {
		t.Fatalf("hook status = %q, want unsupported or dangerous", resource.Status)
	}
	if !hasWarningCode(resource.Warnings, "hook-unsupported") {
		t.Fatalf("hook warnings missing hook-unsupported: %#v", resource.Warnings)
	}
}

func TestScanDetectsUnsupportedSessionHistory(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "unsupported")

	report, err := scanner.Scan(scanner.Options{
		Project:    filepath.Join(fixture, "project"),
		ClaudeHome: filepath.Join(fixture, "claude-home"),
		CodexHome:  filepath.Join(fixture, "codex-home"),
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	resource := requireResource(t, report.Resources, "session:global-session-history")
	assertResource(t, resource, model.KindSession, model.ScopeGlobal, model.StatusUnsupported)
	if resource.Strategy != "skip-session-migration" {
		t.Fatalf("session strategy = %q, want skip-session-migration", resource.Strategy)
	}
	if resource.TargetPathHint != "" {
		t.Fatalf("session target hint = %q, want empty", resource.TargetPathHint)
	}

	plan := planner.Build(report)
	operation := requireOperation(t, plan.Operations, resource.ID)
	if operation.Action != "skip" && operation.Action != "manual" {
		t.Fatalf("session action = %q, want skip or manual", operation.Action)
	}
	if operation.Action != "skip" {
		t.Fatalf("session action = %q, preferred skip", operation.Action)
	}
}

func TestScanReturnsErrorForMalformedSettingsJSON(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")

	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("create codex home: %v", err)
	}

	if _, err := scanner.Scan(scanner.Options{Project: project, ClaudeHome: claudeHome, CodexHome: codexHome}); err == nil {
		t.Fatalf("Scan returned nil error for malformed settings JSON")
	}
}

func TestScanReturnsErrorForMissingRequiredPaths(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "basic")
	existingProject := filepath.Join(fixture, "project")
	existingClaudeHome := filepath.Join(fixture, "claude-home")
	existingCodexHome := filepath.Join(fixture, "codex-home")

	tests := []struct {
		name string
		opts scanner.Options
	}{
		{
			name: "missing project",
			opts: scanner.Options{Project: filepath.Join(fixture, "missing-project"), ClaudeHome: existingClaudeHome, CodexHome: existingCodexHome},
		},
		{
			name: "missing claude home",
			opts: scanner.Options{Project: existingProject, ClaudeHome: filepath.Join(fixture, "missing-claude-home"), CodexHome: existingCodexHome},
		},
		{
			name: "missing codex home",
			opts: scanner.Options{Project: existingProject, ClaudeHome: existingClaudeHome, CodexHome: filepath.Join(fixture, "missing-codex-home")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := scanner.Scan(tt.opts); err == nil {
				t.Fatalf("Scan returned nil error")
			}
		})
	}
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

func requireResource(t *testing.T, resources []model.Resource, id string) model.Resource {
	t.Helper()
	for _, resource := range resources {
		if resource.ID == id {
			return resource
		}
	}
	t.Fatalf("resource %q not found in %#v", id, resources)
	return model.Resource{}
}

func hasResource(resources []model.Resource, id string) bool {
	for _, resource := range resources {
		if resource.ID == id {
			return true
		}
	}
	return false
}

func requireOperation(t *testing.T, operations []model.Operation, resourceID string) model.Operation {
	t.Helper()
	for _, operation := range operations {
		if operation.ResourceID == resourceID {
			return operation
		}
	}
	t.Fatalf("operation for resource %q not found in %#v", resourceID, operations)
	return model.Operation{}
}

func hasWarningCode(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func assertResource(t *testing.T, resource model.Resource, kind model.ResourceKind, scope model.Scope, status model.Status) {
	t.Helper()
	if resource.Kind != kind {
		t.Fatalf("%s kind = %q, want %q", resource.ID, resource.Kind, kind)
	}
	if resource.Scope != scope {
		t.Fatalf("%s scope = %q, want %q", resource.ID, resource.Scope, scope)
	}
	if resource.Status != status {
		t.Fatalf("%s status = %q, want %q", resource.ID, resource.Status, status)
	}
	if resource.SourcePath == "" {
		t.Fatalf("%s source path is empty", resource.ID)
	}
	if !filepath.IsAbs(resource.SourcePath) {
		t.Fatalf("%s source path = %q, want absolute", resource.ID, resource.SourcePath)
	}
}
