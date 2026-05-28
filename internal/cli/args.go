package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const helpText = `agent-canon is a migration inventory, import, plan, sync, conflict, preview export, compile, apply, verify, workspace lifecycle, and rollback tool.

Write boundary: scan, status, diff, conflicts, verify, and compile validation are read-only; init writes only project .agent-canon; import claude/codex writes only project .agent-canon import metadata and the selected baseline snapshot; plan --out writes a JSON plan file; export codex --out writes a Codex preview directory; compile codex --out writes a Codex preview directory after baseline and conflict checks; sync/resolve write only project .agent-canon; apply codex writes Codex target files only after conflict checks, backup, and confirmation; rollback writes only manifest-listed targets after drift checks and confirmation.

Usage:
  agent-canon init [flags]
  agent-canon scan [flags]
  agent-canon status [flags]
  agent-canon diff [codex] [flags]
  agent-canon plan [flags]
  agent-canon export codex [flags]
  agent-canon import claude [flags]
  agent-canon import codex [flags]
  agent-canon compile codex --out <dir> [flags]
  agent-canon sync claude codex [flags]
  agent-canon conflicts [flags]
  agent-canon resolve <conflict-id> --ours
  agent-canon resolve <conflict-id> --theirs
  agent-canon resolve <conflict-id> --accept-suggestion
  agent-canon resolve <conflict-id> --manual <value>
  agent-canon apply codex [flags]
  agent-canon apply claude [flags]
  agent-canon rollback <apply-id> [flags]
  agent-canon verify codex [flags]
  agent-canon verify claude [flags]

Commands:
  init          Initialize project .agent-canon workspace metadata
  scan          Read-only inventory of Claude and Codex resources
  status        Read-only project .agent-canon workspace status
  diff          Read-only diff from base snapshots to current Claude/Codex state
  plan          Read-only migration plan generation except when --out writes a JSON plan file
  export codex   Write a Codex preview directory only when --out is set
  import claude  Import current Claude state into project .agent-canon metadata
  import codex   Import current Codex state into project .agent-canon metadata
  compile codex  Write a Codex preview directory from existing canon sync state
  sync           Sync claude to codex metadata; writes only project .agent-canon
  conflicts     Read-only conflict listing
  resolve       Resolve one conflict; writes only project .agent-canon
  apply codex   Apply Codex target files after conflict checks, backup, and confirmation
  apply claude  Unsupported in agent-canon; Codex -> Claude import is not implemented yet
  rollback      Roll back one apply codex manifest after drift checks and confirmation
  verify codex  Read-only validation of Codex targets, skills, MCP hints, and conflict state
  verify claude Read-only validation of Claude targets, settings, and conflict state

Flags:
  --from string          source tool; currently accepts only claude (default "claude")
  --to string            target tool; currently accepts only codex (default "codex")
  --project string       project directory (default current working directory)
  --claude-home string   Claude Code home (default ~/.claude)
  --codex-home string    Codex home (default ~/.codex)
  --format string        init/scan/status/diff/plan/import/sync/conflicts/verify output format: text or json (default "text")
  --out string           plan: write JSON plan to this path; export/compile codex: write preview directory to this path
  --include-memory       scan/plan/import/sync/diff/verify memory indexes and candidates only; does not migrate content
  --dry-run              apply codex/rollback: show planned changes without writing
  --yes                  apply codex/rollback: skip interactive confirmation
  --global               apply codex/rollback: allow writes under --codex-home
`

type Options struct {
	Command         string
	ExportTarget    string
	ImportTarget    string
	CompileTarget   string
	SyncSource      string
	SyncTarget      string
	ConflictID      string
	ResolveDecision string
	ManualValue     string
	ApplyTarget     string
	RollbackID      string
	VerifyTarget    string
	DiffTarget      string
	From            string
	To              string
	Project         string
	ClaudeHome      string
	CodexHome       string
	Format          string
	Out             string
	IncludeMemory   bool
	DryRun          bool
	Yes             bool
	Global          bool
	Warnings        []string
}

type usageError struct {
	message string
	code    int
}

func (e usageError) Error() string { return e.message }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var usage usageError
	if errors.As(err, &usage) {
		return usage.code
	}
	return 1
}

