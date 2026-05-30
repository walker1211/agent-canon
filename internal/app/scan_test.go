package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestRunScanFormatJSONPrintsValidScanJSON(t *testing.T) {
	fixture := basicFixture(t)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"scan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	var report model.ScanReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid scan JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.ScanSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", report.SchemaVersion, model.ScanSchemaVersion)
	}
}

func TestRunPlanFormatJSONPrintsValidPlanJSON(t *testing.T) {
	fixture := basicFixture(t)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	var report model.PlanReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not valid plan JSON: %v\n%s", err, stdout.String())
	}
	if report.SchemaVersion != model.PlanSchemaVersion {
		t.Fatalf("schemaVersion = %q, want %q", report.SchemaVersion, model.PlanSchemaVersion)
	}
}

func TestRunPlanOutWritesPlanJSONAndPrintsShortSummary(t *testing.T) {
	fixture := basicFixture(t)
	outPath := filepath.Join(t.TempDir(), "plan.json")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--format", "text", "--out", outPath}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}

	payload, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out path: %v", err)
	}
	var report model.PlanReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("out file is not valid plan JSON: %v\n%s", err, string(payload))
	}
	if report.SchemaVersion != model.PlanSchemaVersion {
		t.Fatalf("out schemaVersion = %q, want %q", report.SchemaVersion, model.PlanSchemaVersion)
	}
	if !strings.Contains(stdout.String(), "agent-canon plan: claude -> codex") || !strings.Contains(stdout.String(), "wrote JSON plan") {
		t.Fatalf("stdout missing short summary: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "{\n") || strings.Contains(stdout.String(), "\"schemaVersion\"") {
		t.Fatalf("stdout contains full JSON, want short summary only: %q", stdout.String())
	}
}

func TestRunPlanOutWriteFailureReturnsExitOne(t *testing.T) {
	fixture := basicFixture(t)
	var stdout, stderr bytes.Buffer
	badPath := filepath.Join(t.TempDir(), "missing-parent", "plan.json")

	code := app.Run([]string{"plan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome, "--out", badPath}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunMalformedSettingsJSONReturnsExitTwo(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	claudeHome := filepath.Join(root, "claude-home")
	codexHome := filepath.Join(root, "codex-home")
	writeFile(t, filepath.Join(claudeHome, "settings.json"), "{")
	mustMkdir(t, project)
	mustMkdir(t, codexHome)

	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"scan", "--project", project, "--claude-home", claudeHome, "--codex-home", codexHome}, project, root, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunReturnsExitOneWhenStdoutWriteFails(t *testing.T) {
	fixture := basicFixture(t)
	var stderr bytes.Buffer

	code := app.Run([]string{"scan", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, failingWriter{}, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, stderr.String())
	}
}

func TestRunWithIOAcceptsStdinAndPreservesHelpBehavior(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"scan", "--help"},
		{"apply", "--help"},
		{"apply", "codex", "--help"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			code := app.RunWithIO(args, "/definitely/missing/cwd", "/definitely/missing/home", strings.NewReader("yes\n"), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), "agent-canon apply codex") {
				t.Fatalf("help output missing apply command: %q", stdout.String())
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

type fixturePaths struct {
	home       string
	project    string
	claudeHome string
	codexHome  string
}

func basicFixture(t *testing.T) fixturePaths {
	t.Helper()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	fixture := filepath.Join(repoRoot, "testdata", "basic")
	return fixturePaths{
		home:       fixture,
		project:    filepath.Join(fixture, "project"),
		claudeHome: filepath.Join(fixture, "claude-home"),
		codexHome:  filepath.Join(fixture, "codex-home"),
	}
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
