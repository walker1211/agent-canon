package apply

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/codexpath"
	"github.com/zhangyoujun/agent-canon/internal/configmerge"
	"github.com/zhangyoujun/agent-canon/internal/exporter"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

type CodexPlanInput struct {
	Scan           model.ScanReport
	Plan           model.PlanReport
	IncludeGlobal  bool
	Filters        ApplyFilters
	MergeConfig    bool
	MCPResolutions []configmerge.CodexMCPResolution
}

type CodexPlan struct {
	Changes  []FileChange
	Warnings []model.Warning
}

type ClaudePlanInput struct {
	Scan          model.ScanReport
	Plan          model.PlanReport
	IncludeGlobal bool
	Filters       ApplyFilters
}

type ClaudePlan struct {
	Changes  []FileChange
	Warnings []model.Warning
}

type FileChange struct {
	model.ApplyFileChange
	PreviewPath string `json:"previewPath,omitempty"`
	Contents    []byte `json:"-"`
}

func BuildCodexPlan(input CodexPlanInput) (CodexPlan, error) {
	var out CodexPlan
	out.Warnings = appendWarningsUnique(out.Warnings, input.Scan.Warnings...)
	out.Warnings = appendWarningsUnique(out.Warnings, input.Plan.Warnings...)
	roots := FilterRoots{Project: input.Scan.Project, Home: input.Scan.CodexHome}

	projectScan := scanForScope(input.Scan, model.ScopeProject)
	if len(projectScan.Resources) > 0 {
		changes, warnings, err := changesForScope(projectScan, planForScan(input.Plan, projectScan), input.Scan.Project, model.ScopeProject, exporter.BuildCodexPreview, targetPath, input.Filters, roots, changeOptions{MergeCodexConfig: input.MergeConfig, CodexMCPResolutions: input.MCPResolutions})
		if err != nil {
			return CodexPlan{}, err
		}
		out.Changes = append(out.Changes, changes...)
		out.Warnings = append(out.Warnings, warnings...)
	}

	globalScan := scanForScope(input.Scan, model.ScopeGlobal)
	if len(globalScan.Resources) > 0 {
		if input.IncludeGlobal {
			changes, warnings, err := changesForScope(globalScan, planForScan(input.Plan, globalScan), input.Scan.CodexHome, model.ScopeGlobal, exporter.BuildCodexPreview, targetPath, input.Filters, roots, changeOptions{MergeCodexConfig: input.MergeConfig, CodexMCPResolutions: input.MCPResolutions})
			if err != nil {
				return CodexPlan{}, err
			}
			out.Changes = append(out.Changes, changes...)
			out.Warnings = append(out.Warnings, warnings...)
		} else {
			out.Warnings = append(out.Warnings, model.Warning{Code: "global-skipped", Message: "global Codex targets were skipped; rerun with --global to include --codex-home writes"})
		}
	}

	sort.SliceStable(out.Changes, func(i, j int) bool {
		return out.Changes[i].Path < out.Changes[j].Path
	})
	return out, nil
}

func BuildClaudePlan(input ClaudePlanInput) (ClaudePlan, error) {
	var out ClaudePlan
	out.Warnings = appendWarningsUnique(out.Warnings, input.Scan.Warnings...)
	out.Warnings = appendWarningsUnique(out.Warnings, input.Plan.Warnings...)
	roots := FilterRoots{Project: input.Scan.Project, Home: input.Scan.ClaudeHome}

	projectScan := scanForScope(input.Scan, model.ScopeProject)
	if len(projectScan.Resources) > 0 {
		changes, warnings, err := changesForScope(projectScan, planForScan(input.Plan, projectScan), input.Scan.Project, model.ScopeProject, exporter.BuildClaudePreview, claudeTargetPath, input.Filters, roots, changeOptions{})
		if err != nil {
			return ClaudePlan{}, err
		}
		out.Changes = append(out.Changes, changes...)
		out.Warnings = append(out.Warnings, warnings...)
	}

	globalScan := scanForScope(input.Scan, model.ScopeGlobal)
	if len(globalScan.Resources) > 0 {
		if input.IncludeGlobal {
			changes, warnings, err := changesForScope(globalScan, planForScan(input.Plan, globalScan), input.Scan.ClaudeHome, model.ScopeGlobal, exporter.BuildClaudePreview, claudeTargetPath, input.Filters, roots, changeOptions{})
			if err != nil {
				return ClaudePlan{}, err
			}
			out.Changes = append(out.Changes, changes...)
			out.Warnings = append(out.Warnings, warnings...)
		} else {
			out.Warnings = append(out.Warnings, model.Warning{Code: "global-skipped", Message: "global Claude targets were skipped; rerun with --global to include --claude-home writes"})
		}
	}

	sort.SliceStable(out.Changes, func(i, j int) bool {
		return out.Changes[i].Path < out.Changes[j].Path
	})
	return out, nil
}

