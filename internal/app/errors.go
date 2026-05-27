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
