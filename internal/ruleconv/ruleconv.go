package ruleconv

import (
	"bytes"
	"fmt"
	"strings"
)

type PathScopedRule struct {
	Body  []byte
	Paths []string
}

type CodexSkillMetadata struct {
	Name           string
	ResourceID     string
	SourceScope    string
	SourceStrategy string
}

func FromClaude(contents []byte) PathScopedRule {
	body, paths := stripFrontmatter(contents, "paths")
	return PathScopedRule{Body: body, Paths: paths}
}

func FromCodexSkill(contents []byte) PathScopedRule {
	body, paths := stripFrontmatter(contents, "source_paths")
	return PathScopedRule{Body: stripGeneratedSkillComment(body), Paths: paths}
}

func SemanticDocument(rule PathScopedRule) []byte {
	var buf bytes.Buffer
	buf.WriteString("## path-scoped-rule-paths\n")
	if len(rule.Paths) == 0 {
		buf.WriteString("<none>\n")
	} else {
		for _, path := range rule.Paths {
			buf.WriteString(path)
			buf.WriteByte('\n')
		}
	}
	buf.WriteString("\n## path-scoped-rule-body\n\n")
	buf.Write(bytes.TrimSpace(rule.Body))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func CodexSkillDocument(rule PathScopedRule, meta CodexSkillMetadata) []byte {
	var buf bytes.Buffer
	writeLine(&buf, "---")
	writeLine(&buf, "name: %s", meta.Name)
	writeLine(&buf, "description: >-")
	writeLine(&buf, "  %s", pathScopedRuleDescription(meta.ResourceID, rule.Paths))
	writeLine(&buf, "agent_canon:")
	writeLine(&buf, "  source_tool: claude")
	writeLine(&buf, "  source_kind: Rule")
	writeLine(&buf, "  source_id: %s", meta.ResourceID)
	writeLine(&buf, "  source_scope: %s", meta.SourceScope)
	writeLine(&buf, "  source_strategy: %s", meta.SourceStrategy)
	if len(rule.Paths) == 0 {
		writeLine(&buf, "  source_paths: []")
	} else {
		writeLine(&buf, "  source_paths:")
		for _, path := range rule.Paths {
			writeLine(&buf, "    - %q", path)
		}
	}
	writeLine(&buf, "---")
	writeLine(&buf, "")
	writeLine(&buf, "<!-- Generated Codex skill from Claude path-scoped rule %s. -->", meta.ResourceID)
	writeLine(&buf, "")
	buf.Write(bytes.TrimSpace(rule.Body))
	writeLine(&buf, "")
	return buf.Bytes()
}

func ClaudeRuleDocument(rule PathScopedRule) []byte {
	var buf bytes.Buffer
	writeLine(&buf, "---")
	if len(rule.Paths) == 0 {
		writeLine(&buf, "paths: []")
	} else {
		writeLine(&buf, "paths:")
		for _, path := range rule.Paths {
			writeLine(&buf, "  - %q", path)
		}
	}
	writeLine(&buf, "---")
	writeLine(&buf, "")
	buf.Write(bytes.TrimSpace(rule.Body))
	writeLine(&buf, "")
	return buf.Bytes()
}

func pathScopedRuleDescription(resourceID string, paths []string) string {
	if len(paths) == 0 {
		return "Use when this Claude path-scoped rule applies. Converted from Claude path-scoped rule " + resourceID + "."
	}
	return "Use when working with files matching " + strings.Join(paths, ", ") + ". Converted from Claude path-scoped rule " + resourceID + "."
}

func writeLine(buf *bytes.Buffer, format string, args ...any) {
	if len(args) == 0 {
		buf.WriteString(format)
	} else {
		fmt.Fprintf(buf, format, args...)
	}
	buf.WriteByte('\n')
}

func stripFrontmatter(contents []byte, pathsKey string) ([]byte, []string) {
	text := strings.TrimPrefix(string(contents), "\ufeff")
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return contents, nil
	}
	var paths []string
	inPaths := false
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			return []byte(strings.Join(lines[i+1:], "\n")), paths
		}
		if strings.HasPrefix(trimmed, pathsKey+":") {
			inPaths = true
			paths = append(paths, parseInlinePathValues(strings.TrimSpace(strings.TrimPrefix(trimmed, pathsKey+":")))...)
			continue
		}
		if inPaths && strings.HasPrefix(trimmed, "-") {
			if path := cleanPath(strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))); path != "" {
				paths = append(paths, path)
			}
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inPaths = false
		}
	}
	return contents, nil
}

func stripGeneratedSkillComment(contents []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return contents
	}
	trimmed := strings.TrimSpace(lines[i])
	if !strings.HasPrefix(trimmed, "<!-- Generated Codex skill from Claude path-scoped rule ") {
		return contents
	}
	i++
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return []byte(strings.Join(lines[i:], "\n"))
}

func parseInlinePathValues(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.Trim(value, "[]")
	parts := strings.Split(value, ",")
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		if path := cleanPath(part); path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func cleanPath(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"'`)
}
