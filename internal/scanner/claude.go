package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

func scanClaudeHome(claudeHome string, codexHome string, targets codexTargets) []model.Resource {
	var resources []model.Resource
	globalAgentsHint := filepath.Join(codexHome, "AGENTS.md")

	if path, ok := existingFile(filepath.Join(claudeHome, "CLAUDE.md")); ok {
		resource := newResource("instruction:global-claude-md", model.KindInstruction, model.ScopeGlobal, path, globalAgentsHint, model.StatusCompatible, "append-to-agents-md")
		addCodexTargetWarning(&resource, targets.GlobalAgents)
		resources = append(resources, resource)
	}

	for _, name := range []string{"settings.json", "settings.local.json"} {
		if path, ok := existingFile(filepath.Join(claudeHome, name)); ok {
			resources = append(resources, newResource("config:global-"+slug(name), model.KindConfig, model.ScopeGlobal, path, "", model.StatusPartial, "review-settings-config"))
		}
	}

	if rulesDir, ok := existingDir(filepath.Join(claudeHome, "rules")); ok {
		matches, _ := filepath.Glob(filepath.Join(rulesDir, "*.md"))
		for _, match := range matches {
			path, ok := existingFile(match)
			if !ok {
				continue
			}
			status, strategy, warnings := ruleMigrationPlan(path)
			resource := newResource("rule:global-"+slug(path), model.KindRule, model.ScopeGlobal, path, globalAgentsHint, status, strategy)
			resource.Warnings = append(resource.Warnings, warnings...)
			addCodexTargetWarning(&resource, targets.GlobalAgents)
			resources = append(resources, resource)
		}
	}

	resources = append(resources, scanSkillDirs(filepath.Join(claudeHome, "skills"), model.ScopeGlobal, filepath.Join(codexHome, "skills"), "skill:global-")...)
	resources = append(resources, scanCommandEntries(filepath.Join(claudeHome, "commands"), model.ScopeGlobal, "command:global-", filepath.Join(codexHome, "skills"))...)
	resources = append(resources, scanAgentEntries(filepath.Join(claudeHome, "agents"), model.ScopeGlobal, "agent:global-", filepath.Join(codexHome, "agents"))...)
	resources = append(resources, scanPluginEntries(filepath.Join(claudeHome, "plugins"), filepath.Join(codexHome, "plugins"))...)
	resources = append(resources, scanSessionEntries(claudeHome, model.ScopeGlobal, "session:global-", []string{
		"history.jsonl",
		"session-history.jsonl",
		filepath.Join("sessions", "*"),
		filepath.Join("projects", "*", "sessions", "*"),
	})...)

	return resources
}

func scanProject(project string, targets codexTargets) []model.Resource {
	var resources []model.Resource
	projectAgentsHint := filepath.Join(project, "AGENTS.md")

	if path, ok := existingFile(filepath.Join(project, "CLAUDE.md")); ok {
		resource := newResource("instruction:project-claude-md", model.KindInstruction, model.ScopeProject, path, projectAgentsHint, model.StatusCompatible, "merge-section-into-agents-md")
		addCodexTargetWarning(&resource, targets.ProjectAgents)
		if targets.ProjectAgents == "" {
			addCodexTargetWarning(&resource, targets.GlobalAgents)
		}
		resources = append(resources, resource)
	}

	for _, name := range []string{"settings.json", "settings.local.json"} {
		if path, ok := existingFile(filepath.Join(project, ".claude", name)); ok {
			resources = append(resources, newResource("config:project-"+slug(name), model.KindConfig, model.ScopeProject, path, "", model.StatusPartial, "review-settings-config"))
		}
	}

	resources = append(resources, scanSkillDirs(filepath.Join(project, ".claude", "skills"), model.ScopeProject, filepath.Join(project, ".agents", "skills"), "skill:project-")...)
	resources = append(resources, scanCommandEntries(filepath.Join(project, ".claude", "commands"), model.ScopeProject, "command:project-", filepath.Join(project, ".agents", "skills"))...)
	resources = append(resources, scanAgentEntries(filepath.Join(project, ".claude", "agents"), model.ScopeProject, "agent:project-", filepath.Join(project, ".codex", "agents"))...)
	resources = append(resources, scanSessionEntries(filepath.Join(project, ".claude"), model.ScopeProject, "session:project-", []string{
		"history.jsonl",
		"session-history.jsonl",
		filepath.Join("sessions", "*"),
	})...)

	return resources
}

