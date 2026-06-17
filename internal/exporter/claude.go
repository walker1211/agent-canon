package exporter

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
	"github.com/zhangyoujun/agent-canon/internal/skillbundle"
)

func BuildClaudePreview(scan model.ScanReport, plan model.PlanReport) (CodexPreview, error) {
	builder := claudeBuilder{scan: scan, plan: plan}
	warnings := append([]model.Warning{}, scan.Warnings...)
	warnings = append(warnings, plan.Warnings...)

	files, err := builder.files()
	if err != nil {
		return CodexPreview{}, err
	}
	return CodexPreview{Files: files, Warnings: warnings}, nil
}

type claudeBuilder struct {
	scan model.ScanReport
	plan model.PlanReport
}

func (b claudeBuilder) files() ([]PreviewFile, error) {
	files := []PreviewFile{}

	claude, err := b.claudeMarkdown()
	if err != nil {
		return nil, err
	}
	files = append(files, PreviewFile{Path: "CLAUDE.md", Contents: claude})
	files = append(files, PreviewFile{Path: ".claude/settings.json", Contents: b.settingsJSON()})

	for _, resource := range b.scan.Resources {
		if !canGenerateResourcePreview(resource.Status) {
			continue
		}
		switch resource.Kind {
		case model.KindSkill:
			skillFiles, err := b.skillPreview(resource)
			if err != nil {
				return nil, err
			}
			files = append(files, skillFiles...)
		case model.KindCommand:
			file, err := b.commandPreview(resource)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		case model.KindAgent:
			file, err := b.agentPreview(resource)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
	}

	if err := ensureUniquePreviewPaths(files); err != nil {
		return nil, err
	}
	reportFiles := append([]PreviewFile{}, files...)
	reportFiles = append(reportFiles, PreviewFile{Path: "migration-report.md"})
	files = append(files, PreviewFile{Path: "migration-report.md", Contents: b.migrationReport(reportFiles)})
	if err := ensureUniquePreviewPaths(files); err != nil {
		return nil, err
	}
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (b claudeBuilder) claudeMarkdown() ([]byte, error) {
	var buf bytes.Buffer
	writeLine(&buf, "# CLAUDE.md preview")
	writeLine(&buf, "")
	writeLine(&buf, "Generated preview for Claude. Review before copying into a real configuration.")

	for _, resource := range b.scan.Resources {
		if resource.Status != model.StatusCompatible {
			continue
		}
		if resource.Kind != model.KindInstruction && resource.Kind != model.KindRule {
			continue
		}
		contents, err := readSource(resource)
		if err != nil {
			return nil, err
		}
		writeLine(&buf, "")
		writeLine(&buf, "## %s", resource.ID)
		writeLine(&buf, "")
		writeLine(&buf, "- kind: %s", resource.Kind)
		writeLine(&buf, "- scope: %s", resource.Scope)
		writeLine(&buf, "- source file: %s", filepath.Base(resource.SourcePath))
		writeLine(&buf, "")
		buf.Write(bytes.TrimSpace(redactSourceLines(contents)))
		writeLine(&buf, "")
	}
	return buf.Bytes(), nil
}

func (b claudeBuilder) settingsJSON() []byte {
	settings := claudeSettingsPreview{
		AgentCanonPreview: claudeSettingsBody{
			GeneratedBy:     "agent-canon",
			ReviewRequired:  true,
			MCPServers:      b.mcpServerPreviews(),
			Warnings:        redactedWarnings(append(append([]model.Warning{}, b.scan.Warnings...), b.plan.Warnings...)),
			NoRunnableEdits: true,
		},
	}
	contents, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(contents, '\n')
}

func (b claudeBuilder) mcpServerPreviews() []claudeMCPPreview {
	var servers []claudeMCPPreview
	for _, resource := range filterResources(b.scan.Resources, model.KindMCPServer) {
		servers = append(servers, claudeMCPPreview{
			ResourceID:     resource.ID,
			Status:         resource.Status,
			ReviewRequired: true,
			Warnings:       redactedWarnings(resource.Warnings),
		})
	}
	return servers
}

type claudeSettingsPreview struct {
	AgentCanonPreview claudeSettingsBody `json:"agentCanonPreview"`
}

type claudeSettingsBody struct {
	GeneratedBy     string             `json:"generatedBy"`
	ReviewRequired  bool               `json:"reviewRequired"`
	NoRunnableEdits bool               `json:"noRunnableEdits"`
	MCPServers      []claudeMCPPreview `json:"mcpServers,omitempty"`
	Warnings        []model.Warning    `json:"warnings,omitempty"`
}

type claudeMCPPreview struct {
	ResourceID     string          `json:"resourceId"`
	Status         model.Status    `json:"status"`
	ReviewRequired bool            `json:"reviewRequired"`
	Warnings       []model.Warning `json:"warnings,omitempty"`
}

func (b claudeBuilder) skillPreview(resource model.Resource) ([]PreviewFile, error) {
	files, err := skillbundle.Files(resource.SourcePath)
	if err != nil {
		return nil, err
	}
	out := make([]PreviewFile, 0, len(files))
	for _, file := range files {
		contents := redactSourceLines(file.Contents)
		if file.RelativePath == "SKILL.md" {
			var buf bytes.Buffer
			writeLine(&buf, "<!-- Generated Claude skill candidate from %s. Review required. -->", resource.ID)
			writeLine(&buf, "")
			buf.Write(bytes.TrimSpace(contents))
			writeLine(&buf, "")
			contents = buf.Bytes()
		}
		out = append(out, PreviewFile{
			Path:     filepath.ToSlash(filepath.Join(".claude", "skills", safeName(resource), filepath.FromSlash(file.RelativePath))),
			Contents: contents,
		})
	}
	return out, nil
}

func (b claudeBuilder) commandPreview(resource model.Resource) (PreviewFile, error) {
	contents, err := readOptionalSource(resource)
	if err != nil {
		return PreviewFile{}, err
	}
	var buf bytes.Buffer
	writeLine(&buf, "<!-- Generated Claude command candidate from %s. Review required. -->", resource.ID)
	writeLine(&buf, "")
	if len(contents) == 0 {
		writeLine(&buf, "Source command content was not a regular file; review manually.")
	} else {
		buf.Write(bytes.TrimSpace(redactSourceLines(contents)))
		writeLine(&buf, "")
	}
	return PreviewFile{Path: filepath.ToSlash(filepath.Join(".claude", "commands", safeName(resource)+".md")), Contents: buf.Bytes()}, nil
}

func (b claudeBuilder) agentPreview(resource model.Resource) (PreviewFile, error) {
	contents, err := readOptionalSource(resource)
	if err != nil {
		return PreviewFile{}, err
	}
	var buf bytes.Buffer
	writeLine(&buf, "<!-- Generated Claude agent candidate from %s. Review required. -->", resource.ID)
	writeLine(&buf, "")
	if len(contents) == 0 {
		writeLine(&buf, "Source agent content was not a regular file; review manually.")
	} else {
		buf.Write(bytes.TrimSpace(redactSourceLines(contents)))
		writeLine(&buf, "")
	}
	return PreviewFile{Path: filepath.ToSlash(filepath.Join(".claude", "agents", safeName(resource)+".md")), Contents: buf.Bytes()}, nil
}

func (b claudeBuilder) migrationReport(generated []PreviewFile) []byte {
	var buf bytes.Buffer
	writeLine(&buf, "# Migration report")
	writeLine(&buf, "")
	writeLine(&buf, "no real Claude configuration was written. This is a preview only.")

	writeLine(&buf, "")
	writeLine(&buf, "## generated files")
	sort.SliceStable(generated, func(i, j int) bool {
		return generated[i].Path < generated[j].Path
	})
	for _, file := range generated {
		writeLine(&buf, "- `%s`", file.Path)
	}

	writeResourceSection(&buf, "review-required resources", b.reviewRequiredResources())
	writeResourceSection(&buf, "skipped unsupported resources", statusResources(b.scan.Resources, model.StatusUnsupported))
	writeResourceSection(&buf, "dangerous resources/warnings", statusResources(b.scan.Resources, model.StatusDangerous))

	writeLine(&buf, "")
	writeLine(&buf, "## top-level warnings")
	if len(b.scan.Warnings) == 0 && len(b.plan.Warnings) == 0 {
		writeLine(&buf, "- none")
	} else {
		writeWarnings(&buf, b.scan.Warnings)
		writeWarnings(&buf, b.plan.Warnings)
	}
	return buf.Bytes()
}

func (b claudeBuilder) reviewRequiredResources() []model.Resource {
	byID := map[string]model.Resource{}
	for _, resource := range b.scan.Resources {
		byID[resource.ID] = resource
	}
	var resources []model.Resource
	for _, operation := range b.plan.Operations {
		if !operation.RequiresReview {
			continue
		}
		if resource, ok := byID[operation.ResourceID]; ok {
			resources = append(resources, resource)
		}
	}
	return resources
}

func redactedWarnings(warnings []model.Warning) []model.Warning {
	redacted := make([]model.Warning, 0, len(warnings))
	for _, warning := range warnings {
		message := scrubWarningMessage(warning.Message)
		message, _ = security.RedactContent(message)
		if warning.Code == "secret-redacted" && !strings.Contains(message, security.RedactedValue) {
			message = strings.TrimSpace(message + " " + security.RedactedValue)
		}
		redacted = append(redacted, model.Warning{Code: warning.Code, Message: message})
	}
	return redacted
}
