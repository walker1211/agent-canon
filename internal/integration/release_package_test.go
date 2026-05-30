package integration_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestReleasePackageCreatesMinimalArchive(t *testing.T) {
	repoRoot, err := filepath.Abs(publicReadinessRepoRoot())
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	distDir := filepath.Join(t.TempDir(), "dist")
	version := "v0.0.0-test"
	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "package-release.sh"), version)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "DIST_DIR="+distDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("package release failed: %v\n%s", err, string(output))
	}

	entries, err := os.ReadDir(distDir)
	if err != nil {
		t.Fatalf("read dist directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("dist entry count = %d, want 1", len(entries))
	}
	packageName := "agent-canon_" + version + "_" + runtime.GOOS + "_" + runtime.GOARCH
	archivePath := filepath.Join(distDir, packageName+".tar.gz")
	if entries[0].Name() != filepath.Base(archivePath) {
		t.Fatalf("archive name = %q, want %q", entries[0].Name(), filepath.Base(archivePath))
	}

	files := releaseArchiveFiles(t, archivePath)
	binaryName := "agent-canon"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	want := []string{
		packageName + "/" + binaryName,
		packageName + "/LICENSE",
		packageName + "/README.md",
		packageName + "/README.en.md",
		packageName + "/README.zh-CN.md",
	}
	sort.Strings(want)
	if strings.Join(files, "\n") != strings.Join(want, "\n") {
		t.Fatalf("archive files = %#v, want %#v", files, want)
	}
	for _, file := range files {
		for _, forbidden := range []string{"/.git/", "/.github/", "/docs/", "/testdata/", "/dist/", "/build/", "/.env", "/.agent-canon/", ".log", ".db", ".sqlite"} {
			if strings.Contains(file, forbidden) {
				t.Fatalf("archive contains forbidden path %q in %q", forbidden, file)
			}
		}
	}
}

func TestSecretScanRedactsFindingsAndPassesCurrentRepo(t *testing.T) {
	repoRoot, err := filepath.Abs(publicReadinessRepoRoot())
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	secretRepo := t.TempDir()
	runCommand(t, secretRepo, "git", "init")
	secretValue := "gh" + "p_" + strings.Repeat("A", 36)
	if err := os.WriteFile(filepath.Join(secretRepo, "leak.txt"), []byte(secretValue+"\n"), 0o644); err != nil {
		t.Fatalf("write leak fixture: %v", err)
	}
	runCommand(t, secretRepo, "git", "add", "leak.txt")

	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "secret-scan.sh"), "--root", secretRepo)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("secret scan passed for leaked token; output=%q", string(output))
	}
	if strings.Contains(string(output), secretValue) {
		t.Fatalf("secret scan output leaked raw token: %q", string(output))
	}
	if !strings.Contains(string(output), "<REDACTED>") {
		t.Fatalf("secret scan output missing redaction marker: %q", string(output))
	}

	cmd = exec.Command(filepath.Join(repoRoot, "scripts", "secret-scan.sh"))
	cmd.Dir = repoRoot
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("secret scan failed on current repo: %v\n%s", err, string(output))
	}
}

func TestSecretScanHistoryModeGuardsShallowClones(t *testing.T) {
	repoRoot := publicReadinessRepoRoot()
	contents := readFileString(t, filepath.Join(repoRoot, "scripts", "secret-scan.sh"))
	if !strings.Contains(contents, "--is-shallow-repository") {
		t.Fatalf("secret-scan.sh must check whether history is shallow")
	}
	if !strings.Contains(contents, "--history") {
		t.Fatalf("secret-scan.sh must support history mode")
	}
}

func TestReleaseScriptsContracts(t *testing.T) {
	repoRoot, err := filepath.Abs(publicReadinessRepoRoot())
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	ciLocal := readFileString(t, filepath.Join(repoRoot, "scripts", "ci-local.sh"))
	for _, want := range []string{"clean", "git -C \"$work_dir\" init -q", "git -C \"$work_dir\" add -A", "scripts/secret-scan.sh", "go vet ./...", "go test ./...", "scripts/package-release.sh"} {
		if !strings.Contains(ciLocal, want) {
			t.Fatalf("ci-local.sh missing %q", want)
		}
	}
	buildScript := readFileString(t, filepath.Join(repoRoot, "build.sh"))
	for _, want := range []string{"#!/bin/sh", "go build -o agent-canon ./cmd/agent-canon"} {
		if !strings.Contains(buildScript, want) {
			t.Fatalf("build.sh missing %q", want)
		}
	}
	tagRelease := readFileString(t, filepath.Join(repoRoot, "scripts", "tag-release.sh"))
	for _, want := range []string{"status --porcelain", "scripts/ci-local.sh clean", "git tag", "git push origin"} {
		if !strings.Contains(tagRelease, want) {
			t.Fatalf("tag-release.sh missing %q", want)
		}
	}
	cmd := exec.Command(filepath.Join(repoRoot, "scripts", "tag-release.sh"), "1.0.0")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("tag-release accepted non-v tag; output=%q", string(output))
	}
	if !strings.Contains(string(output), "v") {
		t.Fatalf("tag-release invalid-tag output missing v guidance: %q", string(output))
	}
}

func runCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(output))
	}
}

func releaseArchiveFiles(t *testing.T, archivePath string) []string {
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
	var files []string
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read tar entry: %v", err)
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		files = append(files, header.Name)
	}
	sort.Strings(files)
	return files
}
