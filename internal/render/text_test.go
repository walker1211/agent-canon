package render_test

import (
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

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
