package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const helpText = `agent-canon is an agent-canon read-only migration inventory and plan tool.

agent-canon is read-only: it never writes Claude or Codex configuration directories.
The only supported write boundary is that only plan --out writes a JSON plan file.

Usage:
  agent-canon scan [flags]
  agent-canon plan [flags]

Commands:
  scan    Read-only inventory of Claude and Codex resources
  plan    Read-only migration plan generation

Flags:
  --from string          source tool; currently accepts only claude (default "claude")
  --to string            target tool; currently accepts only codex (default "codex")
  --project string       project directory (default current working directory)
  --claude-home string   Claude Code home (default ~/.claude)
  --codex-home string    Codex home (default ~/.codex)
  --format string        output format: text or json (default "text")
  --out string           plan only: write JSON plan to this path
  --include-memory       scan memory indexes and candidates only; does not migrate content
`

type Options struct {
	Command       string
	From          string
	To            string
	Project       string
	ClaudeHome    string
	CodexHome     string
	Format        string
	Out           string
	IncludeMemory bool
	Warnings      []string
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
	if command != "scan" && command != "plan" {
		return Options{}, usageError{message: fmt.Sprintf("unknown command %q", command), code: 1}
	}

	defaults := Options{
		Command:    command,
		From:       "claude",
		To:         "codex",
		Project:    cwd,
		ClaudeHome: filepath.Join(homeDir, ".claude"),
		CodexHome:  filepath.Join(homeDir, ".codex"),
		Format:     "text",
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
	if err := flags.Parse(args[1:]); err != nil {
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
	if opts.Command == "scan" && opts.Out != "" {
		return Options{}, usageError{message: "--out is supported only for plan", code: 1}
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
