package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAcceptsValidScanDefaultsAndWarnsForMissingDefaultHomes(t *testing.T) {
	root := t.TempDir()
	opts, err := Parse([]string{"scan", "--project", root}, root, root)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if opts.Command != "scan" {
		t.Fatalf("Command = %q, want scan", opts.Command)
	}
	if opts.From != "claude" || opts.To != "codex" {
		t.Fatalf("direction = %s -> %s, want claude -> codex", opts.From, opts.To)
	}
	if opts.Project != root {
		t.Fatalf("Project = %q, want %q", opts.Project, root)
	}
	if opts.ClaudeHome != root+"/.claude" || opts.CodexHome != root+"/.codex" {
		t.Fatalf("homes = %q, %q; want default homes under temp home", opts.ClaudeHome, opts.CodexHome)
	}
	if opts.Format != "text" {
		t.Fatalf("Format = %q, want text", opts.Format)
	}
	if len(opts.Warnings) != 2 {
		t.Fatalf("Warnings len = %d, want 2 for missing default homes: %#v", len(opts.Warnings), opts.Warnings)
	}
}

func TestParseRejectsMissingAndUnknownCommand(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing", args: []string{}},
		{name: "unknown", args: []string{"apply"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args, root, root)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if ExitCode(err) != 1 {
				t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
			}
		})
	}
}

func TestParseRejectsScanOut(t *testing.T) {
	root := t.TempDir()
	_, err := Parse([]string{"scan", "--project", root, "--out", "plan.json"}, root, root)
	if err == nil {
		t.Fatal("Parse returned nil error")
	}
	if ExitCode(err) != 1 {
		t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
	}
}

func TestParseAcceptsExportCodexOut(t *testing.T) {
	root := t.TempDir()
	opts, err := Parse([]string{"export", "codex", "--project", root, "--out", "preview"}, root, root)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if opts.Command != "export" {
		t.Fatalf("Command = %q, want export", opts.Command)
	}
	if opts.ExportTarget != "codex" {
		t.Fatalf("ExportTarget = %q, want codex", opts.ExportTarget)
	}
	if opts.Out != "preview" {
		t.Fatalf("Out = %q, want preview", opts.Out)
	}
}

func TestParseRejectsInvalidExportForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing target", args: []string{"export", "--project", root, "--out", "preview"}},
		{name: "unsupported target", args: []string{"export", "claude", "--project", root, "--out", "preview"}},
		{name: "missing out", args: []string{"export", "codex", "--project", root}},
		{name: "format not supported", args: []string{"export", "codex", "--project", root, "--format", "json", "--out", "preview"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args, root, root)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if ExitCode(err) != 1 {
				t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
			}
		})
	}
}

func TestParseValidatesFormat(t *testing.T) {
	root := t.TempDir()
	_, err := Parse([]string{"plan", "--project", root, "--format", "yaml"}, root, root)
	if err == nil {
		t.Fatal("Parse returned nil error")
	}
	if !strings.Contains(err.Error(), "--format") {
		t.Fatalf("error = %q, want format validation", err)
	}
}

func TestParseAcceptsOnlyClaudeToCodex(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"scan", "--project", root, "--from", "codex", "--to", "claude"},
		{"plan", "--project", root, "--from", "claude", "--to", "other"},
	} {
		_, err := Parse(args, root, root)
		if err == nil {
			t.Fatalf("Parse(%v) returned nil error", args)
		}
		if !strings.Contains(err.Error(), "claude -> codex") {
			t.Fatalf("error = %q, want direction validation", err)
		}
	}
}

func TestParseRequiresExistingProject(t *testing.T) {
	root := t.TempDir()
	_, err := Parse([]string{"scan", "--project", root + "/missing"}, root, root)
	if err == nil {
		t.Fatal("Parse returned nil error")
	}
	if ExitCode(err) != 1 {
		t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
	}
}

func TestParseRequiresExplicitCustomHomesToExist(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"scan", "--project", root, "--claude-home", root + "/missing-claude"},
		{"scan", "--project", root, "--codex-home", root + "/missing-codex"},
	} {
		_, err := Parse(args, root, root)
		if err == nil {
			t.Fatalf("Parse(%v) returned nil error", args)
		}
		if ExitCode(err) != 1 {
			t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
		}
	}
}

func TestParseAcceptsValidSyncClaudeCodex(t *testing.T) {
	root := t.TempDir()
	opts, err := Parse([]string{"sync", "claude", "codex", "--project", root, "--format", "json"}, root, root)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if opts.Command != "sync" {
		t.Fatalf("Command = %q, want sync", opts.Command)
	}
	if opts.SyncSource != "claude" || opts.SyncTarget != "codex" {
		t.Fatalf("sync direction = %s -> %s, want claude -> codex", opts.SyncSource, opts.SyncTarget)
	}
	if opts.Format != "json" {
		t.Fatalf("Format = %q, want json", opts.Format)
	}
}

func TestParseRejectsInvalidSyncForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing direction", args: []string{"sync", "--project", root}},
		{name: "reversed direction", args: []string{"sync", "codex", "claude", "--project", root}},
		{name: "extra arg", args: []string{"sync", "claude", "codex", "extra", "--project", root}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args, root, root)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if ExitCode(err) != 1 {
				t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
			}
		})
	}
}

func TestParseAcceptsValidConflicts(t *testing.T) {
	root := t.TempDir()
	opts, err := Parse([]string{"conflicts", "--project", root, "--format", "json"}, root, root)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if opts.Command != "conflicts" {
		t.Fatalf("Command = %q, want conflicts", opts.Command)
	}
	if opts.Format != "json" {
		t.Fatalf("Format = %q, want json", opts.Format)
	}
}

func TestParseRejectsConflictsUnexpectedArg(t *testing.T) {
	root := t.TempDir()
	_, err := Parse([]string{"conflicts", "unexpected", "--project", root}, root, root)
	if err == nil {
		t.Fatal("Parse returned nil error")
	}
	if ExitCode(err) != 1 {
		t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
	}
}

func TestParseAcceptsValidResolveDecisions(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name        string
		args        []string
		decision    string
		manualValue string
	}{
		{name: "ours", args: []string{"resolve", "conflict-1", "--ours", "--project", root}, decision: "ours"},
		{name: "theirs", args: []string{"resolve", "conflict-1", "--theirs", "--project", root}, decision: "theirs"},
		{name: "accept suggestion", args: []string{"resolve", "conflict-1", "--accept-suggestion", "--project", root}, decision: "accept-suggestion"},
		{name: "manual", args: []string{"resolve", "conflict-1", "--manual", "merged value", "--project", root}, decision: "manual", manualValue: "merged value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := Parse(tc.args, root, root)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if opts.Command != "resolve" {
				t.Fatalf("Command = %q, want resolve", opts.Command)
			}
			if opts.ConflictID != "conflict-1" {
				t.Fatalf("ConflictID = %q, want conflict-1", opts.ConflictID)
			}
			if opts.ResolveDecision != tc.decision {
				t.Fatalf("ResolveDecision = %q, want %q", opts.ResolveDecision, tc.decision)
			}
			if opts.ManualValue != tc.manualValue {
				t.Fatalf("ManualValue = %q, want %q", opts.ManualValue, tc.manualValue)
			}
		})
	}
}

func TestParseRejectsInvalidResolveForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing ID", args: []string{"resolve", "--ours", "--project", root}},
		{name: "missing decision", args: []string{"resolve", "conflict-1", "--project", root}},
		{name: "multiple decisions", args: []string{"resolve", "conflict-1", "--ours", "--theirs", "--project", root}},
		{name: "empty manual", args: []string{"resolve", "conflict-1", "--manual", "", "--project", root}},
		{name: "explicit format", args: []string{"resolve", "conflict-1", "--ours", "--format", "json", "--project", root}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.args, root, root)
			if err == nil {
				t.Fatal("Parse returned nil error")
			}
			if ExitCode(err) != 1 {
				t.Fatalf("ExitCode = %d, want 1", ExitCode(err))
			}
		})
	}
}

func TestRunHelpAliasesDoNotValidatePaths(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"--help"},
		{"-h"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr strings.Builder
			code := Run(args, "/definitely/missing/cwd", "/definitely/missing/home", &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			help := stdout.String()
			for _, want := range []string{
				"scan and conflicts are read-only",
				"plan --out writes",
				"export codex --out writes",
				"sync/resolve write only project .agent-canon",
				"agent-canon sync claude codex [flags]",
				"agent-canon conflicts [flags]",
				"agent-canon resolve <conflict-id>",
			} {
				if !strings.Contains(help, want) {
					t.Fatalf("help output missing %q: %q", want, help)
				}
			}
			if stderr.String() != "" {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunReturnsExitOneForValidationErrors(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr strings.Builder
	if code := Run([]string{"scan", "--project", root, "--out", "plan.json"}, root, root, &stdout, &stderr); code != 1 {
		t.Fatalf("validation exit code = %d, want 1", code)
	}
}

func TestRunReturnsExitOneWhenOutputWriteFails(t *testing.T) {
	root := t.TempDir()
	stderr := &strings.Builder{}
	code := Run([]string{"help"}, root, root, failingWriter{}, stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "write output") {
		t.Fatalf("stderr = %q, want write output error", stderr.String())
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}
