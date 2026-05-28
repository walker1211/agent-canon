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
	backups := filepath.Join(root, "backups")
	rollback := filepath.Join(root, "rollback")
	imports := filepath.Join(root, "imports")
	got := map[string]string{
		"Project":            layout.Project,
		"Root":               layout.Root,
		"BaseDir":            layout.BaseDir,
		"BaseClaude":         layout.BaseClaude,
		"BaseCodex":          layout.BaseCodex,
		"BaseCanon":          layout.BaseCanon,
		"Manifest":           layout.Manifest,
		"SyncState":          layout.SyncState,
		"ResolutionsDir":     layout.ResolutionsDir,
		"LearnedResolutions": layout.LearnedResolutions,
		"BackupsDir":         layout.BackupsDir,
		"RollbackDir":        layout.RollbackDir,
		"ImportsDir":         layout.ImportsDir,
		"ImportClaude":       layout.ImportClaude,
		"ImportCodex":        layout.ImportCodex,
	}
	want := map[string]string{
		"Project":            project,
		"Root":               root,
		"BaseDir":            base,
		"BaseClaude":         filepath.Join(base, "claude.snapshot.json"),
		"BaseCodex":          filepath.Join(base, "codex.snapshot.json"),
		"BaseCanon":          filepath.Join(base, "canon.snapshot.json"),
		"Manifest":           filepath.Join(root, "manifest.json"),
		"SyncState":          filepath.Join(root, "sync-state.json"),
		"ResolutionsDir":     resolutions,
		"LearnedResolutions": filepath.Join(resolutions, "learned-resolutions.json"),
		"BackupsDir":         backups,
		"RollbackDir":        rollback,
		"ImportsDir":         imports,
		"ImportClaude":       filepath.Join(imports, "claude.import.json"),
		"ImportCodex":        filepath.Join(imports, "codex.import.json"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Layout paths = %#v, want %#v", got, want)
	}
}

func TestSaveAndLoadManifestCreatesWorkspaceAndWritesIndentedJSON(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	value := map[string]any{"schemaVersion": "agent-canon.workspace-manifest.v1", "source": "claude", "target": "codex"}
	if err := layout.SaveManifest(value); err != nil {
		t.Fatalf("SaveManifest returned error: %v", err)
	}

	assertDirMode(t, layout.Root, 0o755)
	assertFileMode(t, layout.Manifest, 0o644)
	assertFileContents(t, layout.Manifest, "{\n  \"schemaVersion\": \"agent-canon.workspace-manifest.v1\",\n  \"source\": \"claude\",\n  \"target\": \"codex\"\n}\n")

	var got map[string]string
	if err := layout.LoadManifest(&got); err != nil {
		t.Fatalf("LoadManifest returned error: %v", err)
	}
	if got["schemaVersion"] != "agent-canon.workspace-manifest.v1" || got["source"] != "claude" || got["target"] != "codex" {
		t.Fatalf("manifest = %#v, want saved values", got)
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

func TestSaveAndLoadImportCodexReport(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	value := map[string]any{"schemaVersion": "agent-canon.import.v1", "tool": "codex"}
	if err := layout.SaveImportCodex(value); err != nil {
		t.Fatalf("SaveImportCodex returned error: %v", err)
	}

	assertDirMode(t, layout.Root, 0o755)
	assertDirMode(t, layout.ImportsDir, 0o755)
	assertFileMode(t, layout.ImportCodex, 0o644)
	assertFileContents(t, layout.ImportCodex, "{\n  \"schemaVersion\": \"agent-canon.import.v1\",\n  \"tool\": \"codex\"\n}\n")

	var got map[string]string
	if err := layout.LoadImportCodex(&got); err != nil {
		t.Fatalf("LoadImportCodex returned error: %v", err)
	}
	if got["schemaVersion"] != "agent-canon.import.v1" || got["tool"] != "codex" {
		t.Fatalf("import report = %#v, want saved values", got)
	}
}

func TestSaveAndLoadImportClaudeReport(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	value := map[string]any{"schemaVersion": "agent-canon.import.v1", "tool": "claude"}
	if err := layout.SaveImportClaude(value); err != nil {
		t.Fatalf("SaveImportClaude returned error: %v", err)
	}

	assertDirMode(t, layout.Root, 0o755)
	assertDirMode(t, layout.ImportsDir, 0o755)
	assertFileMode(t, layout.ImportClaude, 0o644)
	assertFileContents(t, layout.ImportClaude, "{\n  \"schemaVersion\": \"agent-canon.import.v1\",\n  \"tool\": \"claude\"\n}\n")

	var got map[string]string
	if err := layout.LoadImportClaude(&got); err != nil {
		t.Fatalf("LoadImportClaude returned error: %v", err)
	}
	if got["schemaVersion"] != "agent-canon.import.v1" || got["tool"] != "claude" {
		t.Fatalf("import report = %#v, want saved values", got)
	}
}

func TestLoadImportCodexReturnsErrNotFoundForMissingFile(t *testing.T) {
	layout, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	var dest map[string]any
	err = layout.LoadImportCodex(&dest)
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("LoadImportCodex error = %v, want ErrNotFound", err)
	}
}

func TestLoadImportClaudeReturnsErrNotFoundForMissingFile(t *testing.T) {
	layout, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	var dest map[string]any
	err = layout.LoadImportClaude(&dest)
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("LoadImportClaude error = %v, want ErrNotFound", err)
	}
}

