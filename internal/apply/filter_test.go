package apply_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestFilterChangesKeepsOnlySelectedGroups(t *testing.T) {
	root := t.TempDir()
	roots := applypkg.FilterRoots{Project: filepath.Join(root, "project"), Home: filepath.Join(root, "home", ".codex")}
	changes := []applypkg.FileChange{
		change(filepath.Join(roots.Home, "config.toml"), model.ScopeGlobal),
		change(filepath.Join(roots.Home, "AGENTS.md"), model.ScopeGlobal),
		change(filepath.Join(roots.Home, "skills", "review", "SKILL.md"), model.ScopeGlobal),
	}

	filtered := applypkg.FilterChanges(changes, applypkg.ApplyFilters{Only: []string{"config", "skills"}}, roots)
	got := joinedChangePaths(filtered)
	if strings.Contains(got, "AGENTS.md") || !strings.Contains(got, "config.toml") || !strings.Contains(got, "skills") {
		t.Fatalf("filtered paths = %q, want config and skills only", got)
	}
}

func TestFilterChangesAppliesOnlyBeforeExclude(t *testing.T) {
	root := t.TempDir()
	roots := applypkg.FilterRoots{Project: filepath.Join(root, "project"), Home: filepath.Join(root, "home", ".claude")}
	changes := []applypkg.FileChange{
		change(filepath.Join(roots.Home, "skills", "keep", "SKILL.md"), model.ScopeGlobal),
		change(filepath.Join(roots.Home, "skills", "drop", "SKILL.md"), model.ScopeGlobal),
		change(filepath.Join(roots.Home, "settings.json"), model.ScopeGlobal),
	}

	filtered := applypkg.FilterChanges(changes, applypkg.ApplyFilters{Only: []string{"skills"}, Exclude: []string{"skills/drop/SKILL.md"}}, roots)
	got := joinedChangePaths(filtered)
	if !strings.Contains(got, filepath.Join("skills", "keep", "SKILL.md")) || strings.Contains(got, filepath.Join("skills", "drop", "SKILL.md")) || strings.Contains(got, "settings.json") {
		t.Fatalf("filtered paths = %q, want kept skill only", got)
	}
}

func TestFilterChangesMatchesAbsoluteRelativeAndSuffixPaths(t *testing.T) {
	root := t.TempDir()
	roots := applypkg.FilterRoots{Project: filepath.Join(root, "project"), Home: filepath.Join(root, "home", ".codex")}
	config := filepath.Join(roots.Home, "config.toml")
	agent := filepath.Join(roots.Home, "agents", "review.toml")
	changes := []applypkg.FileChange{change(config, model.ScopeGlobal), change(agent, model.ScopeGlobal)}

	absolute := applypkg.FilterChanges(changes, applypkg.ApplyFilters{Only: []string{config}}, roots)
	if len(absolute) != 1 || absolute[0].Path != config {
		t.Fatalf("absolute match = %#v, want config", absolute)
	}
	relative := applypkg.FilterChanges(changes, applypkg.ApplyFilters{Only: []string{"agents/review.toml"}}, roots)
	if len(relative) != 1 || relative[0].Path != agent {
		t.Fatalf("relative match = %#v, want agent", relative)
	}
	suffix := applypkg.FilterChanges(changes, applypkg.ApplyFilters{Only: []string{"review.toml"}}, roots)
	if len(suffix) != 1 || suffix[0].Path != agent {
		t.Fatalf("suffix match = %#v, want agent", suffix)
	}
}

func TestGroupGlobalChangesUsesStableDogfoodingGroups(t *testing.T) {
	root := t.TempDir()
	changes := []applypkg.FileChange{
		change(filepath.Join(root, ".codex", "config.toml"), model.ScopeGlobal),
		change(filepath.Join(root, ".codex", "AGENTS.md"), model.ScopeGlobal),
		change(filepath.Join(root, ".codex", "agents", "review.toml"), model.ScopeGlobal),
		change(filepath.Join(root, ".codex", "skills", "review", "SKILL.md"), model.ScopeGlobal),
		change(filepath.Join(root, "project", "AGENTS.md"), model.ScopeProject),
	}

	groups := applypkg.GroupGlobalChanges(changes)
	got := make([]string, 0, len(groups))
	for _, group := range groups {
		got = append(got, group.Name+":"+strconv.Itoa(len(group.Changes)))
	}
	if strings.Join(got, ",") != "root:1,config:1,agents:1,skills:1" {
		t.Fatalf("groups = %#v, want stable root/config/agents/skills groups", groups)
	}
}

func change(path string, scope model.Scope) applypkg.FileChange {
	return applypkg.FileChange{ApplyFileChange: model.ApplyFileChange{Path: path, Scope: scope, Action: model.ApplyActionModify}}
}

func joinedChangePaths(changes []applypkg.FileChange) string {
	paths := make([]string, 0, len(changes))
	for _, change := range changes {
		paths = append(paths, change.Path)
	}
	return strings.Join(paths, "\n")
}
