package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var ErrNotFound = errors.New("workspace state not found")

const (
	workspaceDirMode  os.FileMode = 0o755
	workspaceFileMode os.FileMode = 0o644
)

type Layout struct {
	Project            string
	Root               string
	BaseDir            string
	BaseClaude         string
	BaseCodex          string
	BaseCanon          string
	SyncState          string
	ResolutionsDir     string
	LearnedResolutions string
	BackupsDir         string
	RollbackDir        string

	paths layoutPaths
}

type layoutPaths struct {
	Project            string
	Root               string
	BaseDir            string
	BaseClaude         string
	BaseCodex          string
	BaseCanon          string
	SyncState          string
	ResolutionsDir     string
	LearnedResolutions string
	BackupsDir         string
	RollbackDir        string
}

func New(project string) (Layout, error) {
	paths, err := newLayoutPaths(project)
	if err != nil {
		return Layout{}, err
	}
	return Layout{
		Project:            paths.Project,
		Root:               paths.Root,
		BaseDir:            paths.BaseDir,
		BaseClaude:         paths.BaseClaude,
		BaseCodex:          paths.BaseCodex,
		BaseCanon:          paths.BaseCanon,
		SyncState:          paths.SyncState,
		ResolutionsDir:     paths.ResolutionsDir,
		LearnedResolutions: paths.LearnedResolutions,
		BackupsDir:         paths.BackupsDir,
		RollbackDir:        paths.RollbackDir,
		paths:              paths,
	}, nil
}

func newLayoutPaths(project string) (layoutPaths, error) {
	if project == "" {
		return layoutPaths{}, fmt.Errorf("project path is empty")
	}

	cleanProject, err := filepath.Abs(project)
	if err != nil {
		return layoutPaths{}, fmt.Errorf("resolve project path %q: %w", project, err)
	}
	cleanProject = filepath.Clean(cleanProject)

	root := filepath.Join(cleanProject, ".agent-canon")
	base := filepath.Join(root, "base")
	resolutions := filepath.Join(root, "resolutions")
	backups := filepath.Join(root, "backups")
	rollback := filepath.Join(root, "rollback")
	return layoutPaths{
		Project:            cleanProject,
		Root:               root,
		BaseDir:            base,
		BaseClaude:         filepath.Join(base, "claude.snapshot.json"),
		BaseCodex:          filepath.Join(base, "codex.snapshot.json"),
		BaseCanon:          filepath.Join(base, "canon.snapshot.json"),
		SyncState:          filepath.Join(root, "sync-state.json"),
		ResolutionsDir:     resolutions,
		LearnedResolutions: filepath.Join(resolutions, "learned-resolutions.json"),
		BackupsDir:         backups,
		RollbackDir:        rollback,
	}, nil
}

func (l Layout) SaveBaseClaude(value any) error {
	return l.writeJSON(l.BaseClaude, l.paths.BaseClaude, value)
}

func (l Layout) SaveBaseCodex(value any) error {
	return l.writeJSON(l.BaseCodex, l.paths.BaseCodex, value)
}

func (l Layout) SaveBaseCanon(value any) error {
	return l.writeJSON(l.BaseCanon, l.paths.BaseCanon, value)
}

func (l Layout) LoadBaseClaude(dest any) error {
	return l.readJSON(l.BaseClaude, l.paths.BaseClaude, dest)
}

func (l Layout) LoadBaseCodex(dest any) error {
	return l.readJSON(l.BaseCodex, l.paths.BaseCodex, dest)
}

func (l Layout) LoadBaseCanon(dest any) error {
	return l.readJSON(l.BaseCanon, l.paths.BaseCanon, dest)
}

func (l Layout) SaveSyncState(value any) error {
	return l.writeJSON(l.SyncState, l.paths.SyncState, value)
}

func (l Layout) LoadSyncState(dest any) error {
	return l.readJSON(l.SyncState, l.paths.SyncState, dest)
}

func (l Layout) SaveLearnedResolutions(value any) error {
	return l.writeJSON(l.LearnedResolutions, l.paths.LearnedResolutions, value)
}

func (l Layout) LoadLearnedResolutions(dest any) error {
	return l.readJSON(l.LearnedResolutions, l.paths.LearnedResolutions, dest)
}

func (l Layout) BackupDir(name string) (string, error) {
	if err := validateWorkspaceName(name); err != nil {
		return "", err
	}
	known, err := l.canonicalPaths()
	if err != nil {
		return "", err
	}
	path := filepath.Join(known.BackupsDir, name)
	if err := l.ensurePathInsideProject(path); err != nil {
		return "", err
	}
	return path, nil
}

func (l Layout) RollbackManifestPath(name string) (string, error) {
	if err := validateWorkspaceName(name); err != nil {
		return "", err
	}
	known, err := l.canonicalPaths()
	if err != nil {
		return "", err
	}
	path := filepath.Join(known.RollbackDir, name+".json")
	if err := l.ensurePathInsideProject(path); err != nil {
		return "", err
	}
	return path, nil
}

