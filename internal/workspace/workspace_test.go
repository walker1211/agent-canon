package workspace_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/workspace"
)

func TestNewReturnsExpectedProjectLocalPaths(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	root := filepath.Join(project, ".agent-canon")
	base := filepath.Join(root, "base")
	resolutions := filepath.Join(root, "resolutions")
	got := map[string]string{
		"Project":            layout.Project,
		"Root":               layout.Root,
		"BaseDir":            layout.BaseDir,
		"BaseClaude":         layout.BaseClaude,
		"BaseCodex":          layout.BaseCodex,
		"BaseCanon":          layout.BaseCanon,
		"SyncState":          layout.SyncState,
		"ResolutionsDir":     layout.ResolutionsDir,
		"LearnedResolutions": layout.LearnedResolutions,
	}
	want := map[string]string{
		"Project":            project,
		"Root":               root,
		"BaseDir":            base,
		"BaseClaude":         filepath.Join(base, "claude.snapshot.json"),
		"BaseCodex":          filepath.Join(base, "codex.snapshot.json"),
		"BaseCanon":          filepath.Join(base, "canon.snapshot.json"),
		"SyncState":          filepath.Join(root, "sync-state.json"),
		"ResolutionsDir":     resolutions,
		"LearnedResolutions": filepath.Join(resolutions, "learned-resolutions.json"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Layout paths = %#v, want %#v", got, want)
	}
}

func TestSaveSyncStateCreatesWorkspaceAndWritesIndentedJSON(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	value := map[string]any{"tool": "agent-canon", "count": 2}
	if err := layout.SaveSyncState(value); err != nil {
		t.Fatalf("SaveSyncState returned error: %v", err)
	}

	assertDirMode(t, layout.Root, 0o755)
	assertFileMode(t, layout.SyncState, 0o644)
	assertFileContents(t, layout.SyncState, "{\n  \"count\": 2,\n  \"tool\": \"agent-canon\"\n}\n")
}

func TestSaveSyncStateForcesExpectedModesUnderRestrictiveUmask(t *testing.T) {
	oldUmask := syscall.Umask(0o077)
	t.Cleanup(func() {
		syscall.Umask(oldUmask)
	})

	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := layout.SaveSyncState(map[string]string{"tool": "agent-canon"}); err != nil {
		t.Fatalf("SaveSyncState returned error: %v", err)
	}

	assertDirMode(t, layout.Root, 0o755)
	assertFileMode(t, layout.SyncState, 0o644)
}

func TestLoadSyncStateReturnsErrNotFoundForMissingFile(t *testing.T) {
	layout, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	var dest map[string]any
	err = layout.LoadSyncState(&dest)
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("LoadSyncState error = %v, want ErrNotFound", err)
	}
}

func TestSaveFailsWhenWorkspaceSymlinkEscapesProjectAndDoesNotWriteOutside(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(project, ".agent-canon")); err != nil {
		t.Fatalf("create workspace symlink: %v", err)
	}
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	err = layout.SaveSyncState(map[string]string{"status": "unsafe"})
	if err == nil {
		t.Fatalf("SaveSyncState returned nil error for escaping symlink")
	}
	if !strings.Contains(err.Error(), ".agent-canon") || !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing workspace/project context: %v", err)
	}
	assertPathMissing(t, filepath.Join(outside, "sync-state.json"))
}

func TestSaveFailsWhenWorkspaceSubdirSymlinkEscapesProjectAndDoesNotWriteOutside(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.Symlink(outside, layout.BaseDir); err != nil {
		t.Fatalf("create base symlink: %v", err)
	}

	err = layout.SaveBaseClaude(map[string]string{"status": "unsafe"})
	if err == nil {
		t.Fatalf("SaveBaseClaude returned nil error for escaping base symlink")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing project boundary context: %v", err)
	}
	assertPathMissing(t, filepath.Join(outside, "claude.snapshot.json"))
}

func TestLoadFailsWhenWorkspaceSubdirSymlinkEscapesProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "learned-resolutions.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.Symlink(outside, layout.ResolutionsDir); err != nil {
		t.Fatalf("create resolutions symlink: %v", err)
	}

	var dest map[string]any
	err = layout.LoadLearnedResolutions(&dest)
	if err == nil {
		t.Fatalf("LoadLearnedResolutions returned nil error for escaping resolutions symlink")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing project boundary context: %v", err)
	}
}

