package apply_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestWriteCodexPlanBacksUpExistingFileBeforeModify(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, "AGENTS.md")
	writeFile(t, target, "old\n")

	result, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan: applypkg.CodexPlan{Changes: []applypkg.FileChange{{
			ApplyFileChange: model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionModify, BeforeHash: testHash("old\n"), AfterHash: testHash("new\n")},
			Contents:        []byte("new\n"),
		}}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err != nil {
		t.Fatalf("WriteCodexPlan returned error: %v", err)
	}

	assertFileContents(t, target, "new\n")
	backupPath := filepath.Join(backupDir, "project", "AGENTS.md")
	assertFileContents(t, backupPath, "old\n")
	if len(result.Changes) != 1 || result.Changes[0].BackupPath != backupPath || !result.Changes[0].Verified {
		t.Fatalf("result changes = %#v, want backup path and verified", result.Changes)
	}
	assertFileMode(t, target, 0o644)
	assertFileMode(t, backupPath, 0o644)
}

func TestWriteCodexPlanCreatesFileWithoutBackup(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, ".codex", "config.toml")

	result, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan: applypkg.CodexPlan{Changes: []applypkg.FileChange{{
			ApplyFileChange: model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("config\n")},
			Contents:        []byte("config\n"),
		}}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err != nil {
		t.Fatalf("WriteCodexPlan returned error: %v", err)
	}

	assertFileContents(t, target, "config\n")
	if result.Changes[0].BackupPath != "" || !result.Changes[0].Verified {
		t.Fatalf("result change = %#v, want no backup and verified", result.Changes[0])
	}
	assertPathMissing(t, filepath.Join(backupDir, "project", ".codex", "config.toml"))
}

func TestWriteCodexPlanSkipsNoopWrites(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, "AGENTS.md")
	writeFile(t, target, "same\n")

	result, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan: applypkg.CodexPlan{Changes: []applypkg.FileChange{{
			ApplyFileChange: model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionNoop, BeforeHash: testHash("same\n"), AfterHash: testHash("same\n")},
			Contents:        []byte("different but should not be written\n"),
		}}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err != nil {
		t.Fatalf("WriteCodexPlan returned error: %v", err)
	}

	assertFileContents(t, target, "same\n")
	if result.Changes[0].BackupPath != "" || !result.Changes[0].Verified {
		t.Fatalf("result change = %#v, want verified noop without backup", result.Changes[0])
	}
}

func TestWriteCodexPlanRejectsEscapingSymlinks(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	outside := t.TempDir()
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	target := filepath.Join(project, "AGENTS.md")
	outsideTarget := filepath.Join(outside, "AGENTS.md")
	if err := os.Symlink(outsideTarget, target); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan:      applypkg.CodexPlan{Changes: []applypkg.FileChange{{ApplyFileChange: model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("new\n")}, Contents: []byte("new\n")}}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err == nil {
		t.Fatalf("WriteCodexPlan returned nil error for escaping symlink")
	}
	if !strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "outside") {
		t.Fatalf("error missing symlink/outside context: %v", err)
	}
	assertPathMissing(t, outsideTarget)
}

func TestWriteCodexPlanRejectsParentSymlinkEscape(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	outside := t.TempDir()
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(project, ".codex")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	target := filepath.Join(project, ".codex", "config.toml")

	_, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan:      applypkg.CodexPlan{Changes: []applypkg.FileChange{{ApplyFileChange: model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("config\n")}, Contents: []byte("config\n")}}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err == nil {
		t.Fatalf("WriteCodexPlan returned nil error for escaping parent symlink")
	}
	assertPathMissing(t, filepath.Join(outside, "config.toml"))
}

func TestWriteCodexPlanRejectsInvalidTargetsBeforeWriting(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	validTarget := filepath.Join(project, "AGENTS.md")

	_, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan: applypkg.CodexPlan{Changes: []applypkg.FileChange{
			{ApplyFileChange: model.ApplyFileChange{Path: validTarget, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("valid\n")}, Contents: []byte("valid\n")},
			{ApplyFileChange: model.ApplyFileChange{Path: "relative/path", Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("bad\n")}, Contents: []byte("bad\n")},
		}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err == nil {
		t.Fatalf("WriteCodexPlan returned nil error for invalid target")
	}
	assertPathMissing(t, validTarget)
}

func TestWriteCodexPlanRejectsFileDirectoryConflictsBeforeWriting(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	parent := filepath.Join(project, "AGENTS.md")
	child := filepath.Join(parent, "child")

	_, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan: applypkg.CodexPlan{Changes: []applypkg.FileChange{
			{ApplyFileChange: model.ApplyFileChange{Path: parent, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("parent\n")}, Contents: []byte("parent\n")},
			{ApplyFileChange: model.ApplyFileChange{Path: child, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("child\n")}, Contents: []byte("child\n")},
		}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err == nil {
		t.Fatalf("WriteCodexPlan returned nil error for file/directory conflict")
	}
	assertPathMissing(t, parent)
}

func TestWriteCodexPlanRejectsHashMismatch(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, "AGENTS.md")

	_, err := applypkg.WriteCodexPlan(applypkg.WriteInput{
		Plan: applypkg.CodexPlan{Changes: []applypkg.FileChange{{
			ApplyFileChange: model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("different\n")},
			Contents:        []byte("actual\n"),
		}}},
		Project:   project,
		BackupDir: backupDir,
	})
	if err == nil {
		t.Fatalf("WriteCodexPlan returned nil error for hash mismatch")
	}
	if !strings.Contains(err.Error(), "hash") {
		t.Fatalf("error missing hash context: %v", err)
	}
}

func testHash(contents string) string {
	sum := sha256.Sum256([]byte(contents))
	return fmt.Sprintf("sha256:%x", sum)
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}
