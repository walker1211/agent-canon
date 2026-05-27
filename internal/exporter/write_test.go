package exporter_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/exporter"
)

func TestWritePreviewCreatesNonexistentOutputDirAndWritesFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "preview", "out")
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "AGENTS.md", Contents: []byte("agents\n")},
		{Path: ".codex/config.toml", Contents: []byte("config\n")},
	}}

	if err := exporter.WritePreview(root, preview); err != nil {
		t.Fatalf("WritePreview returned error: %v", err)
	}

	assertFileContents(t, filepath.Join(root, "AGENTS.md"), "agents\n")
	assertFileContents(t, filepath.Join(root, ".codex", "config.toml"), "config\n")
}

func TestWritePreviewAcceptsExistingEmptyOutputDir(t *testing.T) {
	root := t.TempDir()
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "AGENTS.md", Contents: []byte("agents\n")},
	}}

	if err := exporter.WritePreview(root, preview); err != nil {
		t.Fatalf("WritePreview returned error: %v", err)
	}

	assertFileContents(t, filepath.Join(root, "AGENTS.md"), "agents\n")
}

func TestWritePreviewRejectsExistingNonEmptyOutputDirAndLeavesFileUnchanged(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(existing, []byte("keep\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "AGENTS.md", Contents: []byte("agents\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for non-empty output dir")
	}
	if !strings.Contains(err.Error(), root) || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error missing root/action context: %v", err)
	}

	assertFileContents(t, existing, "keep\n")
	assertPathMissing(t, filepath.Join(root, "AGENTS.md"))
}

func TestWritePreviewRejectsAbsolutePreviewPathBeforeWriting(t *testing.T) {
	root := t.TempDir()
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: filepath.Join(root, "evil.txt"), Contents: []byte("evil\n")},
		{Path: "AGENTS.md", Contents: []byte("agents\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for absolute preview path")
	}
	if !strings.Contains(err.Error(), "absolute") || !strings.Contains(err.Error(), "evil.txt") {
		t.Fatalf("error missing path/action context: %v", err)
	}

	assertPathMissing(t, filepath.Join(root, "evil.txt"))
	assertPathMissing(t, filepath.Join(root, "AGENTS.md"))
}

func TestWritePreviewRejectsParentTraversalPreviewPathBeforeWriting(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "out")
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "../evil", Contents: []byte("evil\n")},
		{Path: "AGENTS.md", Contents: []byte("agents\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for parent traversal preview path")
	}
	if !strings.Contains(err.Error(), "..") || !strings.Contains(err.Error(), "../evil") {
		t.Fatalf("error missing path/action context: %v", err)
	}

	assertPathMissing(t, filepath.Join(parent, "evil"))
	assertPathMissing(t, filepath.Join(root, "AGENTS.md"))
}

func TestWritePreviewRejectsCleanInsideParentTraversalBeforeWriting(t *testing.T) {
	root := t.TempDir()
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: ".codex/../AGENTS.md", Contents: []byte("agents\n")},
		{Path: ".codex/config.toml", Contents: []byte("config\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for preview path containing parent traversal")
	}
	if !strings.Contains(err.Error(), "..") || !strings.Contains(err.Error(), ".codex/../AGENTS.md") {
		t.Fatalf("error missing path/action context: %v", err)
	}

	assertPathMissing(t, filepath.Join(root, "AGENTS.md"))
	assertPathMissing(t, filepath.Join(root, ".codex", "config.toml"))
}

func TestWritePreviewRejectsDuplicatePreviewPathsBeforeWriting(t *testing.T) {
	root := t.TempDir()
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "AGENTS.md", Contents: []byte("first\n")},
		{Path: "./AGENTS.md", Contents: []byte("second\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for duplicate preview paths")
	}
	if !strings.Contains(err.Error(), "duplicate") || !strings.Contains(err.Error(), "AGENTS.md") {
		t.Fatalf("error missing path/action context: %v", err)
	}

	assertPathMissing(t, filepath.Join(root, "AGENTS.md"))
}

func TestWritePreviewRejectsEmptyRootBeforeWritingToCurrentDirectory(t *testing.T) {
	workdir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(workdir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "AGENTS.md", Contents: []byte("agents\n")},
	}}

	err = exporter.WritePreview("", preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for empty root")
	}
	if !strings.Contains(err.Error(), "root") || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("error missing root/action context: %v", err)
	}

	assertPathMissing(t, filepath.Join(workdir, "AGENTS.md"))
}

func TestWritePreviewRejectsFileDirectoryConflictBeforeWriting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "a", Contents: []byte("file\n")},
		{Path: "a/b", Contents: []byte("nested\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for file/directory conflict")
	}
	if !strings.Contains(err.Error(), "conflict") || !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "a/b") {
		t.Fatalf("error missing path/action context: %v", err)
	}

	assertRootMissingOrEmpty(t, root)
}

func TestWritePreviewRejectsReverseFileDirectoryConflictBeforeWriting(t *testing.T) {
	root := filepath.Join(t.TempDir(), "out")
	preview := exporter.CodexPreview{Files: []exporter.PreviewFile{
		{Path: "a/b", Contents: []byte("nested\n")},
		{Path: "a", Contents: []byte("file\n")},
	}}

	err := exporter.WritePreview(root, preview)
	if err == nil {
		t.Fatalf("WritePreview returned nil error for reverse file/directory conflict")
	}
	if !strings.Contains(err.Error(), "conflict") || !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "a/b") {
		t.Fatalf("error missing path/action context: %v", err)
	}

	assertRootMissingOrEmpty(t, root)
}

func assertFileContents(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s contents = %q, want %q", path, string(got), want)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s exists unexpectedly", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertRootMissingOrEmpty(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	if len(entries) > 0 {
		t.Fatalf("%s has unexpected entries after failed write: %v", root, entries)
	}
}