func TestSaveFailsWhenWorkspaceStateFileSymlinkEscapesProjectAndTargetIsMissing(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	outsideTarget := filepath.Join(outside, "sync-state.json")
	if err := os.Symlink(outsideTarget, layout.SyncState); err != nil {
		t.Fatalf("create state file symlink: %v", err)
	}

	err = layout.SaveSyncState(map[string]string{"status": "unsafe"})
	if err == nil {
		t.Fatalf("SaveSyncState returned nil error for escaping state file symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error missing symlink context: %v", err)
	}
	assertPathMissing(t, outsideTarget)
}

func TestTypedSavesRejectMutatedLayoutPaths(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	unexpected := filepath.Join(layout.Root, "unexpected.json")
	layout.BaseClaude = unexpected

	err = layout.SaveBaseClaude(map[string]string{"name": "claude"})
	if err == nil {
		t.Fatalf("SaveBaseClaude returned nil error for mutated layout path")
	}
	if !strings.Contains(err.Error(), "known layout path") {
		t.Fatalf("error missing known-path context: %v", err)
	}
	assertPathMissing(t, unexpected)
}

func TestTypedSavesRejectMutatedProjectAndLayoutPath(t *testing.T) {
	project := t.TempDir()
	otherProject := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	otherLayout, err := workspace.New(otherProject)
	if err != nil {
		t.Fatalf("New returned error for other project: %v", err)
	}
	layout.Project = otherLayout.Project
	layout.BaseClaude = otherLayout.BaseClaude

	err = layout.SaveBaseClaude(map[string]string{"name": "claude"})
	if err == nil {
		t.Fatalf("SaveBaseClaude returned nil error for mutated project and layout path")
	}
	if !strings.Contains(err.Error(), "known layout path") {
		t.Fatalf("error missing known-path context: %v", err)
	}
	assertPathMissing(t, otherLayout.BaseClaude)
}

func TestTypedSavesRejectCrossCanonicalMutatedLayoutPaths(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	layout.BaseClaude = layout.SyncState

	err = layout.SaveBaseClaude(map[string]string{"name": "claude"})
	if err == nil {
		t.Fatalf("SaveBaseClaude returned nil error for cross-canonical mutated layout path")
	}
	if !strings.Contains(err.Error(), "known layout path") {
		t.Fatalf("error missing known-path context: %v", err)
	}
	assertPathMissing(t, layout.SyncState)
}

func TestTypedSavesWriteOnlyExpectedWorkspaceFiles(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if err := layout.SaveBaseClaude(map[string]string{"name": "claude"}); err != nil {
		t.Fatalf("SaveBaseClaude returned error: %v", err)
	}
	if err := layout.SaveBaseCodex(map[string]string{"name": "codex"}); err != nil {
		t.Fatalf("SaveBaseCodex returned error: %v", err)
	}
	if err := layout.SaveBaseCanon(map[string]string{"name": "canon"}); err != nil {
		t.Fatalf("SaveBaseCanon returned error: %v", err)
	}
	if err := layout.SaveLearnedResolutions(map[string]string{"name": "learned"}); err != nil {
		t.Fatalf("SaveLearnedResolutions returned error: %v", err)
	}

	wantFiles := map[string]string{
		layout.BaseClaude:         "{\n  \"name\": \"claude\"\n}\n",
		layout.BaseCodex:          "{\n  \"name\": \"codex\"\n}\n",
		layout.BaseCanon:          "{\n  \"name\": \"canon\"\n}\n",
		layout.LearnedResolutions: "{\n  \"name\": \"learned\"\n}\n",
	}
	for path, want := range wantFiles {
		assertFileContents(t, path, want)
	}
	assertPathMissing(t, layout.SyncState)

	gotFiles := collectFiles(t, layout.Root)
	if !reflect.DeepEqual(gotFiles, sortedKeys(wantFiles)) {
		t.Fatalf("workspace files = %v, want %v", gotFiles, sortedKeys(wantFiles))
	}
}

func TestGitignoreIncludesWorkspaceDataDirectory(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "..", ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(contents), "\n/.agent-canon/\n") {
		t.Fatalf(".gitignore does not include /.agent-canon/ under Data")
	}
	data := strings.Index(string(contents), "# Data")
	build := strings.Index(string(contents), "# Build artifacts")
	entry := strings.Index(string(contents), "\n/.agent-canon/\n")
	if data == -1 || build == -1 || entry == -1 || entry < data || entry > build {
		t.Fatalf("/.agent-canon/ is not under Data section")
	}
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

func assertDirMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
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

func collectFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	// Paths are inserted in lexical order by this test's map only after sorting.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
