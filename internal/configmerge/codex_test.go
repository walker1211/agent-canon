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

func TestMergeCodexMCPBlocksSameNameDifferentServerWithoutResolution(t *testing.T) {
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
	for _, want := range []string{"unresolved config merge conflict", "agent-canon sync", "agent-canon conflicts", "agent-canon resolve"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}

func TestMergeCodexMCPResolutionOursReplacesOnlyConflictingBlock(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	before := "model = \"fixture-model\"\n\n" +
		"[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"\nargs = [\"--old\"]\n\n" +
		"[profiles.default]\napproval_policy = \"never\"\n\n" +
		"[mcp_servers.\"keep-me\"]\ncommand = \"keep-mcp\"\n"
	writeFile(t, targetPath, before)
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp",
		"args": ["--stdio"],
		"env": {"SAFE_MODE": "1"}
	}`)
	fingerprint := requireConfigMergeFingerprint(t, scanWith(resource), targetPath)

	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{
		Scan:       scanWith(resource),
		TargetPath: targetPath,
		Resolutions: []configmerge.CodexMCPResolution{{
			Fingerprint: fingerprint,
			Decision:    model.ResolutionDecisionOurs,
		}},
	})
	if err != nil {
		t.Fatalf("MergeCodexMCP returned error: %v", err)
	}

	contents := string(result.Contents)
	for _, want := range []string{
		"model = \"fixture-model\"",
		"[profiles.default]\napproval_policy = \"never\"",
		"[mcp_servers.\"keep-me\"]\ncommand = \"keep-mcp\"",
		"[mcp_servers.\"fixture-github\"]\ncommand = \"fixture-mcp\"\nargs = [\"--stdio\"]\nenv = { \"SAFE_MODE\" = \"1\" }",
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("resolved contents missing %q:\n%s", want, contents)
		}
	}
	for _, unwanted := range []string{"command = \"other-mcp\"", "args = [\"--old\"]"} {
		if strings.Contains(contents, unwanted) {
			t.Fatalf("resolved contents still contains %q:\n%s", unwanted, contents)
		}
	}
	if strings.Count(contents, "[mcp_servers.\"fixture-github\"]") != 1 {
		t.Fatalf("resolved contents should contain exactly one fixture-github block:\n%s", contents)
	}
}

func TestMergeCodexMCPResolutionOursReplacesNestedChildTables(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	before := "[mcp_servers.\"fixture-github\"]\ncommand = \"old-github\"\n\n" +
		"[mcp_servers.\"fixture-github\".env]\nSAFE_MODE = \"old\"\n\n" +
		"[profiles.default]\napproval_policy = \"never\"\n\n" +
		"[mcp_servers.\"fixture-linear\"]\ncommand = \"old-linear\"\n\n" +
		"[mcp_servers.\"fixture-linear\".env]\nSAFE_MODE = \"old\"\n\n" +
		"[[unrelated.plugins]]\nname = \"keep\"\n\n" +
		"[profiles.after]\nmodel = \"fixture-model\"\n"
	writeFile(t, targetPath, before)
	settingsPath := filepath.Join(root, "settings.json")
	writeFile(t, settingsPath, `{"mcpServers": {
		"fixture-github": {"command": "new-github", "env": {"SAFE_MODE": "new"}},
		"fixture-linear": {"command": "new-linear", "env": {"SAFE_MODE": "new"}}
	}}
`)
	scan := scanWith(mcpResource(settingsPath, "fixture-github"), mcpResource(settingsPath, "fixture-linear"))
	analysis, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: scan, TargetPath: targetPath})
	if err != nil {
		t.Fatalf("DetectCodexMCPConflicts returned error: %v", err)
	}
	if len(analysis.Conflicts) != 2 {
		t.Fatalf("conflicts = %#v, want two conflicts", analysis.Conflicts)
	}
	resolutions := make([]configmerge.CodexMCPResolution, 0, len(analysis.Conflicts))
	for _, conflict := range analysis.Conflicts {
		resolutions = append(resolutions, configmerge.CodexMCPResolution{Fingerprint: conflict.Fingerprint, Decision: model.ResolutionDecisionOurs})
	}

	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{Scan: scan, TargetPath: targetPath, Resolutions: resolutions})
	if err != nil {
		t.Fatalf("MergeCodexMCP returned error: %v", err)
	}

	contents := string(result.Contents)
	for _, want := range []string{
		"[profiles.default]\napproval_policy = \"never\"",
		"[[unrelated.plugins]]\nname = \"keep\"",
		"[profiles.after]\nmodel = \"fixture-model\"",
		"[mcp_servers.\"fixture-github\"]\ncommand = \"new-github\"\nenv = { \"SAFE_MODE\" = \"new\" }",
		"[mcp_servers.\"fixture-linear\"]\ncommand = \"new-linear\"\nenv = { \"SAFE_MODE\" = \"new\" }",
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("resolved contents missing %q:\n%s", want, contents)
		}
	}
	for _, unwanted := range []string{"[mcp_servers.\"fixture-github\".env]", "[mcp_servers.\"fixture-linear\".env]", "old-github", "old-linear", "SAFE_MODE = \"old\""} {
		if strings.Contains(contents, unwanted) {
			t.Fatalf("resolved contents still contains %q:\n%s", unwanted, contents)
		}
	}
}

func TestDetectCodexMCPConflictsIncludesNestedChildTablesInFingerprint(t *testing.T) {
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeFile(t, settingsPath, `{"mcpServers": {"fixture-github": {"command": "fixture-mcp", "env": {"SAFE_MODE": "new"}}}}
`)
	scan := scanWith(mcpResource(settingsPath, "fixture-github"))
	firstTarget := filepath.Join(root, "first.toml")
	secondTarget := filepath.Join(root, "second.toml")
	writeFile(t, firstTarget, "[mcp_servers.\"fixture-github\"]\ncommand = \"fixture-mcp\"\n\n[mcp_servers.\"fixture-github\".env]\nSAFE_MODE = \"old-one\"\n")
	writeFile(t, secondTarget, "[mcp_servers.\"fixture-github\"]\ncommand = \"fixture-mcp\"\n\n[mcp_servers.\"fixture-github\".env]\nSAFE_MODE = \"old-two\"\n")

	first := requireConfigMergeFingerprint(t, scan, firstTarget)
	second := requireConfigMergeFingerprint(t, scan, secondTarget)
	if first == second {
		t.Fatalf("fingerprint did not change after nested child table content changed: %s", first)
	}
}

func TestMergeCodexMCPResolutionTheirsKeepsConflictAndAppendsOtherServers(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "model = \"fixture-model\"\n\n[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"\n")
	settingsPath := filepath.Join(root, "settings.json")
	writeFile(t, settingsPath, `{"mcpServers": {
		"fixture-github": {"command": "fixture-mcp"},
		"new-server": {"command": "new-mcp", "args": ["--stdio"]}
	}}
`)
	conflictResource := mcpResource(settingsPath, "fixture-github")
	newResource := mcpResource(settingsPath, "new-server")
	scan := scanWith(conflictResource, newResource)
	fingerprint := requireConfigMergeFingerprint(t, scan, targetPath)

	result, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{
		Scan:       scan,
		TargetPath: targetPath,
		Resolutions: []configmerge.CodexMCPResolution{{
			Fingerprint: fingerprint,
			Decision:    model.ResolutionDecisionTheirs,
		}},
	})
	if err != nil {
		t.Fatalf("MergeCodexMCP returned error: %v", err)
	}

	contents := string(result.Contents)
	for _, want := range []string{
		"[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"",
		"[mcp_servers.\"new-server\"]\ncommand = \"new-mcp\"\nargs = [\"--stdio\"]",
	} {
		if !strings.Contains(contents, want) {
			t.Fatalf("resolved contents missing %q:\n%s", want, contents)
		}
	}
	if strings.Contains(contents, "command = \"fixture-mcp\"") {
		t.Fatalf("theirs resolution should not append ours duplicate:\n%s", contents)
	}
}

func TestMergeCodexMCPRejectsUnsupportedResolutions(t *testing.T) {
	for _, decision := range []model.ResolutionDecision{model.ResolutionDecisionManual, model.ResolutionDecisionSuggestion, model.ResolutionDecision("unknown")} {
		t.Run(string(decision), func(t *testing.T) {
			root := t.TempDir()
			targetPath := filepath.Join(root, "config.toml")
			writeFile(t, targetPath, "[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"\n")
			resource := writeMCPSettings(t, root, "fixture-github", `{
				"command": "fixture-mcp"
			}`)
			fingerprint := requireConfigMergeFingerprint(t, scanWith(resource), targetPath)

			_, err := configmerge.MergeCodexMCP(configmerge.CodexMCPInput{
				Scan:       scanWith(resource),
				TargetPath: targetPath,
				Resolutions: []configmerge.CodexMCPResolution{{
					Fingerprint: fingerprint,
					Decision:    decision,
				}},
			})
			if err == nil {
				t.Fatal("MergeCodexMCP returned nil error")
			}
			if !strings.Contains(err.Error(), "unsupported config merge resolution") {
				t.Fatalf("error = %q, want unsupported resolution", err)
			}
		})
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

func TestDetectCodexMCPConflictsReportsSameNameDifferentServer(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"\n")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp"
	}`)

	analysis, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err != nil {
		t.Fatalf("DetectCodexMCPConflicts returned error: %v", err)
	}
	if !analysis.Existing {
		t.Fatal("Existing = false, want true")
	}
	if analysis.MergeableServers != 1 {
		t.Fatalf("MergeableServers = %d, want 1", analysis.MergeableServers)
	}
	if len(analysis.Conflicts) != 1 {
		t.Fatalf("conflicts = %#v, want one conflict", analysis.Conflicts)
	}
	conflict := analysis.Conflicts[0]
	if conflict.Kind != model.ConflictKindConfigMerge {
		t.Fatalf("Kind = %s, want %s", conflict.Kind, model.ConflictKindConfigMerge)
	}
	if conflict.ResourceKind != model.KindMCPServer {
		t.Fatalf("ResourceKind = %s, want %s", conflict.ResourceKind, model.KindMCPServer)
	}
	if conflict.ResourceID != resource.ID {
		t.Fatalf("ResourceID = %q, want %q", conflict.ResourceID, resource.ID)
	}
	if conflict.Scope != resource.Scope {
		t.Fatalf("Scope = %s, want %s", conflict.Scope, resource.Scope)
	}
	if !conflict.RequiresUserDecision || conflict.Status != model.ConflictStatusOpen {
		t.Fatalf("decision/status = %v/%s, want open user decision", conflict.RequiresUserDecision, conflict.Status)
	}
	if conflict.Fingerprint == "" {
		t.Fatal("Fingerprint is empty")
	}
	if conflict.Details["serverName"] != "fixture-github" {
		t.Fatalf("serverName detail = %q, want fixture-github", conflict.Details["serverName"])
	}
	if conflict.Details["targetPath"] != targetPath {
		t.Fatalf("targetPath detail = %q, want %q", conflict.Details["targetPath"], targetPath)
	}
	if conflict.Details["reason"] == "" {
		t.Fatal("reason detail is empty")
	}
	if conflict.Ours == nil || conflict.Theirs == nil {
		t.Fatalf("conflict states = ours:%#v theirs:%#v, want both summaries", conflict.Ours, conflict.Theirs)
	}
}

