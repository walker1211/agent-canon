package verifier

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
	"github.com/zhangyoujun/agent-canon/internal/security"
	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

type Options struct {
	Target        string
	Project       string
	ClaudeHome    string
	CodexHome     string
	IncludeMemory bool
}

func Verify(opts Options) (model.VerifyReport, error) {
	project, err := absClean(opts.Project)
	if err != nil {
		return model.VerifyReport{}, err
	}
	claudeHome, err := absClean(opts.ClaudeHome)
	if err != nil {
		return model.VerifyReport{}, err
	}
	codexHome, err := absClean(opts.CodexHome)
	if err != nil {
		return model.VerifyReport{}, err
	}
	report := model.VerifyReport{
		SchemaVersion: model.VerifySchemaVersion,
		Target:        opts.Target,
		Project:       project,
		ClaudeHome:    claudeHome,
		CodexHome:     codexHome,
		Checks:        []model.VerifyCheck{},
		Warnings:      []model.Warning{},
	}

	if opts.Target != "codex" && opts.Target != "claude" {
		return model.VerifyReport{}, fmt.Errorf("unsupported verify target %q", opts.Target)
	}

	scanReport, scanErr := scanner.Scan(scanner.Options{Project: project, ClaudeHome: claudeHome, CodexHome: codexHome, IncludeMemory: opts.IncludeMemory})
	scanOK := scanErr == nil

	switch opts.Target {
	case "codex":
		verifyCodex(&report, project, codexHome, scanReport, scanOK)
	case "claude":
		verifyClaude(&report, project)
	}
	verifySyncState(&report, project)
	verifyScanner(&report, scanReport, scanErr)
	report.Summary = summarize(report.Checks)
	return report, nil
}

func verifyCodex(report *model.VerifyReport, project string, codexHome string, scanReport model.ScanReport, scanOK bool) {
	checkReadableTextWithFallback(
		report,
		"codex-instructions-project",
		filepath.Join(project, "AGENTS.md"),
		filepath.Join(codexHome, "AGENTS.md"),
		"AGENTS.md is readable.",
		"Global AGENTS.md is readable; no project AGENTS.md is required for global Codex setup.",
		"AGENTS.md has not been generated yet.",
	)
	configPaths := []string{filepath.Join(project, ".codex", "config.toml"), filepath.Join(codexHome, "config.toml")}
	verifyCodexConfig(report, configPaths[0], configPaths[1])
	verifyCodexMCP(report, configPaths)
	verifyStructuredDirWithFallbackIfExpected(
		report,
		"codex-skills-project",
		filepath.Join(project, ".agents", "skills"),
		filepath.Join(codexHome, "skills"),
		filepath.Join("*", "SKILL.md"),
		"Codex skills directory is recognizable.",
		"Global Codex skills directory is recognizable; no project skills directory is required for global Codex setup.",
		"Codex skills directory has not been generated yet.",
		!scanOK || hasAnyResourceKind(scanReport, model.KindSkill, model.KindCommand),
		"No Claude skills or commands found; no Codex skills directory is required for this migration.",
	)
	verifyStructuredDirWithFallbackIfExpected(
		report,
		"codex-agents-project",
		filepath.Join(project, ".codex", "agents"),
		filepath.Join(codexHome, "agents"),
		"*",
		"Codex agents directory is recognizable.",
		"Global Codex agents directory is recognizable; no project agents directory is required for global Codex setup.",
		"Codex agents directory has not been generated yet.",
		!scanOK || hasAnyResourceKind(scanReport, model.KindAgent),
		"No Claude agents found; no Codex agents directory is required for this migration.",
	)
}

func verifyClaude(report *model.VerifyReport, project string) {
	checkReadableText(report, "claude-instructions-project", filepath.Join(project, "CLAUDE.md"), "CLAUDE.md is readable.", "CLAUDE.md was not found.")
	verifyClaudeSettings(report, project)
	verifyStructuredDir(report, "claude-skills-project", filepath.Join(project, ".claude", "skills"), filepath.Join("*", "SKILL.md"), "Claude skills directory is recognizable.", "Claude skills directory was not found.")
	verifyStructuredDir(report, "claude-agents-project", filepath.Join(project, ".claude", "agents"), "*", "Claude agents directory is recognizable.", "Claude agents directory was not found.")
	verifyStructuredDir(report, "claude-commands-project", filepath.Join(project, ".claude", "commands"), "*", "Claude commands directory is recognizable.", "Claude commands directory was not found.")
}

func hasAnyResourceKind(report model.ScanReport, kinds ...model.ResourceKind) bool {
	wanted := map[model.ResourceKind]bool{}
	for _, kind := range kinds {
		wanted[kind] = true
	}
	for _, resource := range report.Resources {
		if wanted[resource.Kind] {
			return true
		}
	}
	return false
}

