package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

type ParseError struct {
	Path string
	Err  error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("parse Claude settings %q: %v", e.Path, e.Err)
}

func (e ParseError) Unwrap() error {
	return e.Err
}

type claudeSettings struct {
	MCPServers map[string]mcpServerConfig `json:"mcpServers"`
	Hooks      map[string]json.RawMessage `json:"hooks"`
}

type mcpServerConfig struct {
	Env map[string]any `json:"env"`
}

func scanClaudeSettings(project string, claudeHome string, codexHome string) ([]model.Resource, []model.Warning, error) {
	var resources []model.Resource
	var warnings []model.Warning

	settingsFiles := []struct {
		path       string
		scope      model.Scope
		idScope    string
		targetHint string
	}{
		{path: filepath.Join(claudeHome, "settings.json"), scope: model.ScopeGlobal, idScope: "global", targetHint: filepath.Join(codexHome, "config.toml")},
		{path: filepath.Join(claudeHome, "settings.local.json"), scope: model.ScopeLocal, idScope: "local-global", targetHint: filepath.Join(codexHome, "config.toml")},
		{path: filepath.Join(project, ".claude", "settings.json"), scope: model.ScopeProject, idScope: "project", targetHint: filepath.Join(project, ".codex", "config.toml")},
		{path: filepath.Join(project, ".claude", "settings.local.json"), scope: model.ScopeLocal, idScope: "local-project", targetHint: filepath.Join(project, ".codex", "config.toml")},
	}

	for _, file := range settingsFiles {
		fileResources, fileWarnings, err := scanSettingsFile(file.path, file.scope, file.idScope, file.targetHint)
		if err != nil {
			return nil, nil, err
		}
		resources = append(resources, fileResources...)
		warnings = append(warnings, fileWarnings...)
	}

	return resources, warnings, nil
}

func scanSettingsFile(path string, scope model.Scope, idScope string, targetHint string) ([]model.Resource, []model.Warning, error) {
	settingsPath, ok := existingFile(path)
	if !ok {
		return nil, nil, nil
	}

	payload, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read Claude settings %q: %w", settingsPath, err)
	}

	var settings claudeSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		return nil, nil, ParseError{Path: settingsPath, Err: err}
	}

	var resources []model.Resource
	var warnings []model.Warning

	serverNames := sortedKeys(settings.MCPServers)
	for _, serverName := range serverNames {
		server := settings.MCPServers[serverName]
		resource := newResource("mcp:"+idScope+"-"+idSlug(serverName), model.KindMCPServer, scope, settingsPath, targetHint, model.StatusPartial, "manual-mcp-server-review")
		resource.SourceName = serverName

		envKeys := sortedKeys(server.Env)
		for _, key := range envKeys {
			if _, redacted := security.RedactIfSecret(key, fmt.Sprint(server.Env[key])); !redacted {
				continue
			}
			resource.Status = model.StatusDangerous
			resource.Strategy = "manual-redacted-mcp-secret"
			warning := model.Warning{
				Code:    "secret-redacted",
				Message: fmt.Sprintf("MCP server %s contains env key %s; value redacted and requires manual target configuration.", serverName, key),
			}
			resource.Warnings = append(resource.Warnings, warning)
			warnings = append(warnings, warning)
		}

		resources = append(resources, resource)
	}

	hookNames := sortedKeys(settings.Hooks)
	for _, hookName := range hookNames {
		warning := model.Warning{
			Code:    "hook-unsupported",
			Message: fmt.Sprintf("Claude hook %s is not executed or migrated automatically; review manually for Codex.", hookName),
		}
		resource := newResource("hook:"+idScope+"-"+idSlug(hookName), model.KindHook, scope, settingsPath, "", model.StatusUnsupported, "skip-hook-migration")
		resource.Warnings = append(resource.Warnings, warning)
		resources = append(resources, resource)
		warnings = append(warnings, warning)
	}

	return resources, warnings, nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func idSlug(value string) string {
	value = filepath.Base(value)
	value = strings.TrimSuffix(value, filepath.Ext(value))
	value = strings.TrimSpace(value)
	if value == "" || value == "." || value == string(filepath.Separator) {
		return "unnamed"
	}
	return value
}
