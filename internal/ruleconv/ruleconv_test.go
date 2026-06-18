package ruleconv

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodexSkillDocumentPreservesPathScopedRuleMetadata(t *testing.T) {
	rule := PathScopedRule{
		Paths: []string{"**/*.go", "go.mod"},
		Body:  []byte("# Go Rule\n\nUse Go-specific guidance.\n"),
	}

	contents := CodexSkillDocument(rule, CodexSkillMetadata{
		Name:           "go-development",
		ResourceID:     "rule:global-go-development",
		SourceScope:    "global",
		SourceStrategy: "convert-path-scoped-rule-to-skill",
	})

	for _, want := range []string{
		"name: go-development",
		"source_id: rule:global-go-development",
		"source_paths:",
		"    - \"**/*.go\"",
		"    - \"go.mod\"",
		"# Go Rule",
		"Use Go-specific guidance.",
	} {
		if !strings.Contains(string(contents), want) {
			t.Fatalf("CodexSkillDocument missing %q in:\n%s", want, string(contents))
		}
	}
	roundTrip := FromCodexSkill(contents)
	if !bytes.Equal(bytes.TrimSpace(roundTrip.Body), bytes.TrimSpace(rule.Body)) {
		t.Fatalf("round-trip body = %q, want %q", roundTrip.Body, rule.Body)
	}
}

func TestClaudeRuleDocumentRestoresPathsFrontmatter(t *testing.T) {
	rule := PathScopedRule{
		Paths: []string{"**/*.go", "go.mod"},
		Body:  []byte("# Go Rule\n\nUse Go-specific guidance.\n"),
	}

	contents := ClaudeRuleDocument(rule)

	for _, want := range []string{
		"paths:",
		"  - \"**/*.go\"",
		"  - \"go.mod\"",
		"# Go Rule",
		"Use Go-specific guidance.",
	} {
		if !strings.Contains(string(contents), want) {
			t.Fatalf("ClaudeRuleDocument missing %q in:\n%s", want, string(contents))
		}
	}
	roundTrip := FromClaude(contents)
	if !bytes.Equal(bytes.TrimSpace(roundTrip.Body), bytes.TrimSpace(rule.Body)) {
		t.Fatalf("round-trip body = %q, want %q", roundTrip.Body, rule.Body)
	}
}
