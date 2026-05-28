package app_test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/app"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

func TestRunConflictsMissingSyncStateReturnsExitOne(t *testing.T) {
	fixture := copiedFixture(t, "basic")
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"conflicts", "--project", fixture.project}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "no sync state found; run \"agent-canon sync claude codex\" first") {
		t.Fatalf("stderr missing missing-state guidance: %q", stderr.String())
	}
}

func TestRunConflictsTextListsOpenConflicts(t *testing.T) {
	fixture := fixtureWithOpenConflict(t)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"conflicts", "--project", fixture.project}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	for _, want := range []string{
		"agent-canon conflicts: claude -> codex",
		"Project: " + fixture.project,
		"Summary: open=1 resolved=0 diffs=1",
		"Open conflicts:",
		"- conflict-001 ContentConflict instruction:project-claude-md [Instruction] scope=project",
		"why: both sides changed content differently",
		"Resolved conflicts: 0",
		"Next steps:",
		"agent-canon resolve conflict-001 --ours",
		"agent-canon resolve conflict-001 --theirs",
		"agent-canon resolve conflict-001 --manual <value>",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q in %q", want, stdout.String())
		}
	}
}

func TestRunConflictsFormatJSONPrintsSyncState(t *testing.T) {
	fixture := fixtureWithOpenConflict(t)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"conflicts", "--project", fixture.project, "--format", "json"}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	var state model.SyncStateReport
	if err := json.Unmarshal(stdout.Bytes(), &state); err != nil {
		t.Fatalf("stdout is not valid sync state JSON: %v\n%s", err, stdout.String())
	}
	if state.SchemaVersion != model.SyncStateSchemaVersion || state.Summary.OpenConflicts != 1 || len(state.Conflicts) != 1 {
		t.Fatalf("conflicts JSON = %#v", state)
	}
}

func TestRunConflictsDoesNotWriteWorkspaceState(t *testing.T) {
	fixture := fixtureWithOpenConflict(t)
	workspaceRoot := filepath.Join(fixture.project, ".agent-canon")
	before := directorySnapshot(t, workspaceRoot)
	var stdout, stderr bytes.Buffer

	code := app.Run([]string{"conflicts", "--project", fixture.project}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if after := directorySnapshot(t, workspaceRoot); !reflect.DeepEqual(after, before) {
		t.Fatalf("conflicts modified workspace state")
	}
}

func fixtureWithOpenConflict(t *testing.T) fixturePaths {
	t.Helper()
	fixture := copiedFixture(t, "basic")
	claudePath := filepath.Join(fixture.project, "CLAUDE.md")
	codexPath := filepath.Join(fixture.project, "AGENTS.md")
	writeFile(t, claudePath, "shared base\n")
	writeFile(t, codexPath, "shared base\n")

	var stdout, stderr bytes.Buffer
	code := app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("initial sync exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	writeFile(t, claudePath, "ours changed\n")
	writeFile(t, codexPath, "theirs changed\n")
	stdout.Reset()
	stderr.Reset()
	code = app.Run([]string{"sync", "claude", "codex", "--project", fixture.project, "--claude-home", fixture.claudeHome, "--codex-home", fixture.codexHome}, fixture.project, fixture.home, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("second sync exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	return fixture
}
