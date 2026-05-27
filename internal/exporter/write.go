package exporter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func WritePreview(root string, preview CodexPreview) error {
	validated, err := validatePreviewFiles(preview.Files)
	if err != nil {
		return err
	}
	if err := validatePreviewRoot(root); err != nil {
		return err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create preview root %q: %w", root, err)
	}
	for i, file := range preview.Files {
		target := filepath.Join(root, validated[i])
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create parent for preview file %q: %w", file.Path, err)
		}
		if err := os.WriteFile(target, file.Contents, 0o644); err != nil {
			return fmt.Errorf("write preview file %q: %w", file.Path, err)
		}
	}
	return nil
}

func validatePreviewFiles(files []PreviewFile) ([]string, error) {
	validated := make([]string, 0, len(files))
	seen := map[string]bool{}
	for _, file := range files {
		path, err := validatePreviewPath(file.Path)
		if err != nil {
			return nil, err
		}
		slashPath := filepath.ToSlash(path)
		if seen[slashPath] {
			return nil, fmt.Errorf("duplicate preview path %q", file.Path)
		}
		seen[slashPath] = true
		validated = append(validated, path)
	}
	if err := validatePreviewPathConflicts(validated); err != nil {
		return nil, err
	}
	return validated, nil
}

func validatePreviewPathConflicts(paths []string) error {
	slashPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		slashPaths = append(slashPaths, filepath.ToSlash(path))
	}
	sort.Strings(slashPaths)
	for i := 1; i < len(slashPaths); i++ {
		parent := slashPaths[i-1]
		child := slashPaths[i]
		if strings.HasPrefix(child, parent+"/") {
			return fmt.Errorf("preview path conflict: %q conflicts with descendant %q", parent, child)
		}
	}
	return nil
}

func validatePreviewPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("preview path is empty")
	}
	path = filepath.FromSlash(path)
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("preview path %q is absolute", path)
	}
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == ".." {
			return "", fmt.Errorf("preview path %q contains parent traversal '..'", path)
		}
	}
	clean := filepath.Clean(path)
	if clean == "." {
		return "", fmt.Errorf("preview path %q does not name a file", path)
	}
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("preview path %q is absolute after cleaning", path)
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("preview path %q escapes preview root", path)
	}
	return clean, nil
}

func validatePreviewRoot(root string) error {
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("preview root is empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat preview root %q: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("preview root %q is not a directory", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read preview root %q: %w", root, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("preview root %q must be empty; remove existing files or choose a new --out directory", root)
	}
	return nil
}