func checkReadableText(report *model.VerifyReport, id string, path string, passMessage string, missingMessage string) {
	payload, ok, err := readOptionalFile(path)
	if errors.Is(err, errPathIsDir) {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("%s must be a file.", filepath.Base(path)), path, nil)
		return
	}
	if err != nil {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("read %s: %v", filepath.Base(path), err), path, nil)
		return
	}
	if !ok {
		addCheck(report, id, model.VerifyStatusWarn, missingMessage, path, []model.Warning{{Code: "missing-target", Message: missingMessage}})
		return
	}
	if strings.TrimSpace(string(payload)) == "" {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("%s is empty.", filepath.Base(path)), path, nil)
		return
	}
	addCheck(report, id, model.VerifyStatusPass, passMessage, path, nil)
}

func checkReadableTextWithFallback(report *model.VerifyReport, id string, path string, fallbackPath string, passMessage string, fallbackPassMessage string, missingMessage string) {
	payload, ok, err := readOptionalFile(path)
	if errors.Is(err, errPathIsDir) {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("%s must be a file.", filepath.Base(path)), path, nil)
		return
	}
	if err != nil {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("read %s: %v", filepath.Base(path), err), path, nil)
		return
	}
	if ok {
		if strings.TrimSpace(string(payload)) == "" {
			addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("%s is empty.", filepath.Base(path)), path, nil)
			return
		}
		addCheck(report, id, model.VerifyStatusPass, passMessage, path, nil)
		return
	}

	payload, ok, err = readOptionalFile(fallbackPath)
	if errors.Is(err, errPathIsDir) {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("%s must be a file.", filepath.Base(fallbackPath)), fallbackPath, nil)
		return
	}
	if err != nil {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("read %s: %v", filepath.Base(fallbackPath), err), fallbackPath, nil)
		return
	}
	if !ok {
		addCheck(report, id, model.VerifyStatusWarn, missingMessage, path, []model.Warning{{Code: "missing-target", Message: missingMessage}})
		return
	}
	if strings.TrimSpace(string(payload)) == "" {
		addCheck(report, id, model.VerifyStatusFail, fmt.Sprintf("%s is empty.", filepath.Base(fallbackPath)), fallbackPath, nil)
		return
	}
	addCheck(report, id, model.VerifyStatusPass, fallbackPassMessage, fallbackPath, nil)
}

func verifyCodexConfig(report *model.VerifyReport, path string, fallbackPath string) {
	payload, ok, err := readOptionalFile(path)
	if errors.Is(err, errPathIsDir) {
		addCheck(report, "codex-config-project", model.VerifyStatusFail, ".codex/config.toml must be a file.", path, nil)
		return
	}
	if err != nil {
		addCheck(report, "codex-config-project", model.VerifyStatusFail, fmt.Sprintf("read .codex/config.toml: %v", err), path, nil)
		return
	}
	if ok {
		if err := validateLightTOML(string(payload)); err != nil {
			addCheck(report, "codex-config-project", model.VerifyStatusFail, fmt.Sprintf(".codex/config.toml is structurally invalid: %v", err), path, nil)
			return
		}
		addCheck(report, "codex-config-project", model.VerifyStatusPass, "Codex project config structural syntax passed.", path, nil)
		return
	}

	payload, ok, err = readOptionalFile(fallbackPath)
	if errors.Is(err, errPathIsDir) {
		addCheck(report, "codex-config-project", model.VerifyStatusFail, filepath.Base(fallbackPath)+" must be a file.", fallbackPath, nil)
		return
	}
	if err != nil {
		addCheck(report, "codex-config-project", model.VerifyStatusFail, fmt.Sprintf("read %s: %v", filepath.Base(fallbackPath), err), fallbackPath, nil)
		return
	}
	if !ok {
		message := ".codex/config.toml has not been generated yet."
		addCheck(report, "codex-config-project", model.VerifyStatusWarn, message, path, []model.Warning{{Code: "missing-target", Message: message}})
		return
	}
	if err := validateLightTOML(string(payload)); err != nil {
		addCheck(report, "codex-config-project", model.VerifyStatusFail, fmt.Sprintf("%s is structurally invalid: %v", filepath.Base(fallbackPath), err), fallbackPath, nil)
		return
	}
	addCheck(report, "codex-config-project", model.VerifyStatusPass, "Global Codex config structural syntax passed; no project .codex/config.toml is required for global Codex setup.", fallbackPath, nil)
}