func TestDetectCodexMCPConflictsIgnoresIdenticalExistingServer(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "[mcp_servers.\"fixture-github\"]\ncommand = \"fixture-mcp\"\nargs = [\"--stdio\"]\n")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp",
		"args": ["--stdio"]
	}`)

	analysis, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err != nil {
		t.Fatalf("DetectCodexMCPConflicts returned error: %v", err)
	}
	if len(analysis.Conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", analysis.Conflicts)
	}
}

func TestDetectCodexMCPConflictsSkipsSecretEnvWithoutLeak(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "[mcp_servers.\"fixture-github\"]\ncommand = \"other-mcp\"\n")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp",
		"env": {"SAFE_VALUE": "`+fixtureSecret+`"}
	}`)

	analysis, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err != nil {
		t.Fatalf("DetectCodexMCPConflicts returned error: %v", err)
	}
	if len(analysis.Conflicts) != 0 {
		t.Fatalf("conflicts = %#v, want none", analysis.Conflicts)
	}
	if !hasWarning(analysis.Warnings, "mcp-merge-skipped-secret") {
		t.Fatalf("warnings missing mcp-merge-skipped-secret: %#v", analysis.Warnings)
	}
	if strings.Contains(warningsText(analysis.Warnings), fixtureSecret) {
		t.Fatalf("warnings leaked fixture secret: %#v", analysis.Warnings)
	}
}