func TestSaveImportCodexFailsWhenImportsDirSymlinkEscapesProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.Symlink(outside, layout.ImportsDir); err != nil {
		t.Fatalf("create imports symlink: %v", err)
	}

	err = layout.SaveImportCodex(map[string]string{"tool": "codex"})
	if err == nil {
		t.Fatalf("SaveImportCodex returned nil error for escaping imports symlink")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing project boundary context: %v", err)
	}
	assertPathMissing(t, filepath.Join(outside, "codex.import.json"))
}

func TestSaveImportClaudeFailsWhenImportsDirSymlinkEscapesProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.Symlink(outside, layout.ImportsDir); err != nil {
		t.Fatalf("create imports symlink: %v", err)
	}

	err = layout.SaveImportClaude(map[string]string{"tool": "claude"})
	if err == nil {
		t.Fatalf("SaveImportClaude returned nil error for escaping imports symlink")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing project boundary context: %v", err)
	}
	assertPathMissing(t, filepath.Join(outside, "claude.import.json"))
}

func TestSaveRollbackManifestWritesExpectedFile(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	path, err := layout.SaveRollbackManifest("apply-001", map[string]string{"target": "codex"})
	if err != nil {
		t.Fatalf("SaveRollbackManifest returned error: %v", err)
	}

	wantPath := filepath.Join(layout.RollbackDir, "apply-001.json")
	if path != wantPath {
		t.Fatalf("rollback manifest path = %q, want %q", path, wantPath)
	}
	assertDirMode(t, layout.Root, 0o755)
	assertDirMode(t, layout.RollbackDir, 0o755)
	assertFileMode(t, wantPath, 0o644)
	assertFileContents(t, wantPath, "{\n  \"target\": \"codex\"\n}\n")
}

func TestLoadRollbackManifestReturnsExpectedPathAndDecodedReport(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	savedPath, err := layout.SaveRollbackManifest("apply-001", map[string]string{"target": "codex"})
	if err != nil {
		t.Fatalf("SaveRollbackManifest returned error: %v", err)
	}

	var got map[string]string
	loadedPath, err := layout.LoadRollbackManifest("apply-001", &got)
	if err != nil {
		t.Fatalf("LoadRollbackManifest returned error: %v", err)
	}
	if loadedPath != savedPath {
		t.Fatalf("loaded path = %q, want %q", loadedPath, savedPath)
	}
	if got["target"] != "codex" {
		t.Fatalf("manifest = %#v, want target codex", got)
	}
}

func TestLoadRollbackManifestReturnsErrNotFoundForMissingFile(t *testing.T) {
	layout, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	var got map[string]any
	_, err = layout.LoadRollbackManifest("apply-missing", &got)
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("LoadRollbackManifest error = %v, want ErrNotFound", err)
	}
}

func TestLoadRollbackManifestFailsWhenRollbackDirSymlinkEscapesProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "apply-001.json"), []byte("{\"target\":\"outside\"}\n"), 0o644); err != nil {
		t.Fatalf("write outside manifest: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.Symlink(outside, layout.RollbackDir); err != nil {
		t.Fatalf("create rollback symlink: %v", err)
	}

	var got map[string]string
	_, err = layout.LoadRollbackManifest("apply-001", &got)
	if err == nil {
		t.Fatalf("LoadRollbackManifest returned nil error for escaping rollback symlink")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing project boundary context: %v", err)
	}
	if got["target"] == "outside" {
		t.Fatalf("LoadRollbackManifest read outside manifest")
	}
}

func TestLoadRollbackManifestFailsWhenManifestFileSymlinkEscapesProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	outsideManifest := filepath.Join(outside, "apply-001.json")
	if err := os.WriteFile(outsideManifest, []byte("{\"target\":\"outside\"}\n"), 0o644); err != nil {
		t.Fatalf("write outside manifest: %v", err)
	}
	if err := os.MkdirAll(layout.RollbackDir, 0o755); err != nil {
		t.Fatalf("create rollback dir: %v", err)
	}
	if err := os.Symlink(outsideManifest, filepath.Join(layout.RollbackDir, "apply-001.json")); err != nil {
		t.Fatalf("create rollback manifest symlink: %v", err)
	}

	var got map[string]string
	_, err = layout.LoadRollbackManifest("apply-001", &got)
	if err == nil {
		t.Fatalf("LoadRollbackManifest returned nil error for escaping manifest symlink")
	}
	if !strings.Contains(err.Error(), "project") && !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("error missing boundary context: %v", err)
	}
	if got["target"] == "outside" {
		t.Fatalf("LoadRollbackManifest read outside manifest")
	}
}

