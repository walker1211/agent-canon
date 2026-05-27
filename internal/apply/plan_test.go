package apply_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/planner"
)

const fixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestBuildCodexPlanMapsProjectFilesAndSkipsGlobalByDefault(t *testing.T) {
	root := t.TempDir()
	scan := syntheticScan(t, root,
		resource(t, root, model.ScopeProject, model.KindInstruction, "instruction:project", "CLAUDE.md", "AGENTS.md", model.StatusCompatible, "# Project\n"),
		resource(t, root, model.ScopeProject, model.KindSkill, "skill:project-review", filepath.Join(".claude", "skills", "review", "SKILL.md"), filepath.Join(".agents", "skills", "review", "SKILL.md"), model.StatusPartial, "# Review\n"),
		resource(t, root, model.ScopeGlobal, model.KindInstruction, "instruction:global", filepath.Join("claude-home", "CLAUDE.md"), filepath.Join("codex-home", "AGENTS.md"), model.StatusCompatible, "# Global\n"),
	)

	plan, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scan, Plan: planner.Build(scan)})
	if err != nil {
		t.Fatalf("BuildCodexPlan returned error: %v", err)
	}

	assertChangePath(t, plan, filepath.Join(scan.Project, "AGENTS.md"))
	assertChangePath(t, plan, filepath.Join(scan.Project, ".codex", "config.toml"))
	assertChangePath(t, plan, filepath.Join(scan.Project, ".agents", "skills", "review", "SKILL.md"))
	assertNoChangePath(t, plan, filepath.Join(scan.Project, "migration-report.md"))
	assertNoChangePath(t, plan, filepath.Join(scan.CodexHome, "AGENTS.md"))
	if !hasWarning(plan.Warnings, "global-skipped") {
		t.Fatalf("warnings = %#v, want global-skipped warning", plan.Warnings)
	}
}

func TestBuildCodexPlanIncludesGlobalFilesWhenEnabled(t *testing.T) {
	root := t.TempDir()
	scan := syntheticScan(t, root,
		resource(t, root, model.ScopeGlobal, model.KindInstruction, "instruction:global", filepath.Join("claude-home", "CLAUDE.md"), filepath.Join("codex-home", "AGENTS.md"), model.StatusCompatible, "# Global\n"),
	)

	plan, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scan, Plan: planner.Build(scan), IncludeGlobal: true})
	if err != nil {
		t.Fatalf("BuildCodexPlan returned error: %v", err)
	}

	assertNoChangePath(t, plan, filepath.Join(scan.Project, "AGENTS.md"))
	assertChangePath(t, plan, filepath.Join(scan.CodexHome, "AGENTS.md"))
	assertChangePath(t, plan, filepath.Join(scan.CodexHome, "config.toml"))
}

func TestBuildCodexPlanClassifiesCreateModifyAndNoop(t *testing.T) {
	root := t.TempDir()
	scan := syntheticScan(t, root,
		resource(t, root, model.ScopeProject, model.KindInstruction, "instruction:project", "CLAUDE.md", "AGENTS.md", model.StatusCompatible, "# Project\n"),
	)

	created, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scan, Plan: planner.Build(scan)})
	if err != nil {
		t.Fatalf("BuildCodexPlan create returned error: %v", err)
	}
	agents := requireChange(t, created, filepath.Join(scan.Project, "AGENTS.md"))
	if agents.Action != model.ApplyActionCreate {
		t.Fatalf("missing target action = %q, want create", agents.Action)
	}

	writeFile(t, agents.Path, string(agents.Contents))
	noop, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scan, Plan: planner.Build(scan)})
	if err != nil {
		t.Fatalf("BuildCodexPlan noop returned error: %v", err)
	}
	agents = requireChange(t, noop, filepath.Join(scan.Project, "AGENTS.md"))
	if agents.Action != model.ApplyActionNoop || agents.BeforeHash != agents.AfterHash {
		t.Fatalf("same target change = %#v, want noop with matching hashes", agents.ApplyFileChange)
	}

	writeFile(t, agents.Path, "old\n")
	modified, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scan, Plan: planner.Build(scan)})
	if err != nil {
		t.Fatalf("BuildCodexPlan modify returned error: %v", err)
	}
	agents = requireChange(t, modified, filepath.Join(scan.Project, "AGENTS.md"))
	if agents.Action != model.ApplyActionModify || agents.BeforeHash == "" || agents.BeforeHash == agents.AfterHash {
		t.Fatalf("modified target change = %#v, want modify with distinct hashes", agents.ApplyFileChange)
	}
}

