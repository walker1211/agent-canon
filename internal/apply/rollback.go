package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

type RollbackInput struct {
	Manifest      model.RollbackManifestReport
	Project       string
	CodexHome     string
	ClaudeHome    string
	IncludeGlobal bool
	DryRun        bool
}

type RollbackResult struct {
	Changes []model.ApplyFileChange
}

type rollbackTarget struct {
	change         model.ApplyFileChange
	target         string
	backupContents []byte
}

func RollbackCodex(input RollbackInput) (RollbackResult, error) {
	targets, err := validateRollbackInput(input)
	if err != nil {
		return RollbackResult{}, err
	}

	changes := make([]model.ApplyFileChange, 0, len(targets))
	for _, target := range targets {
		change := target.change
		change.Verified = true
		if !input.DryRun {
			switch change.Action {
			case model.ApplyActionCreate:
				if err := os.Remove(target.target); err != nil {
					return RollbackResult{}, fmt.Errorf("delete rollback target %s: %w", target.target, err)
				}
			case model.ApplyActionModify:
				if err := writeTargetFile(target.target, target.backupContents); err != nil {
					return RollbackResult{}, err
				}
				if err := verifyFileHash(target.target, change.BeforeHash); err != nil {
					return RollbackResult{}, err
				}
			case model.ApplyActionNoop:
			default:
				return RollbackResult{}, fmt.Errorf("unsupported rollback action %q", change.Action)
			}
		}
		changes = append(changes, change)
	}
	return RollbackResult{Changes: changes}, nil
}

func validateRollbackInput(input RollbackInput) ([]rollbackTarget, error) {
	if input.Manifest.SchemaVersion != model.RollbackManifestSchemaVersion {
		return nil, fmt.Errorf("rollback manifest schema %q is not supported", input.Manifest.SchemaVersion)
	}
	if input.Manifest.Target != "codex" && input.Manifest.Target != "claude" {
		return nil, fmt.Errorf("rollback manifest target %q is not supported", input.Manifest.Target)
	}
	project, err := absCleanRequired(input.Project, "project path")
	if err != nil {
		return nil, err
	}
	manifestProject, err := absCleanRequired(input.Manifest.Project, "rollback manifest project")
	if err != nil {
		return nil, err
	}
	if manifestProject != project {
		return nil, fmt.Errorf("rollback manifest project %s does not match project %s", input.Manifest.Project, project)
	}

	targets := make([]rollbackTarget, 0, len(input.Manifest.Changes))
	writeTargets := make([]writeTarget, 0, len(input.Manifest.Changes))
	for _, change := range input.Manifest.Changes {
		root, err := rollbackRoot(input, change)
		if err != nil {
			return nil, err
		}
		requireTarget := change.Action == model.ApplyActionCreate || change.Action == model.ApplyActionModify
		target, err := validateRollbackTarget(root, change.Path, requireTarget)
		if err != nil {
			return nil, err
		}
		if err := validateRollbackTargetHash(change, target); err != nil {
			return nil, err
		}

		rollback := rollbackTarget{change: change, target: target}
		if change.Action == model.ApplyActionModify {
			contents, err := readRollbackBackup(input, change, project)
			if err != nil {
				return nil, err
			}
			rollback.backupContents = contents
		}
		targets = append(targets, rollback)
		writeTargets = append(writeTargets, writeTarget{target: target})
	}
	if err := validateWritePathConflicts(writeTargets); err != nil {
		return nil, err
	}
	return targets, nil
}

func rollbackRoot(input RollbackInput, change model.ApplyFileChange) (string, error) {
	switch change.Scope {
	case model.ScopeProject:
		if strings.TrimSpace(input.Project) == "" {
			return "", fmt.Errorf("project path is required for rollback target %s", change.Path)
		}
		return input.Project, nil
	case model.ScopeGlobal:
		if !input.IncludeGlobal {
			return "", fmt.Errorf("global rollback target %s requires --global", change.Path)
		}
		switch input.Manifest.Target {
		case "codex":
			if strings.TrimSpace(input.CodexHome) == "" {
				return "", fmt.Errorf("codex home is required for global rollback target %s", change.Path)
			}
			return input.CodexHome, nil
		case "claude":
			if strings.TrimSpace(input.ClaudeHome) == "" {
				return "", fmt.Errorf("claude home is required for global rollback target %s", change.Path)
			}
			return input.ClaudeHome, nil
		default:
			return "", fmt.Errorf("rollback manifest target %q is not supported", input.Manifest.Target)
		}
	default:
		return "", fmt.Errorf("unsupported rollback scope %q for %s", change.Scope, change.Path)
	}
}

