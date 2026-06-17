package skillbundle

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type File struct {
	SourcePath   string
	RelativePath string
	Contents     []byte
}

func Files(skillFile string) ([]File, error) {
	root := filepath.Dir(skillFile)
	if resolved, err := filepath.EvalSymlinks(skillFile); err == nil {
		root = filepath.Dir(resolved)
	} else if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	var files []File
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk skill bundle %s: %w", path, err)
		}
		if path == root {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat skill bundle file %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel skill bundle file %s: %w", path, err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read skill bundle file %s: %w", path, err)
		}
		files = append(files, File{
			SourcePath:   path,
			RelativePath: filepath.ToSlash(relativePath),
			Contents:     contents,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	sort.SliceStable(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})
	return files, nil
}
