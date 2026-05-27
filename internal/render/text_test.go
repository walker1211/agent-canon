package render_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

const fixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestApplyTextPrintsChangedFilesAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := render.ApplyTextReport{
		Target:       "codex",
		Project:      "/repo",
		Mode:         "dry-run",
		BackupDir:    "/repo/.agent-canon/backups/apply-001",
		ManifestPath: "/repo/.agent-canon/rollback/apply-001.json",
		Changes: []model.ApplyFileChange{
			{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionModify, BeforeHash: "sha256:before", AfterHash: "sha256:after"},
			{Path: "/repo/.codex/config.toml", Scope: model.ScopeProject, Action: model.ApplyActionNoop, BeforeHash: "sha256:same", AfterHash: "sha256:same"},
		},
		Warnings: []model.Warning{{Code: "secret-redacted", Message: "GITHUB_TOKEN=" + fixtureSecret}},
	}

	if err := render.ApplyText(&out, report); err != nil {
		t.Fatalf("ApplyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon apply codex: dry-run",
		"Project: /repo",
		"Summary: create=0 modify=1 noop=1 warnings=1",
		"- modify [project] /repo/AGENTS.md",
		"Backup: /repo/.agent-canon/backups/apply-001",
		"Manifest: /repo/.agent-canon/rollback/apply-001.json",
		"GITHUB_TOKEN=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("ApplyText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("ApplyText leaked fixture secret:\n%s", text)
	}
}

func TestApplyTextPropagatesWriteErrors(t *testing.T) {
	err := render.ApplyText(failingWriter{}, render.ApplyTextReport{Target: "codex", Mode: "dry-run"})
	if err == nil {
		t.Fatalf("ApplyText returned nil error for failing writer")
	}
}

func TestVerifyTextPrintsSummaryChecksAndRedactsWarnings(t *testing.T) {
	var out strings.Builder
	report := model.VerifyReport{
		SchemaVersion: model.VerifySchemaVersion,
		Target:        "codex",
		Project:       "/repo",
		Checks: []model.VerifyCheck{
			{ID: "codex-config-project", Target: "codex", Status: model.VerifyStatusPass, Message: "Codex config passed.", Path: "/repo/.codex/config.toml"},
			{ID: "codex-mcp-list", Target: "codex", Status: model.VerifyStatusWarn, Message: "GITHUB_TOKEN=" + fixtureSecret},
			{ID: "sync-conflicts", Target: "codex", Status: model.VerifyStatusFail, Message: "1 open conflict remains"},
		},
		Summary:  model.VerifySummary{Pass: 1, Warn: 1, Fail: 1},
		Warnings: []model.Warning{{Code: "secret", Message: "token=" + fixtureSecret}},
	}

	if err := render.VerifyText(&out, report); err != nil {
		t.Fatalf("VerifyText returned error: %v", err)
	}
	text := out.String()
	for _, want := range []string{
		"agent-canon verify codex",
		"Project: /repo",
		"Summary: pass=1 warn=1 fail=1 warnings=1",
		"Checks:",
		"- pass codex-config-project: Codex config passed. (/repo/.codex/config.toml)",
		"- warn codex-mcp-list: GITHUB_TOKEN=" + security.RedactedValue,
		"- fail sync-conflicts: 1 open conflict remains",
		"Warnings:",
		"- warning[secret]: token=" + security.RedactedValue,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("VerifyText output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureSecret) {
		t.Fatalf("VerifyText leaked fixture secret:\n%s", text)
	}
}

func TestVerifyTextPropagatesWriteErrors(t *testing.T) {
	err := render.VerifyText(failingWriter{}, model.VerifyReport{Target: "codex"})
	if err == nil {
		t.Fatalf("VerifyText returned nil error for failing writer")
	}
}

func TestVerifyJSONWritesReportShape(t *testing.T) {
	var out strings.Builder
	report := model.VerifyReport{
		SchemaVersion: model.VerifySchemaVersion,
		Target:        "claude",
		Project:       "/repo",
		Checks:        []model.VerifyCheck{{ID: "claude-instructions", Target: "claude", Status: model.VerifyStatusPass, Message: "CLAUDE.md passed."}},
		Summary:       model.VerifySummary{Pass: 1},
		Warnings:      []model.Warning{},
	}

	if err := render.VerifyJSON(&out, report); err != nil {
		t.Fatalf("VerifyJSON returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("VerifyJSON emitted invalid JSON: %v\n%s", err, out.String())
	}
	if got["schemaVersion"] != "agent-canon.verify.v1" || got["target"] != "claude" {
		t.Fatalf("VerifyJSON output = %#v", got)
	}
}

func TestVerifyJSONPropagatesWriteErrors(t *testing.T) {
	err := render.VerifyJSON(failingWriter{}, model.VerifyReport{Target: "codex"})
	if err == nil {
		t.Fatalf("VerifyJSON returned nil error for failing writer")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