func appendWarningsUnique(existing []model.Warning, additions ...model.Warning) []model.Warning {
	out := append([]model.Warning{}, existing...)
	for _, addition := range additions {
		seen := false
		for _, warning := range out {
			if warning.Code == addition.Code && warning.Message == addition.Message {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, addition)
		}
	}
	return out
}

type previewBuilder func(model.ScanReport, model.PlanReport) (exporter.CodexPreview, error)
type targetMapper func(string, model.Scope, string) string

type changeOptions struct {
	MergeCodexConfig    bool
	CodexMCPResolutions []configmerge.CodexMCPResolution
}

func changesForScope(scan model.ScanReport, plan model.PlanReport, root string, scope model.Scope, buildPreview previewBuilder, mapTarget targetMapper, filters ApplyFilters, roots FilterRoots, options changeOptions) ([]FileChange, []model.Warning, error) {
	preview, err := buildPreview(scan, plan)
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]FileChange, 0, len(preview.Files))
	for _, file := range preview.Files {
		if file.Path == "migration-report.md" {
			continue
		}
		candidates = append(candidates, FileChange{
			ApplyFileChange: model.ApplyFileChange{
				Path:  mapTarget(root, scope, file.Path),
				Scope: scope,
			},
			PreviewPath: file.Path,
			Contents:    file.Contents,
		})
	}

	candidates = FilterChanges(candidates, filters, roots)
	changes := make([]FileChange, 0, len(candidates))
	var warnings []model.Warning
	for _, candidate := range candidates {
		if options.MergeCodexConfig && isCodexConfigPreview(candidate.PreviewPath) {
			merged, include, mergeWarnings, err := mergeCodexConfigCandidate(scan, candidate, options.CodexMCPResolutions)
			if err != nil {
				return nil, nil, err
			}
			warnings = append(warnings, mergeWarnings...)
			if !include {
				continue
			}
			candidate = merged
		} else {
			skip, warning, err := shouldSkipReviewOnlyConfigOverwrite(candidate)
			if err != nil {
				return nil, nil, err
			}
			if skip {
				warnings = append(warnings, warning)
				continue
			}
		}
		change, err := buildFileChange(candidate.Path, candidate.Scope, candidate.PreviewPath, candidate.Contents)
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, change)
	}
	return changes, warnings, nil
}

func mergeCodexConfigCandidate(scan model.ScanReport, candidate FileChange, resolutions []configmerge.CodexMCPResolution) (FileChange, bool, []model.Warning, error) {
	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scan, TargetPath: candidate.Path, Resolutions: resolutions})
	if err != nil {
		return FileChange{}, false, nil, err
	}
	if result.MergeableServers == 0 && !result.Existing {
		return FileChange{}, false, result.Warnings, nil
	}
	candidate.Contents = result.Contents
	return candidate, true, result.Warnings, nil
}

func isCodexConfigPreview(previewPath string) bool {
	return filepath.ToSlash(previewPath) == ".codex/config.toml"
}

func shouldSkipReviewOnlyConfigOverwrite(change FileChange) (bool, model.Warning, error) {
	if !isReviewOnlyConfigPreview(change.PreviewPath, change.Contents) {
		return false, model.Warning{}, nil
	}
	current, err := os.ReadFile(change.Path)
	if os.IsNotExist(err) {
		return false, model.Warning{}, nil
	}
	if err != nil {
		return false, model.Warning{}, fmt.Errorf("read apply target %s: %w", change.Path, err)
	}
	if !hasUserConfigContent(change.PreviewPath, current) {
		return false, model.Warning{}, nil
	}
	warning := model.Warning{
		Code:    "review-only-config-skipped",
		Message: fmt.Sprintf("%s already contains user configuration; review generated %s manually instead of applying the review-only preview", change.Path, change.PreviewPath),
	}
	return true, warning, nil
}