func verifyCodexMCP(report *model.VerifyReport, paths []string) {
	foundConfig := false
	foundMCP := false
	for _, path := range paths {
		payload, ok, err := readOptionalFile(path)
		if err != nil || !ok {
			continue
		}
		foundConfig = true
		if containsMCPEntry(string(payload)) {
			foundMCP = true
		}
	}
	if foundMCP {
		addCheck(report, "codex-mcp-list", model.VerifyStatusPass, "MCP entries are recognizable in Codex config.", "", nil)
		return
	}
	message := "No MCP entries found in Codex config."
	if !foundConfig {
		message = "No Codex config found for MCP inspection."
	}
	addCheck(report, "codex-mcp-list", model.VerifyStatusWarn, message, "", []model.Warning{{Code: "mcp-missing", Message: message}})
}

func verifyClaudeSettings(report *model.VerifyReport, project string) {
	paths := []string{filepath.Join(project, ".claude", "settings.json"), filepath.Join(project, ".claude", "settings.local.json")}
	seen := false
	for _, path := range paths {
		payload, ok, err := readOptionalFile(path)
		if errors.Is(err, errPathIsDir) {
			addCheck(report, "claude-settings-project", model.VerifyStatusFail, filepath.Base(path)+" must be a file.", path, nil)
			return
		}
		if err != nil {
			addCheck(report, "claude-settings-project", model.VerifyStatusFail, fmt.Sprintf("read %s: %v", filepath.Base(path), err), path, nil)
			return
		}
		if !ok {
			continue
		}
		seen = true
		var decoded any
		if err := json.Unmarshal(payload, &decoded); err != nil {
			addCheck(report, "claude-settings-project", model.VerifyStatusFail, fmt.Sprintf("%s is invalid JSON: %v", filepath.Base(path), err), path, nil)
			return
		}
	}
	if !seen {
		message := "No project Claude settings files found."
		addCheck(report, "claude-settings-project", model.VerifyStatusWarn, message, "", []model.Warning{{Code: "missing-settings", Message: message}})
		return
	}
	addCheck(report, "claude-settings-project", model.VerifyStatusPass, "Project Claude settings JSON passed.", "", nil)
}

type structuredDirCheck struct {
	status   model.VerifyStatus
	message  string
	path     string
	warnings []model.Warning
	missing  bool
}

func verifyStructuredDir(report *model.VerifyReport, id string, root string, pattern string, passMessage string, missingMessage string) {
	addStructuredDirCheck(report, id, inspectStructuredDir(root, pattern, passMessage, missingMessage))
}

func verifyStructuredDirWithFallback(report *model.VerifyReport, id string, root string, fallbackRoot string, pattern string, passMessage string, fallbackPassMessage string, missingMessage string) {
	check := inspectStructuredDir(root, pattern, passMessage, missingMessage)
	if check.missing {
		fallbackCheck := inspectStructuredDir(fallbackRoot, pattern, fallbackPassMessage, missingMessage)
		if !fallbackCheck.missing {
			check = fallbackCheck
		}
	}
	addStructuredDirCheck(report, id, check)
}

func verifyStructuredDirWithFallbackIfExpected(report *model.VerifyReport, id string, root string, fallbackRoot string, pattern string, passMessage string, fallbackPassMessage string, missingMessage string, expected bool, absentPassMessage string) {
	if expected {
		verifyStructuredDirWithFallback(report, id, root, fallbackRoot, pattern, passMessage, fallbackPassMessage, missingMessage)
		return
	}
	check := inspectStructuredDir(root, pattern, passMessage, missingMessage)
	if check.missing {
		fallbackCheck := inspectStructuredDir(fallbackRoot, pattern, fallbackPassMessage, missingMessage)
		if fallbackCheck.missing {
			addCheck(report, id, model.VerifyStatusPass, absentPassMessage, "", nil)
			return
		}
		check = fallbackCheck
	}
	addStructuredDirCheck(report, id, check)
}

func inspectStructuredDir(root string, pattern string, passMessage string, missingMessage string) structuredDirCheck {
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return structuredDirCheck{status: model.VerifyStatusWarn, message: missingMessage, path: root, warnings: []model.Warning{{Code: "missing-target", Message: missingMessage}}, missing: true}
	}
	if err != nil {
		return structuredDirCheck{status: model.VerifyStatusFail, message: fmt.Sprintf("inspect %s: %v", filepath.Base(root), err), path: root}
	}
	if !info.IsDir() {
		return structuredDirCheck{status: model.VerifyStatusFail, message: fmt.Sprintf("%s must be a directory.", root), path: root}
	}
	matches, err := filepath.Glob(filepath.Join(root, pattern))
	if err != nil {
		return structuredDirCheck{status: model.VerifyStatusFail, message: fmt.Sprintf("inspect %s: %v", root, err), path: root}
	}
	if len(matches) == 0 {
		message := fmt.Sprintf("%s does not contain recognizable entries.", root)
		return structuredDirCheck{status: model.VerifyStatusWarn, message: message, path: root, warnings: []model.Warning{{Code: "empty-target", Message: message}}}
	}
	return structuredDirCheck{status: model.VerifyStatusPass, message: passMessage, path: root}
}