func Parse(args []string, cwd string, homeDir string) (Options, error) {
	if len(args) == 0 {
		return Options{}, usageError{message: "missing command", code: 1}
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		return Options{Command: "help"}, nil
	}

	command := args[0]
	if command != "init" && command != "scan" && command != "status" && command != "diff" && command != "plan" && command != "export" && command != "import" && command != "compile" && command != "sync" && command != "conflicts" && command != "resolve" && command != "apply" && command != "rollback" && command != "verify" {
		return Options{}, usageError{message: fmt.Sprintf("unknown command %q", command), code: 1}
	}

	exportTarget := ""
	importTarget := ""
	compileTarget := ""
	syncSource := ""
	syncTarget := ""
	conflictID := ""
	applyTarget := ""
	rollbackID := ""
	verifyTarget := ""
	diffTarget := ""
	flagArgs := args[1:]
	switch command {
	case "diff":
		diffTarget = "codex"
		if len(flagArgs) > 0 && flagArgs[0] != "" && flagArgs[0][0] != '-' {
			diffTarget = flagArgs[0]
			if diffTarget != "codex" {
				return Options{}, usageError{message: fmt.Sprintf("unsupported diff target %q", diffTarget), code: 1}
			}
			flagArgs = flagArgs[1:]
		}
	case "export":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "export requires target codex", code: 1}
		}
		exportTarget = flagArgs[0]
		if exportTarget != "codex" {
			return Options{}, usageError{message: fmt.Sprintf("unsupported export target %q", exportTarget), code: 1}
		}
		flagArgs = flagArgs[1:]
	case "import":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "import requires target claude or codex", code: 1}
		}
		importTarget = flagArgs[0]
		if importTarget != "claude" && importTarget != "codex" {
			return Options{}, usageError{message: fmt.Sprintf("unsupported import target %q", importTarget), code: 1}
		}
		flagArgs = flagArgs[1:]
	case "compile":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "compile requires target codex", code: 1}
		}
		compileTarget = flagArgs[0]
		if compileTarget != "codex" {
			return Options{}, usageError{message: fmt.Sprintf("unsupported compile target %q", compileTarget), code: 1}
		}
		flagArgs = flagArgs[1:]
	case "sync":
		if len(flagArgs) < 2 || flagArgs[0] == "" || flagArgs[0][0] == '-' || flagArgs[1] == "" || flagArgs[1][0] == '-' {
			return Options{}, usageError{message: "sync requires direction claude codex", code: 1}
		}
		syncSource = flagArgs[0]
		syncTarget = flagArgs[1]
		if syncSource != "claude" || syncTarget != "codex" {
			return Options{}, usageError{message: "agent-canon supports only sync claude codex", code: 1}
		}
		flagArgs = flagArgs[2:]
	case "resolve":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "resolve requires conflict ID", code: 1}
		}
		conflictID = flagArgs[0]
		flagArgs = flagArgs[1:]
	case "apply":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "apply requires target codex or claude", code: 1}
		}
		applyTarget = flagArgs[0]
		if applyTarget != "codex" && applyTarget != "claude" {
			return Options{}, usageError{message: fmt.Sprintf("unsupported apply target %q", applyTarget), code: 1}
		}
		flagArgs = flagArgs[1:]
	case "rollback":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "rollback requires apply ID", code: 1}
		}
		rollbackID = flagArgs[0]
		flagArgs = flagArgs[1:]
	case "verify":
		if len(flagArgs) == 0 || flagArgs[0] == "" || flagArgs[0][0] == '-' {
			return Options{}, usageError{message: "verify requires target codex or claude", code: 1}
		}
		verifyTarget = flagArgs[0]
		if verifyTarget != "codex" && verifyTarget != "claude" {
			return Options{}, usageError{message: fmt.Sprintf("unsupported verify target %q", verifyTarget), code: 1}
		}
		flagArgs = flagArgs[1:]
	}

	defaults := Options{
		Command:       command,
		ExportTarget:  exportTarget,
		ImportTarget:  importTarget,
		CompileTarget: compileTarget,
		SyncSource:    syncSource,
		SyncTarget:    syncTarget,
		ConflictID:    conflictID,
		ApplyTarget:   applyTarget,
		RollbackID:    rollbackID,
		VerifyTarget:  verifyTarget,
		DiffTarget:    diffTarget,
		From:          "claude",
		To:            "codex",
		Project:       cwd,
		ClaudeHome:    filepath.Join(homeDir, ".claude"),
		CodexHome:     filepath.Join(homeDir, ".codex"),
		Format:        "text",
	}
	opts := defaults

	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.From, "from", opts.From, "source tool")
	flags.StringVar(&opts.To, "to", opts.To, "target tool")
	flags.StringVar(&opts.Project, "project", opts.Project, "project directory")
	flags.StringVar(&opts.ClaudeHome, "claude-home", opts.ClaudeHome, "Claude Code home")
	flags.StringVar(&opts.CodexHome, "codex-home", opts.CodexHome, "Codex home")
	flags.StringVar(&opts.Format, "format", opts.Format, "output format")
	flags.StringVar(&opts.Out, "out", opts.Out, "plan output path")
	flags.BoolVar(&opts.IncludeMemory, "include-memory", opts.IncludeMemory, "include memory candidates")
	flags.BoolVar(&opts.DryRun, "dry-run", opts.DryRun, "show planned apply or rollback changes without writing")
	flags.BoolVar(&opts.Yes, "yes", opts.Yes, "skip apply or rollback confirmation")
	flags.BoolVar(&opts.Global, "global", opts.Global, "allow apply or rollback writes under --codex-home")
	resolveOurs := false
	resolveTheirs := false
	resolveAcceptSuggestion := false
	flags.BoolVar(&resolveOurs, "ours", false, "resolve conflict using our value")
	flags.BoolVar(&resolveTheirs, "theirs", false, "resolve conflict using their value")
	flags.BoolVar(&resolveAcceptSuggestion, "accept-suggestion", false, "resolve conflict using suggested value")
	flags.StringVar(&opts.ManualValue, "manual", opts.ManualValue, "resolve conflict using manual value")
	if err := flags.Parse(flagArgs); err != nil {
		return Options{}, usageError{message: err.Error(), code: 1}
	}
	if flags.NArg() > 0 {
		return Options{}, usageError{message: fmt.Sprintf("unexpected argument %q", flags.Arg(0)), code: 1}
	}

	if opts.From != "claude" || opts.To != "codex" {
		return Options{}, usageError{message: "agent-canon supports only claude -> codex", code: 1}
	}
	if opts.Format != "text" && opts.Format != "json" {
		return Options{}, usageError{message: "--format must be text or json", code: 1}
	}
	if flagWasSet(flags, "include-memory") && opts.Command != "scan" && opts.Command != "plan" && opts.Command != "import" && opts.Command != "sync" && opts.Command != "diff" && opts.Command != "verify" {
		return Options{}, usageError{message: "--include-memory is supported only for scan, plan, import, sync, diff, and verify", code: 1}
	}
	if opts.Command == "rollback" && opts.Format != "text" {
		return Options{}, usageError{message: "--format json is not supported for rollback", code: 1}
	}
	if opts.Command == "resolve" && flagWasSet(flags, "format") {
		return Options{}, usageError{message: "--format is not supported for resolve", code: 1}
	}
	if opts.Command != "plan" && opts.Command != "export" && opts.Command != "compile" && opts.Out != "" {
		return Options{}, usageError{message: "--out is supported only for plan, export codex, and compile codex", code: 1}
	}
	if opts.Command == "export" {
		if flagWasSet(flags, "format") {
			return Options{}, usageError{message: "--format is not supported for export codex", code: 1}
		}
		if opts.Out == "" {
			return Options{}, usageError{message: "export codex requires --out", code: 1}
		}
	}
	if opts.Command == "compile" {
		if flagWasSet(flags, "format") {
			return Options{}, usageError{message: "--format is not supported for compile codex in agent-canon", code: 1}
		}
		if opts.Out == "" {
			return Options{}, usageError{message: "compile codex requires --out", code: 1}
		}
	}
	if err := validateApplyFlags(opts.Command, flags); err != nil {
		return Options{}, err
	}
	if err := validateResolveDecision(opts.Command, flags, &opts); err != nil {
		return Options{}, err
	}

	project, err := cleanExistingDir(opts.Project, "--project")
	if err != nil {
		return Options{}, err
	}
	opts.Project = project

	claudeHomeExplicit := flagWasSet(flags, "claude-home")
	codexHomeExplicit := flagWasSet(flags, "codex-home")
	if opts.ClaudeHome, err = validateHome(opts.ClaudeHome, "--claude-home", claudeHomeExplicit); err != nil {
		return Options{}, err
	} else if !claudeHomeExplicit && !dirExists(opts.ClaudeHome) {
		opts.Warnings = append(opts.Warnings, fmt.Sprintf("default Claude home does not exist: %s", opts.ClaudeHome))
	}
	if opts.CodexHome, err = validateHome(opts.CodexHome, "--codex-home", codexHomeExplicit); err != nil {
		return Options{}, err
	} else if !codexHomeExplicit && !dirExists(opts.CodexHome) {
		opts.Warnings = append(opts.Warnings, fmt.Sprintf("default Codex home does not exist: %s", opts.CodexHome))
	}

	return opts, nil
}

