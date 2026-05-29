package apply

import (
	"path/filepath"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

const (
	GroupRoot   = "root"
	GroupConfig = "config"
	GroupAgents = "agents"
	GroupSkills = "skills"
)

type ApplyFilters struct {
	Only    []string
	Exclude []string
}

type FilterRoots struct {
	Project string
	Home    string
}

type ChangeGroupSummary struct {
	Name    string
	Changes []model.ApplyFileChange
}

func FilterChanges(changes []FileChange, filters ApplyFilters, roots FilterRoots) []FileChange {
	filtered := changes
	if len(filters.Only) > 0 {
		filtered = filterMatching(filtered, filters.Only, roots, true)
	}
	if len(filters.Exclude) > 0 {
		filtered = filterMatching(filtered, filters.Exclude, roots, false)
	}
	return filtered
}

func GroupGlobalChanges(changes []FileChange) []ChangeGroupSummary {
	groups := []ChangeGroupSummary{
		{Name: GroupRoot},
		{Name: GroupConfig},
		{Name: GroupAgents},
		{Name: GroupSkills},
	}

	for _, change := range changes {
		if change.Scope != model.ScopeGlobal {
			continue
		}
		for i := range groups {
			if matchesGroup(change.Path, groups[i].Name) {
				groups[i].Changes = append(groups[i].Changes, change.ApplyFileChange)
				break
			}
		}
	}

	return groups
}

func filterMatching(changes []FileChange, selectors []string, roots FilterRoots, keepMatches bool) []FileChange {
	filtered := make([]FileChange, 0, len(changes))
	for _, change := range changes {
		matched := matchesAnySelector(change, selectors, roots)
		if matched == keepMatches {
			filtered = append(filtered, change)
		}
	}
	return filtered
}

func matchesAnySelector(change FileChange, selectors []string, roots FilterRoots) bool {
	for _, selector := range selectors {
		if matchesSelector(change, selector, roots) {
			return true
		}
	}
	return false
}

func matchesSelector(change FileChange, selector string, roots FilterRoots) bool {
	selector = filepath.Clean(filepath.FromSlash(selector))
	if matchesGroup(change.Path, selector) {
		return true
	}
	return matchesPath(change, selector, roots)
}

func matchesPath(change FileChange, selector string, roots FilterRoots) bool {
	target := filepath.Clean(change.Path)
	if filepath.IsAbs(selector) {
		return samePath(target, selector)
	}

	root := roots.Project
	if change.Scope == model.ScopeGlobal {
		root = roots.Home
	}
	if root != "" {
		if samePath(target, filepath.Join(root, selector)) {
			return true
		}
		if rel, err := filepath.Rel(root, target); err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			return samePath(rel, selector) || hasPathSuffix(rel, selector)
		}
	}

	return hasPathSuffix(target, selector)
}

func matchesGroup(path string, group string) bool {
	switch group {
	case GroupRoot:
		base := filepath.Base(path)
		return base == "CLAUDE.md" || base == "AGENTS.md"
	case GroupConfig:
		base := filepath.Base(path)
		return base == "settings.json" || base == "config.toml"
	case GroupAgents:
		return hasComponent(path, "agents")
	case GroupSkills:
		return hasComponent(path, "skills")
	default:
		return false
	}
}

func hasComponent(path string, component string) bool {
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		if part == component {
			return true
		}
	}
	return false
}

func hasPathSuffix(path string, suffix string) bool {
	path = filepath.Clean(path)
	suffix = filepath.Clean(suffix)
	return samePath(path, suffix) || strings.HasSuffix(path, string(filepath.Separator)+suffix)
}

func samePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