func (l Layout) SaveRollbackManifest(name string, value any) (string, error) {
	path, err := l.RollbackManifestPath(name)
	if err != nil {
		return "", err
	}
	return path, l.writeJSONFile(path, value)
}

func (l Layout) writeJSON(path string, canonicalPath string, value any) error {
	if err := l.validateKnownPath(path, canonicalPath); err != nil {
		return err
	}
	return l.writeJSONFile(canonicalPath, value)
}

func (l Layout) writeJSONFile(path string, value any) error {
	if err := l.ensurePathInsideProject(path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workspace JSON %s: %w", path, err)
	}
	data = append(data, '\n')

	if err := l.ensureWorkspaceDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("workspace file %s must not be a symlink", path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect workspace file %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, workspaceFileMode); err != nil {
		return fmt.Errorf("write workspace file %s: %w", path, err)
	}
	if err := os.Chmod(path, workspaceFileMode); err != nil {
		return fmt.Errorf("set workspace file mode %s: %w", path, err)
	}
	return nil
}

func (l Layout) readJSON(path string, canonicalPath string, dest any) error {
	if err := l.validateKnownPath(path, canonicalPath); err != nil {
		return err
	}
	path = canonicalPath
	if err := l.ensurePathInsideProject(path); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotFound, path)
	}
	if err != nil {
		return fmt.Errorf("read workspace file %s: %w", path, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal workspace JSON %s: %w", path, err)
	}
	return nil
}

func validateWorkspaceName(name string) error {
	if name == "" || name == "." || name == ".." {
		return fmt.Errorf("workspace name %q is not safe", name)
	}
	if filepath.Clean(name) != name || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("workspace name %q is not safe", name)
	}
	return nil
}

func (l Layout) validateKnownPath(path string, canonicalPath string) error {
	known, err := l.canonicalPaths()
	if err != nil {
		return err
	}
	if !slices.Contains([]string{known.BaseClaude, known.BaseCodex, known.BaseCanon, known.SyncState, known.LearnedResolutions}, canonicalPath) {
		return fmt.Errorf("workspace path %s is not a known layout path", canonicalPath)
	}
	if filepath.Clean(path) != canonicalPath {
		return fmt.Errorf("workspace path %s is not a known layout path", path)
	}
	return nil
}

func (l Layout) canonicalPaths() (layoutPaths, error) {
	if l.paths.Project == "" {
		return layoutPaths{}, fmt.Errorf("workspace layout is not initialized")
	}
	return l.paths, nil
}

func (l Layout) ensureWorkspaceDirectory(dir string) error {
	if err := os.MkdirAll(dir, workspaceDirMode); err != nil {
		return fmt.Errorf("create workspace directory %s: %w", dir, err)
	}
	dirs, err := l.workspaceDirectoriesTo(dir)
	if err != nil {
		return err
	}
	for _, path := range dirs {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect workspace directory %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("workspace directory %s must not be a symlink", path)
		}
		if !info.IsDir() {
			return fmt.Errorf("workspace path %s is not a directory", path)
		}
		if err := os.Chmod(path, workspaceDirMode); err != nil {
			return fmt.Errorf("set workspace directory mode %s: %w", path, err)
		}
	}
	return nil
}

func (l Layout) workspaceDirectoriesTo(dir string) ([]string, error) {
	known, err := l.canonicalPaths()
	if err != nil {
		return nil, err
	}
	root := filepath.Clean(known.Root)
	dir = filepath.Clean(dir)
	inside, err := isInside(root, dir)
	if err != nil {
		return nil, err
	}
	if !inside {
		return nil, fmt.Errorf("workspace directory %s is not under workspace root %s", dir, root)
	}
	dirs := []string{root}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return nil, fmt.Errorf("compare workspace directory %s to root %s: %w", dir, root, err)
	}
	if rel == "." {
		return dirs, nil
	}
	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		dirs = append(dirs, current)
	}
	return dirs, nil
}

func (l Layout) ensurePathInsideProject(path string) error {
	known, err := l.canonicalPaths()
	if err != nil {
		return err
	}
	project, err := filepath.EvalSymlinks(known.Project)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("project path does not exist %s: %w", known.Project, err)
	}
	if err != nil {
		return fmt.Errorf("resolve project path %s: %w", known.Project, err)
	}
	project = filepath.Clean(project)

	resolved, err := resolveWorkspaceBoundary(path)
	if err != nil {
		return err
	}
	inside, err := isInside(project, resolved)
	if err != nil {
		return err
	}
	if !inside {
		return fmt.Errorf("workspace path %s resolves outside project %s", path, known.Project)
	}
	return nil
}

func resolveWorkspaceBoundary(path string) (string, error) {
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
			return "", fmt.Errorf("resolve workspace path %s: %w", path, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("resolve workspace path %s: %w", path, err)
		}
		missing = append(missing, filepath.Base(current))
		current = parent
	}
}

func isInside(parent string, child string) (bool, error) {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if child == parent {
		return true, nil
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false, fmt.Errorf("compare workspace path %s to project %s: %w", child, parent, err)
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".", nil
}
