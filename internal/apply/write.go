package apply

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/codexpath"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

const (
	applyDirMode  os.FileMode = 0o755
	applyFileMode os.FileMode = 0o644
)

type WriteInput struct {
	Plan      CodexPlan
	Project   string
	CodexHome string
	BackupDir string
}

type WriteClaudeInput struct {
	Plan       ClaudePlan
	Project    string
	ClaudeHome string
	BackupDir  string
}

type WriteResult struct {
	Changes []model.ApplyFileChange
}

type writeTarget struct {
	change       FileChange
	root         string
	backupPrefix string
	target       string
	rel          string
}

func WriteCodexPlan(input WriteInput) (WriteResult, error) {
	targets, err := validateWriteTargets(input.Plan.Changes, input.BackupDir, func(change FileChange) (string, string, error) {
		return rootForChange(input, change)
	})
	if err != nil {
		return WriteResult{}, err
	}
	return writeTargets(targets, input.BackupDir)
}

func WriteClaudePlan(input WriteClaudeInput) (WriteResult, error) {
	targets, err := validateWriteTargets(input.Plan.Changes, input.BackupDir, func(change FileChange) (string, string, error) {
		return rootForClaudeChange(input, change)
	})
	if err != nil {
		return WriteResult{}, err
	}
	return writeTargets(targets, input.BackupDir)
}

type writeRootResolver func(FileChange) (string, string, error)

func writeTargets(targets []writeTarget, backupDir string) (WriteResult, error) {
	changes := make([]model.ApplyFileChange, 0, len(targets))
	for _, target := range targets {
		change := target.change.ApplyFileChange
		switch target.change.Action {
		case model.ApplyActionNoop:
			if err := verifyExistingHash(target.target, expectedNoopHash(target.change)); err != nil {
				return WriteResult{}, err
			}
			change.Verified = true
		case model.ApplyActionCreate:
			if err := writeTargetFile(target.target, target.change.Contents); err != nil {
				return WriteResult{}, err
			}
			if err := verifyFileHash(target.target, target.change.AfterHash); err != nil {
				return WriteResult{}, err
			}
			change.Verified = true
		case model.ApplyActionModify:
			current, err := os.ReadFile(target.target)
			if err != nil {
				return WriteResult{}, fmt.Errorf("read apply target %s: %w", target.target, err)
			}
			if target.change.BeforeHash != "" && hashBytes(current) != target.change.BeforeHash {
				return WriteResult{}, fmt.Errorf("apply target %s before hash mismatch", target.target)
			}
			backupPath := filepath.Join(backupDir, target.backupPrefix, target.rel)
			if err := writeBackupFile(backupPath, current); err != nil {
				return WriteResult{}, err
			}
			if err := writeTargetFile(target.target, target.change.Contents); err != nil {
				return WriteResult{}, err
			}
			if err := verifyFileHash(target.target, target.change.AfterHash); err != nil {
				return WriteResult{}, err
			}
			change.BackupPath = backupPath
			change.Verified = true
		default:
			return WriteResult{}, fmt.Errorf("unsupported apply action %q", target.change.Action)
		}
		changes = append(changes, change)
	}
	return WriteResult{Changes: changes}, nil
}

