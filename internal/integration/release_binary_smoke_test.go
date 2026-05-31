package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseBinarySmoke(t *testing.T) {
	repoRoot, err := filepath.Abs(publicReadinessRepoRoot())
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	binaryPath := packagedReleaseBinary(t, repoRoot)
	fixture := tempFixturePathsFor(t, "basic")
	isolatedHome := filepath.Join(t.TempDir(), "home")
	mustMkdir(t, isolatedHome)

	runReleaseBinarySmokeCommand(t, binaryPath, fixture.project, isolatedHome, "--help")
	runReleaseBinarySmokeCommand(t, binaryPath, fixture.project, isolatedHome, "scan", "--help")
	runReleaseBinarySmokeCommand(t, binaryPath, fixture.project, isolatedHome, "apply", "codex", "--help")
	runReleaseBinarySmokeCommand(t, binaryPath, fixture.project, isolatedHome, "scan", "--format", "json", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	runReleaseBinarySmokeCommand(t, binaryPath, fixture.project, isolatedHome, "sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)

	beforeDryRun := snapshotFiles(t, fixture.root)
	runReleaseBinarySmokeCommand(t, binaryPath, fixture.project, isolatedHome, "apply", "codex", "--dry-run", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeDryRun)
	assertPathMissing(t, filepath.Join(fixture.project, "AGENTS.md"))

	beforeVerify := snapshotFiles(t, fixture.root)
	output, err := runReleaseBinarySmokeCommandAllowingFailure(t, binaryPath, fixture.project, isolatedHome, "verify", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome)
	assertFilesUnchanged(t, fixture.root, beforeVerify)
	if err != nil && !strings.Contains(output, "agent-canon verify codex") {
		t.Fatalf("verify codex failed without controlled report: %v\n%s", err, output)
	}
}

func packagedReleaseBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	distDir := filepath.Join(t.TempDir(), "dist")
	isolatedHome := filepath.Join(t.TempDir(), "home")
	mustMkdir(t, isolatedHome)
	version := "v0.0.0-smoke"
	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "package-release.sh"), version)
	cmd.Dir = repoRoot
	cmd.Env = releaseBinarySmokeEnv(isolatedHome, "DIST_DIR="+distDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("package release failed: %v\n%s", err, string(output))
	}

	packageName := "agent-canon_" + version + "_" + runtime.GOOS + "_" + runtime.GOARCH
	archivePath := filepath.Join(distDir, packageName+".tar.gz")
	extractDir := filepath.Join(t.TempDir(), "extract")
	extractReleaseArchive(t, archivePath, extractDir)
	binaryName := "agent-canon"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(extractDir, packageName, binaryName)
	assertFileExists(t, binaryPath)
	return binaryPath
}

func extractReleaseArchive(t *testing.T, archivePath string, targetDir string) {
	t.Helper()
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("open gzip reader: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read tar entry: %v", err)
		}
		dest := filepath.Join(targetDir, filepath.Clean(header.Name))
		if !strings.HasPrefix(dest, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			t.Fatalf("archive entry escapes target directory: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, os.FileMode(header.Mode)); err != nil {
				t.Fatalf("mkdir archive directory %s: %v", dest, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				t.Fatalf("mkdir archive parent %s: %v", filepath.Dir(dest), err)
			}
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				t.Fatalf("create archive file %s: %v", dest, err)
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				t.Fatalf("extract archive file %s: %v", dest, copyErr)
			}
			if closeErr != nil {
				t.Fatalf("close archive file %s: %v", dest, closeErr)
			}
		default:
			t.Fatalf("unsupported archive entry %s type %c", header.Name, header.Typeflag)
		}
	}
}

func runReleaseBinarySmokeCommand(t *testing.T, binaryPath string, dir string, home string, args ...string) string {
	t.Helper()
	output, err := runReleaseBinarySmokeCommandAllowingFailure(t, binaryPath, dir, home, args...)
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", binaryPath, strings.Join(args, " "), err, output)
	}
	return output
}

func runReleaseBinarySmokeCommandAllowingFailure(t *testing.T, binaryPath string, dir string, home string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = releaseBinarySmokeEnv(home)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func releaseBinarySmokeEnv(home string, extra ...string) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "HOME="+home, "USERPROFILE="+home)
	env = append(env, extra...)
	return env
}