func Run(args []string, cwd string, homeDir string, stdout io.Writer, stderr io.Writer) int {
	if err := RunE(args, cwd, homeDir, stdout, stderr); err != nil {
		if reportErr := writeLine(stderr, err.Error()); reportErr != nil {
			return 1
		}
		return ExitCode(err)
	}
	return 0
}

func RunE(args []string, cwd string, homeDir string, stdout io.Writer, stderr io.Writer) error {
	opts, err := Parse(args, cwd, homeDir)
	if err != nil {
		return err
	}
	if opts.Command == "help" {
		return writeOutput(stdout, helpText)
	}
	for _, warning := range opts.Warnings {
		if err := writeLine(stderr, "warning: %s", warning); err != nil {
			return err
		}
	}
	if err := writeLine(stdout, "agent-canon %s: %s -> %s", opts.Command, opts.From, opts.To); err != nil {
		return err
	}
	if err := writeLine(stdout, "Project: %s", opts.Project); err != nil {
		return err
	}
	return writeLine(stdout, "command execution is handled by the application layer.")
}

func writeOutput(writer io.Writer, text string) error {
	if _, err := fmt.Fprint(writer, text); err != nil {
		return usageError{message: fmt.Sprintf("write output: %v", err), code: 1}
	}
	return nil
}