func TestDetectCodexMCPConflictsRejectsDuplicateCodexBlocks(t *testing.T) {
	root := t.TempDir()
	targetPath := filepath.Join(root, "config.toml")
	writeFile(t, targetPath, "[mcp_servers.\"fixture-github\"]\ncommand = \"one\"\n\n[mcp_servers.\"fixture-github\"]\ncommand = \"two\"\n")
	resource := writeMCPSettings(t, root, "fixture-github", `{
		"command": "fixture-mcp"
	}`)

	_, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: scanWith(resource), TargetPath: targetPath})
	if err == nil {
		t.Fatal("DetectCodexMCPConflicts returned nil error")
	}
	if !strings.Contains(err.Error(), "defined more than once") {
		t.Fatalf("error = %q, want duplicate block error", err)
	}
}

func requireConfigMergeFingerprint(t *testing.T, scan model.ScanReport, targetPath string) string {
	t.Helper()
	analysis, err := configmerge.DetectCodexMCPConflicts(configmerge.CodexMCPAnalysisInput{Scan: scan, TargetPath: targetPath})
	if err != nil {
		t.Fatalf("DetectCodexMCPConflicts returned error: %v", err)
	}
	if len(analysis.Conflicts) != 1 {
		t.Fatalf("conflicts = %#v, want one conflict", analysis.Conflicts)
	}
	return analysis.Conflicts[0].Fingerprint
}

func writeMCPSettings(t *testing.T, root string, name string, serverJSON string) model.Resource {
	t.Helper()
	settingsPath := filepath.Join(root, "settings.json")
	writeFile(t, settingsPath, "{\"mcpServers\": {\""+name+"\": "+serverJSON+"}}\n")
	return mcpResource(settingsPath, name)
}

func mcpResource(settingsPath string, name string) model.Resource {
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

func warningsText(warnings []model.Warning) string {
	var builder strings.Builder
	for _, warning := range warnings {
		builder.WriteString(warning.Code)
		builder.WriteByte('\n')
		builder.WriteString(warning.Message)
		builder.WriteByte('\n')
	}
	return builder.String()
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