func validateRollbackTarget(root string, target string, requireExists bool) (string, error) {
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("rollback target %q must be absolute", target)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve rollback root %s: %w", root, err)
	}
	rootAbs = filepath.Clean(rootAbs)
	target = filepath.Clean(target)
	inside, err := isInsidePath(rootAbs, target)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", fmt.Errorf("rollback target %s is outside root %s", target, rootAbs)
	}

	rootResolved, err := resolveRootBoundary(rootAbs)
	if err != nil {
		return "", err
	}
	resolved, err := resolveWriteBoundary(target)
	if err != nil {
		return "", err
	}
	inside, err = isInsidePath(rootResolved, resolved)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", fmt.Errorf("rollback target %s resolves outside root %s", target, rootAbs)
	}

	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("rollback target %s must not be a symlink", target)
		}
		if info.IsDir() {
			return "", fmt.Errorf("rollback target %s is a directory", target)
		}
		return target, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect rollback target %s: %w", target, err)
	}
	if requireExists {
		return "", fmt.Errorf("rollback target %s does not exist", target)
	}
	return target, nil
}

func validateRollbackTargetHash(change model.ApplyFileChange, target string) error {
	switch change.Action {
	case model.ApplyActionCreate, model.ApplyActionModify:
		if change.AfterHash == "" {
			return fmt.Errorf("rollback target %s missing after hash", target)
		}
		return verifyExistingHash(target, change.AfterHash)
	case model.ApplyActionNoop:
		info, err := os.Lstat(target)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect rollback target %s: %w", target, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("rollback target %s must not be a symlink", target)
		}
		if info.IsDir() {
			return fmt.Errorf("rollback target %s is a directory", target)
		}
		want := change.AfterHash
		if want == "" {
			want = change.BeforeHash
		}
		if want == "" {
			return fmt.Errorf("rollback target %s missing noop hash", target)
		}
		return verifyExistingHash(target, want)
	default:
		return fmt.Errorf("unsupported rollback action %q", change.Action)
	}
}

func readRollbackBackup(input RollbackInput, change model.ApplyFileChange, project string) ([]byte, error) {
	if change.BeforeHash == "" {
		return nil, fmt.Errorf("rollback backup for %s missing before hash", change.Path)
	}
	backupPath, err := validateRollbackBackupPath(input.Manifest.BackupDir, change.BackupPath, project)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, fmt.Errorf("read rollback backup %s: %w", backupPath, err)
	}
	if got := hashBytes(data); got != change.BeforeHash {
		return nil, fmt.Errorf("rollback backup %s hash mismatch: got %s want %s", backupPath, got, change.BeforeHash)
	}
	return data, nil
}

func validateRollbackBackupPath(backupDir string, backupPath string, project string) (string, error) {
	if strings.TrimSpace(backupDir) == "" {
		return "", fmt.Errorf("rollback backup directory is required")
	}
	if strings.TrimSpace(backupPath) == "" {
		return "", fmt.Errorf("rollback backup path is required")
	}
	backupDir, err := absCleanRequired(backupDir, "rollback backup directory")
	if err != nil {
		return "", err
	}
	backupPath, err = absCleanRequired(backupPath, "rollback backup path")
	if err != nil {
		return "", err
	}
	inside, err := isInsidePath(backupDir, backupPath)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", fmt.Errorf("rollback backup %s is outside backup directory %s", backupPath, backupDir)
	}
	projectResolved, err := resolveRootBoundary(project)
	if err != nil {
		return "", err
	}
	backupDirResolved, err := resolveWriteBoundary(backupDir)
	if err != nil {
		return "", err
	}
	inside, err = isInsidePath(projectResolved, backupDirResolved)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", fmt.Errorf("rollback backup directory %s resolves outside project %s", backupDir, project)
	}
	backupResolved, err := resolveWriteBoundary(backupPath)
	if err != nil {
		return "", err
	}
	inside, err = isInsidePath(backupDirResolved, backupResolved)
	if err != nil {
		return "", err
	}
	if !inside {
		return "", fmt.Errorf("rollback backup %s resolves outside backup directory %s", backupPath, backupDir)
	}
	info, err := os.Lstat(backupPath)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("rollback backup %s does not exist: %w", backupPath, err)
	}
	if err != nil {
		return "", fmt.Errorf("inspect rollback backup %s: %w", backupPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("rollback backup %s must not be a symlink", backupPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("rollback backup %s is a directory", backupPath)
	}
	return backupPath, nil
}

func absCleanRequired(path string, name string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s %s: %w", name, path, err)
	}
	return filepath.Clean(abs), nil
}