func validateWriteTargets(changes []FileChange, backupDir string, rootFor writeRootResolver) ([]writeTarget, error) {
	targets := make([]writeTarget, 0, len(changes))
	for _, change := range changes {
		if err := validatePlannedHash(change); err != nil {
			return nil, err
		}
		root, prefix, err := rootFor(change)
		if err != nil {
			return nil, err
		}
		target, rel, err := validateApplyTarget(root, change.Path, change.Action)
		if err != nil {
			return nil, err
		}
		if change.Action == model.ApplyActionModify && strings.TrimSpace(backupDir) == "" {
			return nil, fmt.Errorf("backup directory is required for modify apply target %s", change.Path)
		}
		targets = append(targets, writeTarget{change: change, root: root, backupPrefix: prefix, target: target, rel: rel})
	}
	if err := validateWritePathConflicts(targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func validatePlannedHash(change FileChange) error {
	switch change.Action {
	case model.ApplyActionCreate, model.ApplyActionModify:
		if change.AfterHash == "" {
			return fmt.Errorf("apply target %s missing after hash", change.Path)
		}
		if got := hashBytes(change.Contents); got != change.AfterHash {
			return fmt.Errorf("apply target %s content hash mismatch: got %s want %s", change.Path, got, change.AfterHash)
		}
	case model.ApplyActionNoop:
		if expectedNoopHash(change) == "" {
			return fmt.Errorf("apply target %s missing noop hash", change.Path)
		}
	default:
		return fmt.Errorf("unsupported apply action %q", change.Action)
	}
	return nil
}

func rootForChange(input WriteInput, change FileChange) (string, string, error) {
	switch change.Scope {
	case model.ScopeProject:
		if strings.TrimSpace(input.Project) == "" {
			return "", "", fmt.Errorf("project path is required for apply target %s", change.Path)
		}
		return input.Project, "project", nil
	case model.ScopeGlobal:
		if strings.TrimSpace(input.CodexHome) == "" {
			return "", "", fmt.Errorf("codex home is required for global apply target %s", change.Path)
		}
		if inside, err := isInsidePath(codexpath.UserSkillsRoot(input.CodexHome), change.Path); err != nil {
			return "", "", err
		} else if inside {
			return filepath.Dir(filepath.Clean(input.CodexHome)), "user-home", nil
		}
		return input.CodexHome, "codex-home", nil
	default:
		return "", "", fmt.Errorf("unsupported apply scope %q for %s", change.Scope, change.Path)
	}
}

func rootForClaudeChange(input WriteClaudeInput, change FileChange) (string, string, error) {
	switch change.Scope {
	case model.ScopeProject:
		if strings.TrimSpace(input.Project) == "" {
			return "", "", fmt.Errorf("project path is required for apply target %s", change.Path)
		}
		return input.Project, "project", nil
	case model.ScopeGlobal:
		if strings.TrimSpace(input.ClaudeHome) == "" {
			return "", "", fmt.Errorf("claude home is required for global apply target %s", change.Path)
		}
		return input.ClaudeHome, "claude-home", nil
	default:
		return "", "", fmt.Errorf("unsupported apply scope %q for %s", change.Scope, change.Path)
	}
}

func validateApplyTarget(root string, target string, action model.ApplyAction) (string, string, error) {
	if !filepath.IsAbs(target) {
		return "", "", fmt.Errorf("apply target %q must be absolute", target)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve apply root %s: %w", root, err)
	}
	rootAbs = filepath.Clean(rootAbs)
	target = filepath.Clean(target)
	inside, err := isInsidePath(rootAbs, target)
	if err != nil {
		return "", "", err
	}
	if !inside {
		return "", "", fmt.Errorf("apply target %s is outside root %s", target, rootAbs)
	}
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return "", "", fmt.Errorf("compare apply target %s to root %s: %w", target, rootAbs, err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", "", fmt.Errorf("apply target %s escapes root %s", target, rootAbs)
	}

	if action == model.ApplyActionNoop {
		info, err := os.Stat(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", "", fmt.Errorf("apply target %s does not exist", target)
			}
			return "", "", fmt.Errorf("inspect apply target %s: %w", target, err)
		}
		if info.IsDir() {
			return "", "", fmt.Errorf("apply target %s is a directory", target)
		}
		return target, rel, nil
	}

	rootResolved, err := resolveRootBoundary(rootAbs)
	if err != nil {
		return "", "", err
	}
	resolved, err := resolveWriteBoundary(target)
	if err != nil {
		return "", "", err
	}
	inside, err = isInsidePath(rootResolved, resolved)
	if err != nil {
		return "", "", err
	}
	if !inside {
		return "", "", fmt.Errorf("apply target %s resolves outside root %s", target, rootAbs)
	}

	info, err := os.Lstat(target)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("apply target %s must not be a symlink", target)
		}
		if info.IsDir() {
			return "", "", fmt.Errorf("apply target %s is a directory", target)
		}
		if action == model.ApplyActionCreate {
			return "", "", fmt.Errorf("apply target %s already exists", target)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect apply target %s: %w", target, err)
	} else if action == model.ApplyActionModify || action == model.ApplyActionNoop {
		return "", "", fmt.Errorf("apply target %s does not exist", target)
	}
	return target, rel, nil
}

func resolveRootBoundary(root string) (string, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err == nil {
		return filepath.Clean(resolved), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return resolveWriteBoundary(root)
	}
	return "", fmt.Errorf("resolve apply root %s: %w", root, err)
}

func resolveWriteBoundary(path string) (string, error) {
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
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("resolve apply target %s: %w", path, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve apply target %s: %w", path, err)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func validateWritePathConflicts(targets []writeTarget) error {
	paths := make([]string, 0, len(targets))
	seen := map[string]bool{}
	for _, target := range targets {
		slashPath := filepath.ToSlash(target.target)
		if seen[slashPath] {
			return fmt.Errorf("duplicate apply target %q", target.target)
		}
		seen[slashPath] = true
		paths = append(paths, slashPath)
	}
	sort.Strings(paths)
	for i := 1; i < len(paths); i++ {
		parent := paths[i-1]
		child := paths[i]
		if strings.HasPrefix(child, parent+"/") {
			return fmt.Errorf("apply target conflict: %q conflicts with descendant %q", parent, child)
		}
	}
	return nil
}

func writeBackupFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), applyDirMode); err != nil {
		return fmt.Errorf("create backup parent for %s: %w", path, err)
	}
	if err := os.WriteFile(path, contents, applyFileMode); err != nil {
		return fmt.Errorf("write backup file %s: %w", path, err)
	}
	if err := os.Chmod(path, applyFileMode); err != nil {
		return fmt.Errorf("set backup file mode %s: %w", path, err)
	}
	return nil
}

func writeTargetFile(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), applyDirMode); err != nil {
		return fmt.Errorf("create parent for apply target %s: %w", path, err)
	}
	if err := os.WriteFile(path, contents, applyFileMode); err != nil {
		return fmt.Errorf("write apply target %s: %w", path, err)
	}
	if err := os.Chmod(path, applyFileMode); err != nil {
		return fmt.Errorf("set apply target mode %s: %w", path, err)
	}
	return nil
}

func verifyExistingHash(path string, want string) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read apply target %s: %w", path, err)
	}
	if got := hashBytes(current); got != want {
		return fmt.Errorf("apply target %s hash mismatch: got %s want %s", path, got, want)
	}
	return nil
}

func verifyFileHash(path string, want string) error {
	if want == "" {
		return fmt.Errorf("apply target %s missing verification hash", path)
	}
	return verifyExistingHash(path, want)
}

func expectedNoopHash(change FileChange) string {
	if change.AfterHash != "" {
		return change.AfterHash
	}
	return change.BeforeHash
}

func isInsidePath(parent string, child string) (bool, error) {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if child == parent {
		return true, nil
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false, fmt.Errorf("compare path %s to root %s: %w", child, parent, err)
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".", nil
}
