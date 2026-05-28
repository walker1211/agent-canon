package app

import (
	"errors"
	"fmt"
	"io"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/render"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
)

func Run(args []string, cwd string, homeDir string, stdout io.Writer, stderr io.Writer) int {
	return RunWithIO(args, cwd, homeDir, nil, stdout, stderr)
}

func RunWithIO(args []string, cwd string, homeDir string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if err := RunEWithIO(args, cwd, homeDir, stdin, stdout, stderr); err != nil {
		if reportErr := writeLine(stderr, err.Error()); reportErr != nil {
			return 1
		}
		return exitCode(err)
	}
	return 0
}

func RunE(args []string, cwd string, homeDir string, stdout io.Writer, stderr io.Writer) error {
	return RunEWithIO(args, cwd, homeDir, nil, stdout, stderr)
}

func RunEWithIO(args []string, cwd string, homeDir string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	_ = stdin
	opts, err := cli.Parse(args, cwd, homeDir)
	if err != nil {
		return withExitCode(cli.ExitCode(err), "%w", err)
	}
	if opts.Command == "help" {
		return cli.RunE(args, cwd, homeDir, stdout, stderr)
	}
	for _, warning := range opts.Warnings {
		if err := writeLine(stderr, "warning: %s", warning); err != nil {
			return withExitCode(1, "%w", err)
		}
	}
	if opts.Command == "init" {
		return runInit(opts, stdout)
	}
	if opts.Command == "scan" {
		return runScan(opts, stdout)
	}
	if opts.Command == "status" {
		return runStatus(opts, stdout)
	}
	if opts.Command == "diff" {
		return runDiff(opts, stdout)
	}
	if opts.Command == "plan" {
		return runPlan(opts, stdout)
	}
	if opts.Command == "export" {
		return runExport(opts, stdout)
	}
	if opts.Command == "import" {
		return runImport(opts, stdout)
	}
	if opts.Command == "sync" {
		return runSync(opts, stdout)
	}
	if opts.Command == "conflicts" {
		return runConflicts(opts, stdout)
	}
	if opts.Command == "resolve" {
		return runResolve(opts, stdout)
	}
	if opts.Command == "apply" {
		return runApply(opts, stdin, stdout)
	}
	if opts.Command == "rollback" {
		return runRollback(opts, stdin, stdout)
	}
	if opts.Command == "verify" {
		return runVerify(opts, stdout)
	}
	return withExitCode(1, "unknown command %q", opts.Command)
}

func runScan(opts cli.Options, stdout io.Writer) error {
	report, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	if opts.Format == "json" {
		if err := render.ScanJSON(stdout, report); err != nil {
			return withExitCode(1, "%w", err)
		}
		return nil
	}
	if err := render.ScanText(stdout, report); err != nil {
		return withExitCode(1, "%w", err)
	}
	return nil
}

func mapScanError(err error) error {
	var parseErr scanner.ParseError
	if errors.As(err, &parseErr) {
		return withExitCode(2, "%w", err)
	}
	return withExitCode(1, "%w", err)
}

func writeLine(writer io.Writer, format string, args ...any) error {
	line := format + "\n"
	if len(args) > 0 {
		line = fmt.Sprintf(line, args...)
	}
	if _, err := io.WriteString(writer, line); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
