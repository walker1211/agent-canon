package app

import (
	"errors"
	"fmt"
)

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string {
	return e.err.Error()
}

func (e exitError) Unwrap() error {
	return e.err
}

func withExitCode(code int, format string, args ...any) error {
	return exitError{code: code, err: fmt.Errorf(format, args...)}
}

func missingSyncStateError() error {
	return withExitCode(1, "no sync state found; run \"agent-canon sync claude codex\" first")
}

func openConflictBlockerError(workflow string, open int) error {
	return withExitCode(1, "%s blocked by %d open conflicts; run \"agent-canon conflicts\" to list them, then \"agent-canon resolve <conflict-id>\"", workflow, open)
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit exitError
	if errors.As(err, &exit) {
		return exit.code
	}
	return 1
}
