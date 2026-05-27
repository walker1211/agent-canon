package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/cli"
	"github.com/zhangyoujun/agent-canon/internal/exporter"
	"github.com/zhangyoujun/agent-canon/internal/planner"
	"github.com/zhangyoujun/agent-canon/internal/scanner"
)

func runExport(opts cli.Options, stdout io.Writer) error {
	if err := validateExportOutputRoot(opts); err != nil {
		return withExitCode(1, "%w", err)
	}
	scanReport, err := scanner.Scan(scanner.Options{Project: opts.Project, ClaudeHome: opts.ClaudeHome, CodexHome: opts.CodexHome, IncludeMemory: opts.IncludeMemory})
	if err != nil {
		return mapScanError(err)
	}
	planReport := planner.Build(scanReport)
	preview, err := exporter.BuildCodexPreview(scanReport, planReport)
	if err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := exporter.WritePreview(opts.Out, preview); err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := writeLine(stdout, "agent-canon export: %s -> %s", planReport.Source, planReport.Target); err != nil {
		return withExitCode(1, "%w", err)
	}
	if err := writeLine(stdout, "Project: %s", planReport.Project); err != nil {
		return withExitCode(1, "%w", err)
	}
	return writeLine(stdout, "wrote Codex preview to %s (%d files)", opts.Out, len(preview.Files))
}

func validateExportOutputRoot(opts cli.Options) error {
	if strings.TrimSpace(opts.Out) == "" {
		return nil
	}
	out, err := cleanAbsPath(opts.Out)
	if err != nil {
		return err
	}
	for _, home := range []struct {
		name string
		path string
	}{
		{name: "Claude home", path: opts.ClaudeHome},
		{name: "Codex home", path: opts.CodexHome},
	} {
		if strings.TrimSpace(home.path) == "" {
			continue
		}
		root, err := cleanAbsPath(home.path)
		if err != nil {
			return err
		}
		if sameOrInside(out, root) {
			return fmt.Errorf("preview output %q must not be inside Claude or Codex home (%s: %q)", opts.Out, home.name, home.path)
		}
	}
	return nil
}

func cleanAbsPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	return resolveSymlinkBoundary(filepath.Clean(abs))
}

func resolveSymlinkBoundary(path string) (string, error) {
	current := path
	var missing []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missing[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("resolve path %q: %w", path, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve path %q: %w", path, err)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func sameOrInside(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}
