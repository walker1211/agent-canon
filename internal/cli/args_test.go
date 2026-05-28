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
		{name: "unknown", args: []string{"unknown"}},
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

func TestParseAcceptsValidApplyTargetsAndFlags(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name        string
		args        []string
		applyTarget string
		dryRun      bool
		yes         bool
		global      bool
	}{
		{name: "codex", args: []string{"apply", "codex", "--project", root}, applyTarget: "codex"},
		{name: "codex dry run", args: []string{"apply", "codex", "--dry-run", "--project", root}, applyTarget: "codex", dryRun: true},
		{name: "codex yes", args: []string{"apply", "codex", "--yes", "--project", root}, applyTarget: "codex", yes: true},
		{name: "codex global yes", args: []string{"apply", "codex", "--global", "--yes", "--project", root}, applyTarget: "codex", yes: true, global: true},
		{name: "claude", args: []string{"apply", "claude", "--project", root}, applyTarget: "claude"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := Parse(tc.args, root, root)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if opts.Command != "apply" {
				t.Fatalf("Command = %q, want apply", opts.Command)
			}
			if opts.ApplyTarget != tc.applyTarget {
				t.Fatalf("ApplyTarget = %q, want %q", opts.ApplyTarget, tc.applyTarget)
			}
			if opts.DryRun != tc.dryRun {
				t.Fatalf("DryRun = %v, want %v", opts.DryRun, tc.dryRun)
			}
			if opts.Yes != tc.yes {
				t.Fatalf("Yes = %v, want %v", opts.Yes, tc.yes)
			}
			if opts.Global != tc.global {
				t.Fatalf("Global = %v, want %v", opts.Global, tc.global)
			}
		})
	}
}

func TestParseRejectsInvalidApplyForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing target", args: []string{"apply", "--project", root}},
		{name: "unsupported target", args: []string{"apply", "other", "--project", root}},
		{name: "extra arg", args: []string{"apply", "codex", "extra", "--project", root}},
		{name: "out", args: []string{"apply", "codex", "--out", "preview", "--project", root}},
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

func TestParseAcceptsValidImportForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name   string
		args   []string
		target string
		format string
		memory bool
	}{
		{name: "codex text", args: []string{"import", "codex", "--project", root}, target: "codex", format: "text"},
		{name: "codex json memory", args: []string{"import", "codex", "--format", "json", "--include-memory", "--project", root}, target: "codex", format: "json", memory: true},
		{name: "claude text", args: []string{"import", "claude", "--project", root}, target: "claude", format: "text"},
		{name: "claude json memory", args: []string{"import", "claude", "--format", "json", "--include-memory", "--project", root}, target: "claude", format: "json", memory: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := Parse(tc.args, root, root)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if opts.Command != "import" {
				t.Fatalf("Command = %q, want import", opts.Command)
			}
			if opts.ImportTarget != tc.target {
				t.Fatalf("ImportTarget = %q, want %q", opts.ImportTarget, tc.target)
			}
			if opts.Format != tc.format {
				t.Fatalf("Format = %q, want %q", opts.Format, tc.format)
			}
			if opts.IncludeMemory != tc.memory {
				t.Fatalf("IncludeMemory = %v, want %v", opts.IncludeMemory, tc.memory)
			}
		})
	}
}

func TestParseRejectsInvalidImportForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing target", args: []string{"import", "--project", root}},
		{name: "unsupported target", args: []string{"import", "other", "--project", root}},
		{name: "codex extra arg", args: []string{"import", "codex", "extra", "--project", root}},
		{name: "codex out", args: []string{"import", "codex", "--out", "report.json", "--project", root}},
		{name: "codex dry run", args: []string{"import", "codex", "--dry-run", "--project", root}},
		{name: "codex yes", args: []string{"import", "codex", "--yes", "--project", root}},
		{name: "codex global", args: []string{"import", "codex", "--global", "--project", root}},
		{name: "codex resolve flag", args: []string{"import", "codex", "--ours", "--project", root}},
		{name: "claude extra arg", args: []string{"import", "claude", "extra", "--project", root}},
		{name: "claude out", args: []string{"import", "claude", "--out", "report.json", "--project", root}},
		{name: "claude dry run", args: []string{"import", "claude", "--dry-run", "--project", root}},
		{name: "claude yes", args: []string{"import", "claude", "--yes", "--project", root}},
		{name: "claude global", args: []string{"import", "claude", "--global", "--project", root}},
		{name: "claude resolve flag", args: []string{"import", "claude", "--ours", "--project", root}},
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

func TestParseAcceptsValidVerifyTargetsAndFlags(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name         string
		args         []string
		verifyTarget string
		format       string
	}{
		{name: "codex", args: []string{"verify", "codex", "--project", root}, verifyTarget: "codex", format: "text"},
		{name: "claude", args: []string{"verify", "claude", "--project", root}, verifyTarget: "claude", format: "text"},
		{name: "codex json", args: []string{"verify", "codex", "--format", "json", "--project", root}, verifyTarget: "codex", format: "json"},
		{name: "claude json", args: []string{"verify", "claude", "--format", "json", "--project", root}, verifyTarget: "claude", format: "json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := Parse(tc.args, root, root)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if opts.Command != "verify" {
				t.Fatalf("Command = %q, want verify", opts.Command)
			}
			if opts.VerifyTarget != tc.verifyTarget {
				t.Fatalf("VerifyTarget = %q, want %q", opts.VerifyTarget, tc.verifyTarget)
			}
			if opts.Format != tc.format {
				t.Fatalf("Format = %q, want %q", opts.Format, tc.format)
			}
		})
	}
}

func TestParseRejectsInvalidVerifyForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing target", args: []string{"verify", "--project", root}},
		{name: "unsupported target", args: []string{"verify", "other", "--project", root}},
		{name: "extra arg", args: []string{"verify", "codex", "extra", "--project", root}},
		{name: "out", args: []string{"verify", "codex", "--out", "preview", "--project", root}},
		{name: "dry run", args: []string{"verify", "codex", "--dry-run", "--project", root}},
		{name: "yes", args: []string{"verify", "codex", "--yes", "--project", root}},
		{name: "global", args: []string{"verify", "codex", "--global", "--project", root}},
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

func TestParseAcceptsValidLifecycleCommands(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name       string
		args       []string
		command    string
		format     string
		diffTarget string
		memory     bool
	}{
		{name: "init", args: []string{"init", "--project", root}, command: "init", format: "text"},
		{name: "init json", args: []string{"init", "--format", "json", "--project", root}, command: "init", format: "json"},
		{name: "status", args: []string{"status", "--project", root}, command: "status", format: "text"},
		{name: "status json", args: []string{"status", "--format", "json", "--project", root}, command: "status", format: "json"},
		{name: "diff default", args: []string{"diff", "--project", root}, command: "diff", format: "text", diffTarget: "codex"},
		{name: "diff codex json", args: []string{"diff", "codex", "--format", "json", "--project", root}, command: "diff", format: "json", diffTarget: "codex"},
		{name: "diff include memory", args: []string{"diff", "--include-memory", "--project", root}, command: "diff", format: "text", diffTarget: "codex", memory: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := Parse(tc.args, root, root)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if opts.Command != tc.command {
				t.Fatalf("Command = %q, want %q", opts.Command, tc.command)
			}
			if opts.Format != tc.format {
				t.Fatalf("Format = %q, want %q", opts.Format, tc.format)
			}
			if opts.DiffTarget != tc.diffTarget {
				t.Fatalf("DiffTarget = %q, want %q", opts.DiffTarget, tc.diffTarget)
			}
			if opts.IncludeMemory != tc.memory {
				t.Fatalf("IncludeMemory = %v, want %v", opts.IncludeMemory, tc.memory)
			}
		})
	}
}

func TestParseAcceptsValidRollbackForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name       string
		args       []string
		dryRun     bool
		yes        bool
		global     bool
		rollbackID string
	}{
		{name: "dry run", args: []string{"rollback", "apply-20260527T120000000000000Z", "--dry-run", "--project", root}, dryRun: true, rollbackID: "apply-20260527T120000000000000Z"},
		{name: "yes global", args: []string{"rollback", "apply-20260527T120000000000000Z", "--yes", "--global", "--project", root}, yes: true, global: true, rollbackID: "apply-20260527T120000000000000Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := Parse(tc.args, root, root)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if opts.Command != "rollback" {
				t.Fatalf("Command = %q, want rollback", opts.Command)
			}
			if opts.RollbackID != tc.rollbackID {
				t.Fatalf("RollbackID = %q, want %q", opts.RollbackID, tc.rollbackID)
			}
			if opts.DryRun != tc.dryRun {
				t.Fatalf("DryRun = %v, want %v", opts.DryRun, tc.dryRun)
			}
			if opts.Yes != tc.yes {
				t.Fatalf("Yes = %v, want %v", opts.Yes, tc.yes)
			}
			if opts.Global != tc.global {
				t.Fatalf("Global = %v, want %v", opts.Global, tc.global)
			}
		})
	}
}

func TestParseRejectsInvalidRollbackForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "missing ID", args: []string{"rollback", "--project", root}},
		{name: "extra arg", args: []string{"rollback", "apply-1", "extra", "--project", root}},
		{name: "json format", args: []string{"rollback", "apply-1", "--format", "json", "--project", root}},
		{name: "out", args: []string{"rollback", "apply-1", "--out", "rollback.json", "--project", root}},
		{name: "include memory", args: []string{"rollback", "apply-1", "--include-memory", "--project", root}},
		{name: "resolve flag", args: []string{"rollback", "apply-1", "--ours", "--project", root}},
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

func TestParseRejectsInvalidLifecycleForms(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "diff unsupported target", args: []string{"diff", "claude", "--project", root}},
		{name: "diff extra arg", args: []string{"diff", "codex", "extra", "--project", root}},
		{name: "init out", args: []string{"init", "--out", "manifest.json", "--project", root}},
		{name: "status out", args: []string{"status", "--out", "status.json", "--project", root}},
		{name: "diff out", args: []string{"diff", "--out", "diff.json", "--project", root}},
		{name: "init include memory", args: []string{"init", "--include-memory", "--project", root}},
		{name: "status include memory", args: []string{"status", "--include-memory", "--project", root}},
		{name: "init dry run", args: []string{"init", "--dry-run", "--project", root}},
		{name: "status yes", args: []string{"status", "--yes", "--project", root}},
		{name: "diff global", args: []string{"diff", "--global", "--project", root}},
		{name: "init resolve flag", args: []string{"init", "--ours", "--project", root}},
		{name: "status manual flag", args: []string{"status", "--manual", "value", "--project", root}},
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
				"scan, status, diff, conflicts, and verify are read-only",
				"init writes only project .agent-canon",
				"agent-canon init [flags]",
				"agent-canon status [flags]",
				"agent-canon diff [codex] [flags]",
				"agent-canon rollback <apply-id> [flags]",
				"agent-canon import claude [flags]",
				"agent-canon import codex [flags]",
				"plan --out writes",
				"export codex --out writes",
				"sync/resolve write only project .agent-canon",
				"agent-canon sync claude codex [flags]",
				"agent-canon conflicts [flags]",
				"agent-canon resolve <conflict-id>",
				"agent-canon verify codex [flags]",
				"agent-canon verify claude [flags]",
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
