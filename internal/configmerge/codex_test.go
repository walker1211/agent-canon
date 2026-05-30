package configmerge_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/configmerge"
	"github.com/zhangyoujun/agent-canon/internal/model"
)

const fixtureSecret = "ghp_agent_canon_fixture_secret_must_not_leak"

func TestMergeCodexMCPAppendsAbsentServerAndPreservesUnrelatedTOML(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "model = \"gpt-5\"\n\n[profiles.default]\napproval_policy = \"never\"\n")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp",
		"args": ["--stdio"],
		"env": {"SAFE_MODE": "1"}
	}`)

	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err != nil {
		t.Fatalf("MergeCodexMCP returned error: %v", err)
	}

	contents := string(result.Contents)
	for _, want := range []string{
		"model = \"gpt-5\"",
		"[profiles.default]",
		"approval_policy = \"never\"",
		"[mcp_servers.\"fixture-github\"]",
		"command = \"fixture-mcp\"",
		"args = [\"--stdio\"]",
		"env = { \"SAFE_MODE\" = \"1\" }",
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("merged contents missing %q:\n%s", want, contents)
		}
	}
	if result.MergeableServers != 1 || !result.Existing {
		t.Fatalf("result metadata = %#v, want one existing mergeable server", result)
	}
}

func TestMergeCodexMCPTreatsIdenticalExistingServerAsNoop(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	existing := "[mcp_servers.\"fixture-github\"]\ncommand = \"fixture-mcp\"\nargs = [\"--stdio\"]\n"
	writeFile(t, targetPath, existing)
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp",
		"args": ["--stdio"]
	}`)

	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err != nil {
		t.Fatalf("MergeCodexMCP returned error: %v", err)
	}
	if string(result.Contents) != existing {
		t.Fatalf("contents changed for identical server:\n%s", result.Contents)
	}
}

func TestMergeCodexMCPBlocksSameNameDifferentServer(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"\n")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp"
	}`)

	_, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err == nil {
		t.Fatal("MergeCodexMCP returned nil error")
	}
	if !strings.Contains(err.Error(), "config-merge-conflict") {
		t.Fatalf("error = %q, want config-merge-conflict", err)
	}
}

func TestMergeCodexMCPSkipsSecretEnvValues(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp",
		"env": {"SAFE_VALUE": "`+fixtureSecret+`"}
	}`)

	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err != nil {
		t.Fatalf("MergeCodexMCP returned error: %v", err)
	}
	if strings.Contains(string(result.Contents), fixtureSecret) {
		t.Fatalf("merged contents leaked fixture secret:\n%s", result.Contents)
	}
	if result.MergeableServers != 0 {
		t.Fatalf("MergeableServers = %d, want 0", result.MergeableServers)
	}
	if !hasWarning(result.Warnings, "mcp-merge-skipped-secret") {
		t.Fatalf("warnings missing mcp-merge-skipped-secret: %#v", result.Warnings)
	}
}

func TestMergeCodexMCPRejectsMalformedClaudeMCPData(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": 42,
		"args": ["--stdio"]
	}`)

	_, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err == nil {
		t.Fatal("MergeCodexMCP returned nil error")
	}
	if !strings.Contains(err.Error(), "command") {
		t.Fatalf("error = %q, want command validation", err)
	}
}

func writeMCPSettings(t *testing.T, root string, name string, serverJSON string) model.Resource {
	t.Helper()
	settingsPath := filepath.Join(root, "settings.json")
	writeFile(t, settingsPath, "{\"mcpServers\": {\""+name+"\": "+serverJSON+"}}\n")
	return model.Resource{
		ID:         "mcp:global-" + name,
		Kind:       model.KindMCPServer,
		Scope:      model.ScopeGlobal,
		SourceTool: "claude",
		SourcePath: settingsPath,
		SourceName: name,
		TargetTool: "codex",
		Status:     model.StatusPartial,
		Strategy:   "manual-mcp-server-review",
	}
}

func scanWith(resources ...model.Resource) model.ScanReport {
	return model.ScanReport{
		SchemaVersion: model.ScanSchemaVersion,
		Source:        "claude",
		Target:        "codex",
		Resources:     resources,
	}
}

func hasWarning(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
