package apply_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestRollbackCodexDeletesCreatedTarget(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	target := filepath.Join(project, "AGENTS.md")
	writeFile(t, target, "new\n")
	manifest := rollbackManifest(project, "", model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("new\n")})

	result, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err != nil {
		t.Fatalf("RollbackCodex returned error: %v", err)
	}

	assertPathMissing(t, target)
	if len(result.Changes) != 1 || !result.Changes[0].Verified {
		t.Fatalf("result changes = %#v, want one verified change", result.Changes)
	}
}

func TestRollbackCodexRestoresModifiedTargetFromBackup(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, "AGENTS.md")
	backupPath := filepath.Join(backupDir, "project", "AGENTS.md")
	writeFile(t, target, "new\n")
	writeFile(t, backupPath, "old\n")
	manifest := rollbackManifest(project, backupDir, model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionModify, BackupPath: backupPath, BeforeHash: testHash("old\n"), AfterHash: testHash("new\n")})

	result, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err != nil {
		t.Fatalf("RollbackCodex returned error: %v", err)
	}

	assertFileContents(t, target, "old\n")
	assertFileMode(t, target, 0o644)
	if len(result.Changes) != 1 || !result.Changes[0].Verified {
		t.Fatalf("result changes = %#v, want one verified change", result.Changes)
	}
}

func TestRollbackCodexDryRunWritesNothing(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	target := filepath.Join(project, "AGENTS.md")
	writeFile(t, target, "new\n")
	manifest := rollbackManifest(project, "", model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("new\n")})

	result, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project, DryRun: true})
	if err != nil {
		t.Fatalf("RollbackCodex returned error: %v", err)
	}

	assertFileContents(t, target, "new\n")
	if len(result.Changes) != 1 || !result.Changes[0].Verified {
		t.Fatalf("result changes = %#v, want one verified planned change", result.Changes)
	}
}

func TestRollbackCodexRejectsTargetDriftBeforeWriting(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	target := filepath.Join(project, "AGENTS.md")
	writeFile(t, target, "drift\n")
	manifest := rollbackManifest(project, "", model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("new\n")})

	_, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error for target drift")
	}
	if !strings.Contains(err.Error(), "hash") {
		t.Fatalf("error missing hash context: %v", err)
	}
	assertFileContents(t, target, "drift\n")
}

func TestRollbackCodexRejectsMissingBackupBeforeWriting(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, "AGENTS.md")
	backupPath := filepath.Join(backupDir, "project", "AGENTS.md")
	writeFile(t, target, "new\n")
	manifest := rollbackManifest(project, backupDir, model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionModify, BackupPath: backupPath, BeforeHash: testHash("old\n"), AfterHash: testHash("new\n")})

	_, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error for missing backup")
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Fatalf("error missing backup context: %v", err)
	}
	assertFileContents(t, target, "new\n")
}

func TestRollbackCodexRequiresGlobalFlagAndCodexHomeBoundary(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	codexHome := filepath.Join(root, "codex-home")
	target := filepath.Join(codexHome, "AGENTS.md")
	writeFile(t, target, "global\n")
	manifest := rollbackManifest(project, "", model.ApplyFileChange{Path: target, Scope: model.ScopeGlobal, Action: model.ApplyActionCreate, AfterHash: testHash("global\n")})

	_, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project, CodexHome: codexHome})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error without IncludeGlobal")
	}
	if !strings.Contains(err.Error(), "global") {
		t.Fatalf("error missing global context: %v", err)
	}
	assertFileContents(t, target, "global\n")

	_, err = applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project, CodexHome: filepath.Join(root, "other-codex-home"), IncludeGlobal: true})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error for global target outside Codex home")
	}
	assertFileContents(t, target, "global\n")

	_, err = applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project, CodexHome: codexHome, IncludeGlobal: true})
	if err != nil {
		t.Fatalf("RollbackCodex returned error with IncludeGlobal: %v", err)
	}
	assertPathMissing(t, target)
}

func TestRollbackCodexRejectsSymlinkTarget(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	outside := t.TempDir()
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	outsideTarget := filepath.Join(outside, "AGENTS.md")
	writeFile(t, outsideTarget, "outside\n")
	target := filepath.Join(project, "AGENTS.md")
	if err := os.Symlink(outsideTarget, target); err != nil {
		t.Fatalf("create target symlink: %v", err)
	}
	manifest := rollbackManifest(project, "", model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("outside\n")})

	_, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error for symlink target")
	}
	if !strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "outside") {
		t.Fatalf("error missing symlink/outside context: %v", err)
	}
	assertFileContents(t, outsideTarget, "outside\n")
}

func TestRollbackCodexRejectsSymlinkBackup(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	outside := t.TempDir()
	backupDir := filepath.Join(project, ".agent-canon", "backups", "apply-001")
	target := filepath.Join(project, "AGENTS.md")
	backupPath := filepath.Join(backupDir, "project", "AGENTS.md")
	writeFile(t, target, "new\n")
	outsideBackup := filepath.Join(outside, "AGENTS.md")
	writeFile(t, outsideBackup, "old\n")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("mkdir backup parent: %v", err)
	}
	if err := os.Symlink(outsideBackup, backupPath); err != nil {
		t.Fatalf("create backup symlink: %v", err)
	}
	manifest := rollbackManifest(project, backupDir, model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionModify, BackupPath: backupPath, BeforeHash: testHash("old\n"), AfterHash: testHash("new\n")})

	_, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error for symlink backup")
	}
	if !strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "outside") {
		t.Fatalf("error missing symlink/outside context: %v", err)
	}
	assertFileContents(t, target, "new\n")
	assertFileContents(t, outsideBackup, "old\n")
}

func TestRollbackCodexRejectsDuplicateTargetsBeforeWriting(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	target := filepath.Join(project, "AGENTS.md")
	writeFile(t, target, "new\n")
	change := model.ApplyFileChange{Path: target, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: testHash("new\n")}
	manifest := rollbackManifest(project, "", change, change)

	_, err := applypkg.RollbackCodex(applypkg.RollbackInput{Manifest: manifest, Project: project})
	if err == nil {
		t.Fatalf("RollbackCodex returned nil error for duplicate targets")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("error missing duplicate context: %v", err)
	}
	assertFileContents(t, target, "new\n")
}

func rollbackManifest(project string, backupDir string, changes ...model.ApplyFileChange) model.RollbackManifestReport {
	return model.RollbackManifestReport{
		SchemaVersion: model.RollbackManifestSchemaVersion,
		Project:       project,
		Target:        "codex",
		BackupDir:     backupDir,
		Changes:       changes,
	}
}
