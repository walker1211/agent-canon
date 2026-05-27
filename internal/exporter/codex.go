package exporter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

type PreviewFile struct {
	Path     string
	Contents []byte
}

type CodexPreview struct {
	Files    []PreviewFile
	Warnings []model.Warning
}

var inlineSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{8,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func BuildCodexPreview(scan model.ScanReport, plan model.PlanReport) (CodexPreview, error) {
	builder := codexBuilder{scan: scan, plan: plan}
	warnings := append([]model.Warning{}, scan.Warnings...)
	warnings = append(warnings, plan.Warnings...)

	files, err := builder.files()
	if err != nil {
		return CodexPreview{}, err
	}
	return CodexPreview{Files: files, Warnings: warnings}, nil
}

type codexBuilder struct {
	scan model.ScanReport
	plan model.PlanReport
}

func (b codexBuilder) files() ([]PreviewFile, error) {
	files := []PreviewFile{}

	agents, err := b.agentsMarkdown()
	if err != nil {
		return nil, err
	}
	files = append(files, PreviewFile{Path: "AGENTS.md", Contents: agents})
	files = append(files, PreviewFile{Path: ".codex/config.toml", Contents: b.configTOML()})

	for _, resource := range b.scan.Resources {
		if !canGenerateResourcePreview(resource.Status) {
			continue
		}
		switch resource.Kind {
		case model.KindSkill:
			file, err := b.skillPreview(resource)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		case model.KindCommand:
			file, err := b.commandPreview(resource)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		case model.KindAgent:
			files = append(files, b.agentPreview(resource))
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

func ensureUniquePreviewPaths(files []PreviewFile) error {
	seen := map[string]bool{}
	for _, file := range files {
		if seen[file.Path] {
			return fmt.Errorf("duplicate preview path %q", file.Path)
		}
		seen[file.Path] = true
	}
	return nil
}

func canGenerateResourcePreview(status model.Status) bool {
	return status == model.StatusCompatible || status == model.StatusPartial
}

func (b codexBuilder) agentsMarkdown() ([]byte, error) {
	var buf bytes.Buffer
	writeLine(&buf, "# AGENTS.md preview")
	writeLine(&buf, "")
	writeLine(&buf, "Generated preview for Codex. Review before copying into a real configuration.")

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

func (b codexBuilder) skillPreview(resource model.Resource) (PreviewFile, error) {
	contents, err := readSource(resource)
	if err != nil {
		return PreviewFile{}, err
	}
	var buf bytes.Buffer
	writeLine(&buf, "<!-- Generated Codex skill candidate from %s. Partial conversion; review required. -->", resource.ID)
	writeLine(&buf, "")
	buf.Write(bytes.TrimSpace(redactSourceLines(contents)))
	writeLine(&buf, "")
	return PreviewFile{Path: filepath.ToSlash(filepath.Join(".agents", "skills", safeName(resource), "SKILL.md")), Contents: buf.Bytes()}, nil
}

func (b codexBuilder) commandPreview(resource model.Resource) (PreviewFile, error) {
	contents, err := readOptionalSource(resource)
	if err != nil {
		return PreviewFile{}, err
	}
	var buf bytes.Buffer
	writeLine(&buf, "<!-- Generated Codex skill candidate from %s. Lossy command-to-skill conversion; review required. -->", resource.ID)
	writeLine(&buf, "")
	if len(contents) == 0 {
		writeLine(&buf, "Source command content was not a regular file; review manually.")
	} else {
		buf.Write(bytes.TrimSpace(redactSourceLines(contents)))
		writeLine(&buf, "")
	}
	return PreviewFile{Path: filepath.ToSlash(filepath.Join(".agents", "skills", safeName(resource), "SKILL.md")), Contents: buf.Bytes()}, nil
}

func (b codexBuilder) agentPreview(resource model.Resource) PreviewFile {
	var buf bytes.Buffer
	writeLine(&buf, "# Generated Codex agent candidate from %s", resource.ID)
	writeLine(&buf, "# Review required: Claude agent configuration does not have schema parity with Codex.")
	writeLine(&buf, "# Source basename: %s", filepath.Base(resource.SourcePath))
	writeLine(&buf, "")
	writeLine(&buf, "# TODO: manually translate supported agent behavior into the current Codex schema.")
	writeLine(&buf, "# kind = %q", resource.Kind)
	writeLine(&buf, "# scope = %q", resource.Scope)
	return PreviewFile{Path: filepath.ToSlash(filepath.Join(".codex", "agents", safeName(resource)+".toml")), Contents: buf.Bytes()}
}

func (b codexBuilder) configTOML() []byte {
	var buf bytes.Buffer
	writeLine(&buf, "# Codex configuration preview")
	writeLine(&buf, "# Conservative draft only. Review manually before use.")
	writeLine(&buf, "")
	mcpResources := filterResources(b.scan.Resources, model.KindMCPServer)
	if len(mcpResources) == 0 {
		writeLine(&buf, "# No MCP resources discovered for automatic configuration.")
	} else {
		writeLine(&buf, "# MCP resources require manual review; no runnable server entries are emitted.")
		for _, resource := range mcpResources {
			writeLine(&buf, "")
			writeLine(&buf, "# resource_id = %q", resource.ID)
			writeLine(&buf, "# status = %q", resource.Status)
			writeLine(&buf, "# review_required = true")
			writeWarnings(&buf, resource.Warnings)
		}
	}
	if len(b.scan.Warnings) > 0 || len(b.plan.Warnings) > 0 {
		writeLine(&buf, "")
		writeLine(&buf, "# Top-level warnings")
		writeWarnings(&buf, b.scan.Warnings)
		writeWarnings(&buf, b.plan.Warnings)
	}
	return buf.Bytes()
}

func (b codexBuilder) migrationReport(generated []PreviewFile) []byte {
	var buf bytes.Buffer
	writeLine(&buf, "# Migration report")
	writeLine(&buf, "")
	writeLine(&buf, "no real Codex configuration was written. This is a preview only.")

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

func (b codexBuilder) reviewRequiredResources() []model.Resource {
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

func writeResourceSection(buf *bytes.Buffer, title string, resources []model.Resource) {
	writeLine(buf, "")
	writeLine(buf, "## %s", title)
	if len(resources) == 0 {
		writeLine(buf, "- none")
		return
	}
	for _, resource := range resources {
		writeLine(buf, "- `%s` kind=%s scope=%s status=%s source=%s", resource.ID, resource.Kind, resource.Scope, resource.Status, filepath.Base(resource.SourcePath))
		writeWarnings(buf, resource.Warnings)
	}
}

func writeWarnings(buf *bytes.Buffer, warnings []model.Warning) {
	for _, warning := range warnings {
		message := scrubWarningMessage(warning.Message)
		if warning.Code == "secret-redacted" {
			message += " " + security.RedactedValue
		}
		writeLine(buf, "  - warning[%s]: %s", warning.Code, message)
	}
}

func filterResources(resources []model.Resource, kind model.ResourceKind) []model.Resource {
	var filtered []model.Resource
	for _, resource := range resources {
		if resource.Kind == kind {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func statusResources(resources []model.Resource, status model.Status) []model.Resource {
	var filtered []model.Resource
	for _, resource := range resources {
		if resource.Status == status {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}

func scrubWarningMessage(message string) string {
	fields := strings.Fields(message)
	for i, field := range fields {
		trimmed := strings.Trim(field, "\"'`.,;:()[]{}<>")
		if filepath.IsAbs(trimmed) {
			fields[i] = strings.Replace(field, trimmed, filepath.Base(trimmed), 1)
		}
	}
	return strings.Join(fields, " ")
}

func redactSourceLines(contents []byte) []byte {
	lines := strings.Split(string(contents), "\n")
	for i, line := range lines {
		if redacted, ok := redactSourceLine(line); ok {
			lines[i] = redacted
		}
	}
	return []byte(redactInlineSecrets(strings.Join(lines, "\n")))
}

func redactInlineSecrets(contents string) string {
	redacted := contents
	for _, pattern := range inlineSecretPatterns {
		redacted = pattern.ReplaceAllString(redacted, security.RedactedValue)
	}
	return redacted
}

func redactSourceLine(line string) (string, bool) {
	if key, _, ok := strings.Cut(line, ":"); ok && security.IsSecretKey(strings.TrimSpace(key)) {
		return key + ": " + security.RedactedValue, true
	}

	trimmed := strings.TrimLeft(line, " \t")
	leading := line[:len(line)-len(trimmed)]
	fields := strings.Fields(trimmed)
	changed := false
	for i, field := range fields {
		key, _, ok := strings.Cut(field, "=")
		if !ok || !security.IsSecretKey(strings.TrimSpace(key)) {
			continue
		}
		fields[i] = key + "=" + security.RedactedValue
		changed = true
	}
	if !changed {
		return "", false
	}
	return leading + strings.Join(fields, " "), true
}

func readSource(resource model.Resource) ([]byte, error) {
	contents, err := os.ReadFile(resource.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source for %s: %w", resource.ID, err)
	}
	return contents, nil
}

func readOptionalSource(resource model.Resource) ([]byte, error) {
	info, err := os.Stat(resource.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("stat source for %s: %w", resource.ID, err)
	}
	if info.IsDir() {
		return nil, nil
	}
	return readSource(resource)
}

func safeName(resource model.Resource) string {
	name := filepath.Base(resource.SourcePath)
	if resource.Kind == model.KindSkill && name == "SKILL.md" {
		name = filepath.Base(filepath.Dir(resource.SourcePath))
	}
	name = strings.TrimSuffix(name, filepath.Ext(name))
	name = strings.TrimSpace(strings.ToLower(name))
	var out strings.Builder
	lastDash := false
	for _, r := range name {
		allowed := unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-'
		if !allowed {
			if !lastDash {
				out.WriteByte('-')
				lastDash = true
			}
			continue
		}
		out.WriteRune(r)
		lastDash = r == '-'
	}
	clean := strings.Trim(out.String(), "-._")
	if clean == "" {
		return "unnamed"
	}
	return clean
}

func writeLine(buf *bytes.Buffer, format string, args ...any) {
	fmt.Fprintf(buf, format, args...)
	buf.WriteByte('\n')
}