func TestBackupDirReturnsSafeProjectLocalDirectory(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	path, err := layout.BackupDir("apply-001")
	if err != nil {
		t.Fatalf("BackupDir returned error: %v", err)
	}
	want := filepath.Join(layout.BackupsDir, "apply-001")
	if path != want {
		t.Fatalf("backup dir = %q, want %q", path, want)
	}
}

func TestWorkspaceDynamicNamesRejectUnsafeSegments(t *testing.T) {
	layout, err := workspace.New(t.TempDir())
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	for _, name := range []string{"", ".", "..", "../escape", "nested/name"} {
		t.Run(name, func(t *testing.T) {
			if _, err := layout.BackupDir(name); err == nil {
				t.Fatalf("BackupDir(%q) returned nil error", name)
			}
			if _, err := layout.SaveRollbackManifest(name, map[string]string{"target": "codex"}); err == nil {
				t.Fatalf("SaveRollbackManifest(%q) returned nil error", name)
			}
			var dest map[string]any
			if _, err := layout.LoadRollbackManifest(name, &dest); err == nil {
				t.Fatalf("LoadRollbackManifest(%q) returned nil error", name)
			}
		})
	}
}

func TestSaveRollbackManifestFailsWhenRollbackDirSymlinkEscapesProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := os.MkdirAll(layout.Root, 0o755); err != nil {
		t.Fatalf("create workspace root: %v", err)
	}
	if err := os.Symlink(outside, layout.RollbackDir); err != nil {
		t.Fatalf("create rollback symlink: %v", err)
	}

	_, err = layout.SaveRollbackManifest("apply-001", map[string]string{"target": "codex"})
	if err == nil {
		t.Fatalf("SaveRollbackManifest returned nil error for escaping rollback symlink")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Fatalf("error missing project boundary context: %v", err)
	}
	assertPathMissing(t, filepath.Join(outside, "apply-001.json"))
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

func TestSaveManifestRejectsMutatedLayoutPath(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	unexpected := filepath.Join(layout.Root, "unexpected-manifest.json")
	layout.Manifest = unexpected

	err = layout.SaveManifest(map[string]string{"name": "manifest"})
	if err == nil {
		t.Fatalf("SaveManifest returned nil error for mutated layout path")
	}
	if !strings.Contains(err.Error(), "known layout path") {
		t.Fatalf("error missing known-path context: %v", err)
	}
	assertPathMissing(t, unexpected)
}

func TestSaveImportCodexRejectsMutatedLayoutPath(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	unexpected := filepath.Join(layout.Root, "unexpected-import.json")
	layout.ImportCodex = unexpected

	err = layout.SaveImportCodex(map[string]string{"tool": "codex"})
	if err == nil {
		t.Fatalf("SaveImportCodex returned nil error for mutated layout path")
	}
	if !strings.Contains(err.Error(), "known layout path") {
		t.Fatalf("error missing known-path context: %v", err)
	}
	assertPathMissing(t, unexpected)
}

func TestSaveImportClaudeRejectsMutatedLayoutPath(t *testing.T) {
	project := t.TempDir()
	layout, err := workspace.New(project)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	unexpected := filepath.Join(layout.Root, "unexpected-import.json")
	layout.ImportClaude = unexpected

	err = layout.SaveImportClaude(map[string]string{"tool": "claude"})
	if err == nil {
		t.Fatalf("SaveImportClaude returned nil error for mutated layout path")
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

	if err := layout.SaveManifest(map[string]string{"name": "manifest"}); err != nil {
		t.Fatalf("SaveManifest returned error: %v", err)
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
	if err := layout.SaveImportCodex(map[string]string{"name": "import"}); err != nil {
		t.Fatalf("SaveImportCodex returned error: %v", err)
	}

	wantFiles := map[string]string{
		layout.Manifest:           "{\n  \"name\": \"manifest\"\n}\n",
		layout.BaseClaude:         "{\n  \"name\": \"claude\"\n}\n",
		layout.BaseCodex:          "{\n  \"name\": \"codex\"\n}\n",
		layout.BaseCanon:          "{\n  \"name\": \"canon\"\n}\n",
		layout.LearnedResolutions: "{\n  \"name\": \"learned\"\n}\n",
		layout.ImportCodex:        "{\n  \"name\": \"import\"\n}\n",
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
