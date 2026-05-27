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
			if !strings.Contains(stdout.String(), "agent-canon is read-only") || !strings.Contains(stdout.String(), "only plan --out writes") {
				t.Fatalf("help output missing read-only/write boundary: %q", stdout.String())
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