func isReviewOnlyConfigPreview(previewPath string, contents []byte) bool {
	switch filepath.ToSlash(previewPath) {
	case ".codex/config.toml":
		return bytes.Contains(contents, []byte("# Codex configuration preview")) && !hasTOMLDataLines(contents)
	case ".claude/settings.json":
		return bytes.Contains(contents, []byte(`"agentCanonPreview"`)) && bytes.Contains(contents, []byte(`"noRunnableEdits": true`))
	default:
		return false
	}
}

func hasUserConfigContent(previewPath string, contents []byte) bool {
	switch filepath.ToSlash(previewPath) {
	case ".codex/config.toml":
		return hasTOMLDataLines(contents)
	case ".claude/settings.json":
		return hasClaudeSettingsUserConfig(contents)
	default:
		return false
	}
}

func hasClaudeSettingsUserConfig(contents []byte) bool {
	trimmed := strings.TrimSpace(string(contents))
	if trimmed == "" {
		return false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &object); err != nil {
		return true
	}
	if len(object) == 0 {
		return false
	}
	_, onlyPreview := object["agentCanonPreview"]
	return len(object) != 1 || !onlyPreview
}

func hasTOMLDataLines(contents []byte) bool {
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return true
		}
	}
	return false
}

func targetPath(root string, scope model.Scope, previewPath string) string {
	clean := filepath.FromSlash(previewPath)
	if scope != model.ScopeGlobal {
		return filepath.Join(root, clean)
	}
	switch clean {
	case filepath.Join(".codex", "config.toml"):
		return filepath.Join(root, "config.toml")
	case "AGENTS.md":
		return filepath.Join(root, "AGENTS.md")
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".agents", "skills")); ok {
		return filepath.Join(codexpath.UserSkillsRoot(root), rest)
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".codex", "agents")); ok {
		return filepath.Join(root, "agents", rest)
	}
	return filepath.Join(root, clean)
}

func claudeTargetPath(root string, scope model.Scope, previewPath string) string {
	clean := filepath.FromSlash(previewPath)
	if scope != model.ScopeGlobal {
		return filepath.Join(root, clean)
	}
	switch clean {
	case filepath.Join(".claude", "settings.json"):
		return filepath.Join(root, "settings.json")
	case "CLAUDE.md":
		return filepath.Join(root, "CLAUDE.md")
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".claude", "skills")); ok {
		return filepath.Join(root, "skills", rest)
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".claude", "commands")); ok {
		return filepath.Join(root, "commands", rest)
	}
	if rest, ok := trimPathPrefix(clean, filepath.Join(".claude", "agents")); ok {
		return filepath.Join(root, "agents", rest)
	}
	return filepath.Join(root, clean)
}

func trimPathPrefix(path string, prefix string) (string, bool) {
	rel, err := filepath.Rel(prefix, path)
	if err != nil || rel == "." || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(os.PathSeparator) {
		return "", false
	}
	return rel, true
}

func buildFileChange(path string, scope model.Scope, previewPath string, contents []byte) (FileChange, error) {
	afterHash := hashBytes(contents)
	change := FileChange{
		ApplyFileChange: model.ApplyFileChange{
			Path:      path,
			Scope:     scope,
			Action:    model.ApplyActionCreate,
			AfterHash: afterHash,
		},
		PreviewPath: previewPath,
		Contents:    append([]byte(nil), contents...),
	}

	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return change, nil
	}
	if err != nil {
		return FileChange{}, fmt.Errorf("read apply target %s: %w", path, err)
	}
	change.BeforeHash = hashBytes(current)
	if change.BeforeHash == afterHash {
		change.Action = model.ApplyActionNoop
		return change, nil
	}
	change.Action = model.ApplyActionModify
	return change, nil
}

func hashBytes(contents []byte) string {
	sum := sha256.Sum256(contents)
	return fmt.Sprintf("sha256:%x", sum)
}

func scanForScope(scan model.ScanReport, scope model.Scope) model.ScanReport {
	out := scan
	out.Resources = nil
	for _, resource := range scan.Resources {
		if resource.Scope == scope {
			out.Resources = append(out.Resources, resource)
		}
	}
	return out
}

func planForScan(plan model.PlanReport, scan model.ScanReport) model.PlanReport {
	ids := make(map[string]bool, len(scan.Resources))
	for _, resource := range scan.Resources {
		ids[resource.ID] = true
	}
	out := plan
	out.Operations = nil
	for _, operation := range plan.Operations {
		if ids[operation.ResourceID] {
			out.Operations = append(out.Operations, operation)
		}
	}
	return out
}