func writeLine(writer io.Writer, format string, args ...any) error {
	if len(args) == 0 {
		return writeOutput(writer, format+"\n")
	}
	return writeOutput(writer, fmt.Sprintf(format+"\n", args...))
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	set := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func validateApplyFlags(command string, flags *flag.FlagSet) error {
	if command == "apply" || command == "rollback" {
		return nil
	}
	for _, name := range []string{"dry-run", "yes", "global"} {
		if flagWasSet(flags, name) {
			return usageError{message: fmt.Sprintf("--%s is supported only for apply codex or rollback", name), code: 1}
		}
	}
	return nil
}

func validateResolveDecision(command string, flags *flag.FlagSet, opts *Options) error {
	decisionCount := 0
	for _, decision := range []struct {
		flagName string
		value    string
	}{
		{flagName: "ours", value: "ours"},
		{flagName: "theirs", value: "theirs"},
		{flagName: "accept-suggestion", value: "accept-suggestion"},
		{flagName: "manual", value: "manual"},
	} {
		if flagWasSet(flags, decision.flagName) {
			decisionCount++
			opts.ResolveDecision = decision.value
		}
	}

	if command != "resolve" {
		if decisionCount > 0 {
			return usageError{message: "resolve decision flags are supported only for resolve", code: 1}
		}
		return nil
	}
	if decisionCount == 0 {
		return usageError{message: "resolve requires exactly one decision flag", code: 1}
	}
	if decisionCount > 1 {
		return usageError{message: "resolve requires exactly one decision flag", code: 1}
	}
	if opts.ResolveDecision == "manual" && opts.ManualValue == "" {
		return usageError{message: "--manual requires a non-empty value", code: 1}
	}
	return nil
}

func validateHome(path string, name string, explicit bool) (string, error) {
	cleaned := filepath.Clean(path)
	if explicit {
		return cleanExistingDir(cleaned, name)
	}
	return cleaned, nil
}

func cleanExistingDir(path string, name string) (string, error) {
	cleaned := filepath.Clean(path)
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", usageError{message: fmt.Sprintf("%s must exist: %s", name, cleaned), code: 1}
	}
	if !info.IsDir() {
		return "", usageError{message: fmt.Sprintf("%s must be a directory: %s", name, cleaned), code: 1}
	}
	return cleaned, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
