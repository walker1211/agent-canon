package main

import (
	"os"

	"github.com/zhangyoujun/agent-canon/internal/app"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	os.Exit(app.RunWithIO(os.Args[1:], cwd, homeDir, os.Stdin, os.Stdout, os.Stderr))
}
