package apply_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	applypkg "github.com/zhangyoujun/agent-canon/internal/apply"
	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

func TestRedactedDiffRedactsBeforeAndAfterContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	writeFile(t, path, "OLD_TOKEN="+fixtureSecret+"\n")
	change := applypkg.FileChange{
		ApplyFileChange: model.ApplyFileChange{
			Path:       path,
			Scope:      model.ScopeProject,
			Action:     model.ApplyActionModify,
			BeforeHash: "sha256:before",
			AfterHash:  "sha256:after",
		},
		Contents: []byte("GITHUB_TOKEN=" + fixtureSecret + "\n"),
	}

	diff, err := applypkg.RedactedDiff(change)
	if err != nil {
		t.Fatalf("RedactedDiff returned error: %v", err)
	}
	if strings.Contains(diff, fixtureSecret) {
		t.Fatalf("redacted diff leaked fixture secret:\n%s", diff)
	}
	if !strings.Contains(diff, "OLD_TOKEN="+security.RedactedValue) || !strings.Contains(diff, "GITHUB_TOKEN="+security.RedactedValue) {
		t.Fatalf("redacted diff missing redaction markers:\n%s", diff)
	}
}

func TestRedactedDiffHandlesMissingBeforeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	change := applypkg.FileChange{
		ApplyFileChange: model.ApplyFileChange{Path: path, Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: "sha256:after"},
		Contents:        []byte("new file\n"),
	}

	diff, err := applypkg.RedactedDiff(change)
	if err != nil {
		t.Fatalf("RedactedDiff returned error: %v", err)
	}
	if !strings.Contains(diff, "(missing)") || !strings.Contains(diff, "new file") {
		t.Fatalf("create diff missing expected content:\n%s", diff)
	}
}

func TestFileChangeJSONOmitsContents(t *testing.T) {
	change := applypkg.FileChange{
		ApplyFileChange: model.ApplyFileChange{Path: "/repo/AGENTS.md", Scope: model.ScopeProject, Action: model.ApplyActionCreate, AfterHash: "sha256:after"},
		PreviewPath:     "AGENTS.md",
		Contents:        []byte("GITHUB_TOKEN=" + fixtureSecret),
	}

	payload, err := json.Marshal(change)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	text := string(payload)
	if strings.Contains(text, fixtureSecret) || strings.Contains(text, "Contents") || strings.Contains(text, "contents") {
		t.Fatalf("FileChange JSON leaked contents: %s", text)
	}
	if !strings.Contains(text, "previewPath") {
		t.Fatalf("FileChange JSON missing preview path context: %s", text)
	}
}

func TestRedactedDiffReturnsReadErrorForUnreadableBeforeFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read files despite mode bits")
	}
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	writeFile(t, path, "old\n")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := applypkg.RedactedDiff(applypkg.FileChange{ApplyFileChange: model.ApplyFileChange{Path: path}, Contents: []byte("new\n")})
	if err == nil {
		t.Fatalf("RedactedDiff returned nil error for unreadable before file")
	}
}