func addStructuredDirCheck(report *model.VerifyReport, id string, check structuredDirCheck) {
	addCheck(report, id, check.status, check.message, check.path, check.warnings)
}

func verifySyncState(report *model.VerifyReport, project string) {
	layout, err := workspace.New(project)
	if err != nil {
		addCheck(report, "sync-conflicts", model.VerifyStatusFail, fmt.Sprintf("inspect sync state: %v", err), "", nil)
		return
	}
	var state model.SyncStateReport
	if err := layout.LoadSyncState(&state); err != nil {
		if errors.Is(err, workspace.ErrNotFound) {
			message := "No sync state found; run agent-canon sync claude codex before verifying migration conflicts."
			addCheck(report, "sync-conflicts", model.VerifyStatusWarn, message, layout.SyncState, []model.Warning{{Code: "missing-sync-state", Message: message}})
			return
		}
		addCheck(report, "sync-conflicts", model.VerifyStatusFail, fmt.Sprintf("read sync state: %v", err), layout.SyncState, nil)
		return
	}
	open := 0
	for _, conflict := range state.Conflicts {
		if conflict.Status != model.ConflictStatusResolved {
			open++
		}
	}
	if open > 0 {
		addCheck(report, "sync-conflicts", model.VerifyStatusFail, fmt.Sprintf("%d open conflicts remain; run agent-canon conflicts and resolve them first.", open), layout.SyncState, nil)
		return
	}
	addCheck(report, "sync-conflicts", model.VerifyStatusPass, "No open sync conflicts found.", layout.SyncState, nil)
}

func verifyScanner(report *model.VerifyReport, scanReport model.ScanReport, err error) {
	if err != nil {
		var parseErr scanner.ParseError
		if errors.As(err, &parseErr) {
			addCheck(report, "scanner", model.VerifyStatusFail, fmt.Sprintf("Scanner parse failed: %v", err), parseErr.Path, nil)
			return
		}
		addCheck(report, "scanner", model.VerifyStatusFail, fmt.Sprintf("Scanner failed: %v", err), "", nil)
		return
	}
	for _, warning := range scanReport.Warnings {
		report.Warnings = append(report.Warnings, redactWarning(warning))
	}
	addCheck(report, "scanner", model.VerifyStatusPass, "Scanner completed without parse errors.", "", nil)
}

func validateLightTOML(contents string) error {
	for lineNumber, line := range strings.Split(contents, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			if !strings.HasSuffix(trimmed, "]") || strings.Count(trimmed, "[") != strings.Count(trimmed, "]") {
				return fmt.Errorf("invalid section header on line %d", lineNumber+1)
			}
			continue
		}
		if strings.Contains(trimmed, "=") {
			continue
		}
		return fmt.Errorf("invalid statement on line %d", lineNumber+1)
	}
	return nil
}

func containsMCPEntry(contents string) bool {
	lower := strings.ToLower(contents)
	return strings.Contains(lower, "mcp_servers") || strings.Contains(lower, "mcpservers") || strings.Contains(lower, "mcp-server")
}

var errPathIsDir = errors.New("path is a directory")

func readOptionalFile(path string) ([]byte, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() {
		return nil, true, errPathIsDir
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, true, err
	}
	return payload, true, nil
}

func addCheck(report *model.VerifyReport, id string, status model.VerifyStatus, message string, path string, warnings []model.Warning) {
	message, _ = security.RedactContent(message)
	redactedWarnings := make([]model.Warning, 0, len(warnings))
	for _, warning := range warnings {
		redactedWarnings = append(redactedWarnings, redactWarning(warning))
	}
	report.Checks = append(report.Checks, model.VerifyCheck{ID: id, Target: report.Target, Status: status, Message: message, Path: path, Warnings: redactedWarnings})
}

func redactWarning(warning model.Warning) model.Warning {
	message, _ := security.RedactContent(warning.Message)
	warning.Message = message
	return warning
}

func summarize(checks []model.VerifyCheck) model.VerifySummary {
	var summary model.VerifySummary
	for _, check := range checks {
		switch check.Status {
		case model.VerifyStatusPass:
			summary.Pass++
		case model.VerifyStatusWarn:
			summary.Warn++
		case model.VerifyStatusFail:
			summary.Fail++
		}
	}
	return summary
}

func absClean(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}