func TestBuildCodexPlanDoesNotLeakSecretSourceContent(t *testing.T) {
	root := t.TempDir()
	scan := syntheticScan(t, root,
		resource(t, root, model.ScopeProject, model.KindCommand, "command:project-token-command", filepath.Join(".claude", "commands", "token-command.md"), filepath.Join(".agents", "skills", "token-command", "SKILL.md"), model.StatusPartial, "# Token\n\nGITHUB_TOKEN="+fixtureSecret+"\n"),
	)

	plan, err := applypkg.BuildCodexPlan(applypkg.CodexPlanInput{Scan: scan, Plan: planner.Build(scan)})
	if err != nil {
		t.Fatalf("BuildCodexPlan returned error: %v", err)
	}

	for _, change := range plan.Changes {
		if strings.Contains(string(change.Contents), fixtureSecret) {
			t.Fatalf("%s leaked fixture secret", change.Path)
		}
	}
	command := requireChange(t, plan, filepath.Join(scan.Project, ".agents", "skills", "token-command", "SKILL.md"))
	if !strings.Contains(string(command.Contents), "GITHUB_TOKEN=<REDACTED>") {
		t.Fatalf("command contents missing redaction marker:\n%s", command.Contents)
	}
}

func syntheticScan(t *testing.T, root string, resources ...model.Resource) model.ScanReport {
	t.Helper()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	for _, path := range []string{project, claudeHome, codexHome} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	return model.ScanReport{
		SchemaVersion: model.ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       project,
		ClaudeHome:    claudeHome,
		CodexHome:     codexHome,
		Resources:     resources,
	}
}

func resource(t *testing.T, root string, scope model.Scope, kind model.ResourceKind, id string, sourceRel string, targetRel string, status model.Status, contents string) model.Resource {
	t.Helper()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	sourceRoot := project
	targetRoot := project
	if scope == model.ScopeGlobal {
		sourceRoot = root
		targetRoot = root
	}
	sourcePath := filepath.Join(sourceRoot, sourceRel)
	targetPath := filepath.Join(targetRoot, targetRel)
	if scope == model.ScopeGlobal {
		if strings.HasPrefix(sourceRel, "claude-home") {
			sourcePath = filepath.Join(root, sourceRel)
		} else {
			sourcePath = filepath.Join(claudeHome, sourceRel)
		}
		if strings.HasPrefix(targetRel, "codex-home") {
			targetPath = filepath.Join(root, targetRel)
		} else {
			targetPath = filepath.Join(codexHome, targetRel)
		}
	}
	writeFile(t, sourcePath, contents)
	return model.Resource{
		ID:             id,
		Kind:           kind,
		Scope:          scope,
		SourceTool:     "claude",
		SourcePath:     sourcePath,
		TargetTool:     "codex",
		TargetPathHint: targetPath,
		Status:         status,
		Strategy:       "apply-test",
	}
}

func requireChange(t *testing.T, plan applypkg.CodexPlan, path string) applypkg.FileChange {
	t.Helper()
	for _, change := range plan.Changes {
		if change.Path == path {
			return change
		}
	}
	t.Fatalf("change %q not found in %#v", path, plan.Changes)
	return applypkg.FileChange{}
}

func assertChangePath(t *testing.T, plan applypkg.CodexPlan, path string) {
	t.Helper()
	requireChange(t, plan, path)
}

func assertNoChangePath(t *testing.T, plan applypkg.CodexPlan, path string) {
	t.Helper()
	for _, change := range plan.Changes {
		if change.Path == path {
			t.Fatalf("change %q found unexpectedly in %#v", path, plan.Changes)
		}
	}
}

func hasWarning(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
