package scanner

import (
	"path/filepath"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

type codexTargets struct {
	GlobalAgents  string
	ProjectAgents string
	GlobalConfig  string
	ProjectConfig string
	Existing      map[string]string
	Warnings      []model.Warning
}

func detectCodexTargets(project string, codexHome string) codexTargets {
	targets := codexTargets{Existing: map[string]string{}}
	if path, ok := existingFile(filepath.Join(codexHome, "AGENTS.md")); ok {
		targets.GlobalAgents = path
		addExistingTarget(&targets, path)
	}
	if path, ok := existingFile(filepath.Join(project, "AGENTS.md")); ok {
		targets.ProjectAgents = path
		addExistingTarget(&targets, path)
	}
	if path, ok := existingFile(filepath.Join(codexHome, "config.toml")); ok {
		targets.GlobalConfig = path
		addExistingTarget(&targets, path)
	}
	if path, ok := existingFile(filepath.Join(project, ".codex", "config.toml")); ok {
		targets.ProjectConfig = path
		addExistingTarget(&targets, path)
	}
	addExistingTargetGlobs(&targets,
		filepath.Join(codexHome, "skills", "*", "SKILL.md"),
		filepath.Join(project, ".agents", "skills", "*", "SKILL.md"),
		filepath.Join(codexHome, "agents", "*"),
		filepath.Join(codexHome, "agents", "*.toml"),
		filepath.Join(project, ".codex", "agents", "*"),
		filepath.Join(project, ".codex", "agents", "*.toml"),
		filepath.Join(codexHome, "plugins", "*"),
	)
	if memories, err := filepath.Glob(filepath.Join(codexHome, "memories", "*")); err == nil && len(memories) > 0 {
		targets.Warnings = append(targets.Warnings, model.Warning{
			Code:    "existing-codex-memories",
			Message: "Codex memories already exist and should be reviewed before any memory migration.",
		})
	}
	return targets
}

func addExistingTarget(targets *codexTargets, path string) {
	clean, err := absClean(path)
	if err != nil {
		return
	}
	targets.Existing[clean] = clean
}

func addExistingTargetGlobs(targets *codexTargets, patterns ...string) {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			if _, ok := existingFile(match); ok {
				addExistingTarget(targets, match)
				continue
			}
			if _, ok := existingDir(match); ok {
				addExistingTarget(targets, match)
			}
		}
	}
}

func existingTargetWarning(path string) model.Warning {
	return model.Warning{
		Code:    "existing-codex-target",
		Message: "Codex target already exists and should be reviewed before merging: " + path,
	}
}

func addCodexTargetWarning(resource *model.Resource, targetPath string) {
	if targetPath == "" || resourceHasWarning(resource, "existing-codex-target") {
		return
	}
	resource.Warnings = append(resource.Warnings, existingTargetWarning(targetPath))
}

func annotateCodexTargetWarnings(resources []model.Resource, targets codexTargets) {
	for i := range resources {
		resource := &resources[i]
		if resource.TargetPathHint != "" {
			if path, ok := targets.Existing[filepath.Clean(resource.TargetPathHint)]; ok {
				addCodexTargetWarning(resource, path)
			}
		}
		switch resource.ID {
		case "config:global-settings", "config:global-settings.local", "mcp:global-settings", "mcp:local-global-settings":
			addCodexTargetWarning(resource, targets.GlobalConfig)
		case "config:project-settings", "config:project-settings.local", "mcp:project-settings", "mcp:local-project-settings":
			addCodexTargetWarning(resource, targets.ProjectConfig)
		}
		if resource.Kind == model.KindMCPServer && resource.Scope == model.ScopeGlobal {
			addCodexTargetWarning(resource, targets.GlobalConfig)
		}
		if resource.Kind == model.KindMCPServer && resource.Scope == model.ScopeProject {
			addCodexTargetWarning(resource, targets.ProjectConfig)
		}
	}
}

func resourceHasWarning(resource *model.Resource, code string) bool {
	for _, warning := range resource.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
