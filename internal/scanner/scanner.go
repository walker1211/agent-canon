package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

type Options struct {
	Project       string
	ClaudeHome    string
	CodexHome     string
	IncludeMemory bool
}

func Scan(opts Options) (model.ScanReport, error) {
	project, err := requiredDir(opts.Project, "project")
	if err != nil {
		return model.ScanReport{}, err
	}
	claudeHome, err := requiredDir(opts.ClaudeHome, "claude home")
	if err != nil {
		return model.ScanReport{}, err
	}
	codexHome, err := requiredDir(opts.CodexHome, "codex home")
	if err != nil {
		return model.ScanReport{}, err
	}

	report := model.ScanReport{
		SchemaVersion: model.ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Project:       project,
		ClaudeHome:    claudeHome,
		CodexHome:     codexHome,
		Resources:     []model.Resource{},
		Warnings:      []model.Warning{},
	}

	codexTargets := detectCodexTargets(project, codexHome)
	report.Warnings = append(report.Warnings, codexTargets.Warnings...)
	report.Resources = append(report.Resources, scanClaudeHome(claudeHome, codexHome, codexTargets)...)
	report.Resources = append(report.Resources, scanProject(project, codexTargets)...)
	if opts.IncludeMemory {
		report.Resources = append(report.Resources, scanMemoryItems(claudeHome)...)
	}

	settingsResources, settingsWarnings, err := scanClaudeSettings(project, claudeHome, codexHome)
	if err != nil {
		return model.ScanReport{}, err
	}
	report.Resources = append(report.Resources, settingsResources...)
	report.Warnings = append(report.Warnings, settingsWarnings...)
	annotateCodexTargetWarnings(report.Resources, codexTargets)

	skipConfig, err := loadSkipConfig(project)
	if err != nil {
		return model.ScanReport{}, err
	}
	applySkipConfig(&report, skipConfig)

	sort.Slice(report.Resources, func(i, j int) bool {
		if report.Resources[i].ID == report.Resources[j].ID {
			return report.Resources[i].SourcePath < report.Resources[j].SourcePath
		}
		return report.Resources[i].ID < report.Resources[j].ID
	})

	report.Summary = summarize(report.Resources)
	return report, nil
}

func requiredDir(path string, label string) (string, error) {
	clean, err := absClean(path)
	if err != nil {
		return "", fmt.Errorf("%s path %q: %w", label, path, err)
	}
	info, err := os.Stat(clean)
	if err != nil {
		return "", fmt.Errorf("%s path %q is not accessible: %w", label, clean, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s path %q is not a directory", label, clean)
	}
	return clean, nil
}

func absClean(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func existingFile(path string) (string, bool) {
	clean, err := absClean(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(clean)
	if err != nil || info.IsDir() {
		return "", false
	}
	return clean, true
}

func existingDir(path string) (string, bool) {
	clean, err := absClean(path)
	if err != nil {
		return "", false
	}
	info, err := os.Stat(clean)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return clean, true
}

func summarize(resources []model.Resource) model.ScanSummary {
	var summary model.ScanSummary
	for _, resource := range resources {
		switch resource.Status {
		case model.StatusCompatible:
			summary.Compatible++
		case model.StatusPartial:
			summary.Partial++
		case model.StatusUnsupported:
			summary.Unsupported++
		case model.StatusDangerous:
			summary.Dangerous++
		}
	}
	return summary
}

func newResource(id string, kind model.ResourceKind, scope model.Scope, sourcePath string, targetPathHint string, status model.Status, strategy string) model.Resource {
	return model.Resource{
		ID:             id,
		Kind:           kind,
		Scope:          scope,
		SourceTool:     "claude",
		SourcePath:     sourcePath,
		TargetTool:     "codex",
		TargetPathHint: targetPathHint,
		Status:         status,
		Strategy:       strategy,
		Warnings:       []model.Warning{},
	}
}

func slug(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	if ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}