func scanSkillDirs(root string, scope model.Scope, targetRoot string, idPrefix string) []model.Resource {
	root, ok := existingDir(root)
	if !ok {
		return nil
	}
	entries, err := filepath.Glob(filepath.Join(root, "*", "SKILL.md"))
	if err != nil {
		return nil
	}
	resources := make([]model.Resource, 0, len(entries))
	for _, entry := range entries {
		path, ok := existingFile(entry)
		if !ok {
			continue
		}
		name := filepath.Base(filepath.Dir(path))
		resources = append(resources, newResource(idPrefix+name, model.KindSkill, scope, path, filepath.Join(targetRoot, name, "SKILL.md"), model.StatusPartial, "convert-skill-with-review"))
	}
	return resources
}

func scanCommandEntries(root string, scope model.Scope, idPrefix string, targetRoot string) []model.Resource {
	return scanEntries(root, model.KindCommand, scope, idPrefix, "convert-command-to-skill-or-workflow", func(name string) string {
		return filepath.Join(targetRoot, name, "SKILL.md")
	})
}

func scanAgentEntries(root string, scope model.Scope, idPrefix string, targetRoot string) []model.Resource {
	return scanEntries(root, model.KindAgent, scope, idPrefix, "rewrite-agent-schema", func(name string) string {
		return filepath.Join(targetRoot, name+".toml")
	})
}

func scanPluginEntries(root string, targetRoot string) []model.Resource {
	return scanEntries(root, model.KindConfig, model.ScopeGlobal, "plugin:global-", "review-plugin-adaptation", func(name string) string {
		return filepath.Join(targetRoot, name)
	})
}

func scanSessionEntries(root string, scope model.Scope, idPrefix string, patterns []string) []model.Resource {
	root, ok := existingDir(root)
	if !ok {
		return nil
	}

	seen := map[string]bool{}
	var resources []model.Resource
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			continue
		}
		for _, match := range matches {
			path, err := absClean(match)
			if err != nil || seen[path] {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			seen[path] = true
			name := slug(path)
			if info.IsDir() {
				name = filepath.Base(path)
			}
			resources = append(resources, newResource(idPrefix+name, model.KindSession, scope, path, "", model.StatusUnsupported, "skip-session-migration"))
		}
	}
	return resources
}

func scanMemoryItems(claudeHome string) []model.Resource {
	projectsRoot, ok := existingDir(filepath.Join(claudeHome, "projects"))
	if !ok {
		return nil
	}

	memoryFiles, err := filepath.Glob(filepath.Join(projectsRoot, "*", "memory", "*.md"))
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	resources := make([]model.Resource, 0, len(memoryFiles))
	for _, entry := range memoryFiles {
		path, ok := existingFile(entry)
		if !ok || seen[path] {
			continue
		}
		seen[path] = true
		projectName := filepath.Base(filepath.Dir(filepath.Dir(path)))
		name := slug(path)
		resources = append(resources, newResource("memory:project-"+projectName+"-"+name, model.KindMemoryItem, model.ScopeProject, path, "", model.StatusPartial, "review-memory-candidate"))
	}
	return resources
}

func scanEntries(root string, kind model.ResourceKind, scope model.Scope, idPrefix string, strategy string, targetHint func(string) string) []model.Resource {
	root, ok := existingDir(root)
	if !ok {
		return nil
	}
	entries, err := filepath.Glob(filepath.Join(root, "*"))
	if err != nil {
		return nil
	}
	resources := make([]model.Resource, 0, len(entries))
	for _, entry := range entries {
		path, err := absClean(entry)
		if err != nil || shouldSkipEntry(path) {
			continue
		}
		name := slug(path)
		resources = append(resources, newResource(idPrefix+name, kind, scope, path, targetHint(name), model.StatusPartial, strategy))
	}
	return resources
}

func ruleMigrationPlan(path string) (model.Status, string, []model.Warning) {
	if !hasPathScopedFrontmatter(path) {
		return model.StatusCompatible, "merge-rule-into-agents-md", nil
	}
	warning := model.Warning{
		Code:    "path-scoped-rule-review",
		Message: "Claude rule uses paths frontmatter; review manually before placing it in an always-on Codex AGENTS.md file.",
	}
	return model.StatusPartial, "review-path-scoped-rule", []model.Warning{warning}
}

func hasPathScopedFrontmatter(path string) bool {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	lines := strings.Split(strings.TrimPrefix(string(contents), "\ufeff"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			return false
		}
		if strings.HasPrefix(trimmed, "paths:") {
			return true
		}
	}
	return false
}

func shouldSkipEntry(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}
