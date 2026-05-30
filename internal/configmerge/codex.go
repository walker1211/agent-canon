package configmerge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/zhangyoujun/agent-canon/internal/model"
	"github.com/zhangyoujun/agent-canon/internal/security"
)

type CodexMCPResolution struct {
	Fingerprint string
	Decision    model.ResolutionDecision
}

type CodexMCPInput struct {
	Scan        model.ScanReport
	TargetPath  string
	Resolutions []CodexMCPResolution
}

type CodexMCPAnalysisInput struct {
	Scan       model.ScanReport
	TargetPath string
}

type CodexMCPAnalysis struct {
	Conflicts        []model.Conflict
	Warnings         []model.Warning
	MergeableServers int
	Existing         bool
}

type CodexMCPResult struct {
	Contents         []byte
	Warnings         []model.Warning
	Existing         bool
	MergeableServers int
}

type claudeSettings struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

type namedMCPServer struct {
	Name   string
	Config mcpServer
}

type mcpServer struct {
	Command string
	Args    []string
	Env     map[string]string
}

type tomlBlock struct {
	Name  string
	Text  string
	Start int
	End   int
}

type tomlReplacement struct {
	Start int
	End   int
	Text  string
}

func MergeCodexMCP(input CodexMCPInput) (CodexMCPResult, error) {
	current, existing, err := readExistingConfig(input.TargetPath)
	if err != nil {
		return CodexMCPResult{}, err
	}
	blocks, err := findMCPBlocks(string(current))
	if err != nil {
		return CodexMCPResult{}, err
	}
	servers, warnings, err := mergeableServers(input.Scan.Resources)
	if err != nil {
		return CodexMCPResult{}, err
	}

	resolutions := codexMCPResolutionsByFingerprint(input.Resolutions)
	resourcesByName := mcpResourcesBySourceName(input.Scan.Resources)
	replacements := make([]tomlReplacement, 0)
	appended := make([]string, 0, len(servers))
	for _, server := range servers {
		block := formatMCPServerBlock(server.Name, server.Config)
		if existingBlock, ok := blocks[server.Name]; ok {
			oursNormalized := normalizeMCPBlock(block)
			theirsNormalized := normalizeMCPBlock(existingBlock.Text)
			if theirsNormalized == oursNormalized {
				continue
			}
			resource, ok := resourcesByName[server.Name]
			if !ok {
				resource = fallbackMCPResource(server.Name)
			}
			conflict := newCodexMCPConflict(resource, server.Name, input.TargetPath, oursNormalized, theirsNormalized)
			resolution, ok := resolutions[conflict.Fingerprint]
			if !ok {
				return CodexMCPResult{}, unresolvedCodexMCPConflictError(server.Name, conflict.Fingerprint)
			}
			switch resolution.Decision {
			case model.ResolutionDecisionOurs:
				replacements = append(replacements, tomlReplacement{Start: existingBlock.Start, End: existingBlock.End, Text: block})
			case model.ResolutionDecisionTheirs:
				continue
			default:
				return CodexMCPResult{}, unsupportedCodexMCPResolutionError(server.Name, conflict.Fingerprint, resolution.Decision)
			}
			continue
		}
		appended = append(appended, block)
	}

	contents := replaceTOMLBlocks(string(current), replacements)
	contents = appendTOMLBlocks(contents, appended)
	return CodexMCPResult{
		Contents:         []byte(contents),
		Warnings:         warnings,
		Existing:         existing,
		MergeableServers: len(servers),
	}, nil
}

func DetectCodexMCPConflicts(input CodexMCPAnalysisInput) (CodexMCPAnalysis, error) {
	current, existing, err := readExistingConfig(input.TargetPath)
	if err != nil {
		return CodexMCPAnalysis{}, err
	}
	servers, warnings, err := mergeableServers(input.Scan.Resources)
	if err != nil {
		return CodexMCPAnalysis{}, err
	}
	analysis := CodexMCPAnalysis{
		Warnings:         warnings,
		MergeableServers: len(servers),
		Existing:         existing,
	}
	if !existing {
		return analysis, nil
	}

	blocks, err := findMCPBlocks(string(current))
	if err != nil {
		return CodexMCPAnalysis{}, err
	}
	resourcesByName := mcpResourcesBySourceName(input.Scan.Resources)
	for _, server := range servers {
		oursBlock := formatMCPServerBlock(server.Name, server.Config)
		existingBlock, ok := blocks[server.Name]
		if !ok {
			continue
		}
		oursNormalized := normalizeMCPBlock(oursBlock)
		theirsNormalized := normalizeMCPBlock(existingBlock.Text)
		if oursNormalized == theirsNormalized {
			continue
		}
		resource, ok := resourcesByName[server.Name]
		if !ok {
			resource = fallbackMCPResource(server.Name)
		}
		analysis.Conflicts = append(analysis.Conflicts, newCodexMCPConflict(resource, server.Name, input.TargetPath, oursNormalized, theirsNormalized))
	}
	return analysis, nil
}

func codexMCPResolutionsByFingerprint(resolutions []CodexMCPResolution) map[string]CodexMCPResolution {
	out := make(map[string]CodexMCPResolution, len(resolutions))
	for _, resolution := range resolutions {
		if resolution.Fingerprint == "" {
			continue
		}
		out[resolution.Fingerprint] = resolution
	}
	return out
}

func unresolvedCodexMCPConflictError(serverName string, fingerprint string) error {
	return fmt.Errorf("unresolved config merge conflict for Codex MCP server %q (fingerprint %s); rerun agent-canon sync, inspect with agent-canon conflicts, then resolve with agent-canon resolve before retrying apply codex --merge-config", serverName, fingerprint)
}

func unsupportedCodexMCPResolutionError(serverName string, fingerprint string, decision model.ResolutionDecision) error {
	return fmt.Errorf("unsupported config merge resolution %q for Codex MCP server %q (fingerprint %s); apply codex --merge-config supports only ours or theirs decisions for MCP config merge conflicts", decision, serverName, fingerprint)
}

func readExistingConfig(path string) ([]byte, bool, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read Codex config %s: %w", path, err)
	}
	return contents, true, nil
}

func mergeableServers(resources []model.Resource) ([]namedMCPServer, []model.Warning, error) {
	var servers []namedMCPServer
	var warnings []model.Warning
	for _, resource := range resources {
		if resource.Kind != model.KindMCPServer {
			continue
		}
		if resource.SourceName == "" {
			warnings = append(warnings, model.Warning{Code: "mcp-merge-skipped", Message: fmt.Sprintf("MCP resource %s has no source name; skipped config merge", resource.ID)})
			continue
		}
		if resource.Status == model.StatusDangerous || hasWarningCode(resource.Warnings, "secret-redacted") {
			warnings = append(warnings, model.Warning{Code: "mcp-merge-skipped-secret", Message: fmt.Sprintf("MCP server %s contains redacted or secret-looking env; skipped config merge", resource.SourceName)})
			continue
		}
		if resource.Status != model.StatusCompatible && resource.Status != model.StatusPartial {
			warnings = append(warnings, model.Warning{Code: "mcp-merge-skipped", Message: fmt.Sprintf("MCP server %s has status %s; skipped config merge", resource.SourceName, resource.Status)})
			continue
		}
		server, secretKey, err := readClaudeMCPServer(resource)
		if err != nil {
			return nil, warnings, err
		}
		if secretKey != "" {
			warnings = append(warnings, model.Warning{Code: "mcp-merge-skipped-secret", Message: fmt.Sprintf("MCP server %s contains secret-looking env key %s; skipped config merge", resource.SourceName, secretKey)})
			continue
		}
		servers = append(servers, namedMCPServer{Name: resource.SourceName, Config: server})
	}
	sort.SliceStable(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})
	return servers, warnings, nil
}

func mcpResourcesBySourceName(resources []model.Resource) map[string]model.Resource {
	out := map[string]model.Resource{}
	for _, resource := range resources {
		if resource.Kind != model.KindMCPServer || resource.SourceName == "" {
			continue
		}
		if _, exists := out[resource.SourceName]; !exists {
			out[resource.SourceName] = resource
		}
	}
	return out
}

func fallbackMCPResource(serverName string) model.Resource {
	return model.Resource{
		ID:         "mcp:global-" + idSlug(serverName),
		Kind:       model.KindMCPServer,
		Scope:      model.ScopeGlobal,
		SourceTool: "claude",
		TargetTool: "codex",
		SourceName: serverName,
		Status:     model.StatusPartial,
		Strategy:   "manual-mcp-server-review",
	}
}

func newCodexMCPConflict(resource model.Resource, serverName string, targetPath string, oursNormalized string, theirsNormalized string) model.Conflict {
	oursHash := contentSHA256(oursNormalized)
	theirsHash := contentSHA256(theirsNormalized)
	fingerprint := codexMCPConflictFingerprint(resource.ID, serverName, targetPath, oursHash, theirsHash)
	details := map[string]string{
		"serverName": serverName,
		"targetPath": targetPath,
		"reason":     "same-name Codex MCP server exists with different normalized configuration",
	}
	if resource.SourcePath != "" {
		details["sourcePath"] = resource.SourcePath
	}
	return model.Conflict{
		ID:                   "conflict:" + fingerprint,
		Kind:                 model.ConflictKindConfigMerge,
		ResourceID:           resource.ID,
		ResourceKind:         model.KindMCPServer,
		Scope:                resource.Scope,
		Ours:                 codexMCPConflictState(resource.ID, resource.Scope, "claude", resource.SourcePath, resource.TargetPathHint, resource.Status, resource.Strategy, serverName, oursNormalized),
		Theirs:               codexMCPConflictState(resource.ID, resource.Scope, "codex", targetPath, "", model.StatusPartial, "manual-mcp-server-review", serverName, theirsNormalized),
		RequiresUserDecision: true,
		Status:               model.ConflictStatusOpen,
		Fingerprint:          fingerprint,
		Details:              details,
	}
}

func codexMCPConflictState(resourceID string, scope model.Scope, tool string, path string, targetPathHint string, status model.Status, strategy string, serverName string, normalized string) *model.ResourceState {
	_, redacted := security.RedactContent(normalized)
	summary := fmt.Sprintf("MCP server %q normalized configuration summary; sha256=%s", serverName, contentSHA256(normalized))
	if redacted {
		summary += "; sensitive-looking content redacted from summary"
	}
	return &model.ResourceState{
		ID:             resourceID,
		Kind:           model.KindMCPServer,
		Scope:          scope,
		Tool:           tool,
		Path:           path,
		TargetPathHint: targetPathHint,
		Status:         status,
		Strategy:       strategy,
		ContentHash:    contentSHA256(normalized),
		NormalizedText: summary,
	}
}

func codexMCPConflictFingerprint(resourceID string, serverName string, targetPath string, oursHash string, theirsHash string) string {
	payload := strings.Join([]string{
		string(model.ConflictKindConfigMerge),
		resourceID,
		serverName,
		targetPath,
		oursHash,
		theirsHash,
	}, "\x00")
	return "config-merge:" + contentSHA256(payload)
}

func contentSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func idSlug(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func readClaudeMCPServer(resource model.Resource) (mcpServer, string, error) {
	payload, err := os.ReadFile(resource.SourcePath)
	if err != nil {
		return mcpServer{}, "", fmt.Errorf("read Claude settings for MCP server %s: %w", resource.ID, err)
	}
	var settings claudeSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		return mcpServer{}, "", fmt.Errorf("parse Claude settings for MCP server %s: %w", resource.ID, err)
	}
	raw, ok := settings.MCPServers[resource.SourceName]
	if !ok {
		return mcpServer{}, "", fmt.Errorf("MCP server %s not found in Claude settings", resource.SourceName)
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return mcpServer{}, "", fmt.Errorf("MCP server %s must be a JSON object", resource.SourceName)
	}
	command, err := readStringField(object, "command", true)
	if err != nil {
		return mcpServer{}, "", fmt.Errorf("MCP server %s: %w", resource.SourceName, err)
	}
	args, err := readStringArrayField(object, "args")
	if err != nil {
		return mcpServer{}, "", fmt.Errorf("MCP server %s: %w", resource.SourceName, err)
	}
	env, secretKey, err := readStringEnvField(object, "env")
	if err != nil {
		return mcpServer{}, "", fmt.Errorf("MCP server %s: %w", resource.SourceName, err)
	}
	return mcpServer{Command: command, Args: args, Env: env}, secretKey, nil
}

func readStringField(object map[string]json.RawMessage, name string, required bool) (string, error) {
	raw, ok := object[name]
	if !ok {
		if required {
			return "", fmt.Errorf("missing %q", name)
		}
		return "", nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", fmt.Errorf("%q must be a string", name)
	}
	if required && strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%q must be a non-empty string", name)
	}
	return value, nil
}

func readStringArrayField(object map[string]json.RawMessage, name string) ([]string, error) {
	raw, ok := object[name]
	if !ok {
		return nil, nil
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("%q must be a string array", name)
	}
	out := make([]string, 0, len(values))
	for i, rawValue := range values {
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, fmt.Errorf("%q[%d] must be a string", name, i)
		}
		out = append(out, value)
	}
	return out, nil
}

func readStringEnvField(object map[string]json.RawMessage, name string) (map[string]string, string, error) {
	raw, ok := object[name]
	if !ok {
		return nil, "", nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, "", fmt.Errorf("%q must be an object with string values", name)
	}
	out := make(map[string]string, len(values))
	for key, rawValue := range values {
		var value string
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, "", fmt.Errorf("%q.%s must be a string", name, key)
		}
		if isSecretEnv(key, value) {
			return nil, key, nil
		}
		out[key] = value
	}
	return out, "", nil
}

func isSecretEnv(key string, value string) bool {
	if _, redacted := security.RedactIfSecret(key, value); redacted {
		return true
	}
	_, redacted := security.RedactContent(key + "=" + value)
	return redacted
}

func hasWarningCode(warnings []model.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func formatMCPServerBlock(name string, server mcpServer) string {
	var builder strings.Builder
	builder.WriteString("[mcp_servers.")
	builder.WriteString(tomlString(name))
	builder.WriteString("]\n")
	builder.WriteString("command = ")
	builder.WriteString(tomlString(server.Command))
	builder.WriteByte('\n')
	if len(server.Args) > 0 {
		builder.WriteString("args = [")
		for i, arg := range server.Args {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(tomlString(arg))
		}
		builder.WriteString("]\n")
	}
	if len(server.Env) > 0 {
		keys := make([]string, 0, len(server.Env))
		for key := range server.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		builder.WriteString("env = { ")
		for i, key := range keys {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(tomlString(key))
			builder.WriteString(" = ")
			builder.WriteString(tomlString(server.Env[key]))
		}
		builder.WriteString(" }\n")
	}
	return builder.String()
}

func tomlString(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\b':
			builder.WriteString(`\b`)
		case '\t':
			builder.WriteString(`\t`)
		case '\n':
			builder.WriteString(`\n`)
		case '\f':
			builder.WriteString(`\f`)
		case '\r':
			builder.WriteString(`\r`)
		case '"':
			builder.WriteString(`\"`)
		case '\\':
			builder.WriteString(`\\`)
		default:
			if unicode.IsControl(r) {
				builder.WriteString(fmt.Sprintf(`\u%04X`, r))
			} else {
				builder.WriteRune(r)
			}
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func findMCPBlocks(contents string) (map[string]tomlBlock, error) {
	lines := strings.SplitAfter(contents, "\n")
	blocks := map[string]tomlBlock{}
	currentName := ""
	currentStart := 0
	seen := map[string]bool{}
	for i, line := range lines {
		if !isTOMLTableBoundary(line) {
			continue
		}
		if currentName != "" {
			child, err := isMCPServerChildHeader(line, currentName)
			if err != nil {
				return nil, err
			}
			if child {
				continue
			}
			end := trimTrailingBlankLines(lines, currentStart, i)
			blocks[currentName] = tomlBlock{Name: currentName, Text: strings.Join(lines[currentStart:end], ""), Start: currentStart, End: end}
			currentName = ""
		}
		name, ok, err := parseMCPServerHeader(line)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if seen[name] {
			return nil, fmt.Errorf("config-merge-conflict: Codex MCP server %q is defined more than once", name)
		}
		seen[name] = true
		currentName = name
		currentStart = i
	}
	if currentName != "" {
		end := trimTrailingBlankLines(lines, currentStart, len(lines))
		blocks[currentName] = tomlBlock{Name: currentName, Text: strings.Join(lines[currentStart:end], ""), Start: currentStart, End: end}
	}
	return blocks, nil
}

func trimTrailingBlankLines(lines []string, start int, end int) int {
	for end > start+1 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return end
}

func isTOMLTableBoundary(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "]")
}

func isTOMLTableHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[") && strings.Contains(trimmed, "]")
}

func parseMCPServerHeader(line string) (string, bool, error) {
	parts, ok, err := parseTOMLTableHeader(line)
	if err != nil || !ok {
		return "", false, err
	}
	if len(parts) != 2 || parts[0] != "mcp_servers" {
		return "", false, nil
	}
	return parts[1], true, nil
}

func isMCPServerChildHeader(line string, serverName string) (bool, error) {
	parts, ok, err := parseTOMLTableHeader(line)
	if err != nil || !ok {
		return false, err
	}
	return len(parts) > 2 && parts[0] == "mcp_servers" && parts[1] == serverName, nil
}

func parseTOMLTableHeader(line string) ([]string, bool, error) {
	if !isTOMLTableHeader(line) {
		return nil, false, nil
	}
	trimmed := strings.TrimSpace(line)
	end := strings.IndexByte(trimmed, ']')
	parts, ok := splitTOMLDottedKey(trimmed[1:end])
	if !ok {
		return nil, false, fmt.Errorf("parse TOML table header %q", strings.TrimSpace(line))
	}
	return parts, true, nil
}

func splitTOMLDottedKey(value string) ([]string, bool) {
	var parts []string
	for {
		value = strings.TrimLeftFunc(value, unicode.IsSpace)
		if value == "" {
			return parts, len(parts) > 0
		}
		var part string
		var ok bool
		if value[0] == '"' {
			part, value, ok = consumeQuotedKey(value)
			if !ok {
				return nil, false
			}
		} else {
			idx := strings.IndexByte(value, '.')
			if idx < 0 {
				part = strings.TrimSpace(value)
				value = ""
			} else {
				part = strings.TrimSpace(value[:idx])
				value = value[idx:]
			}
			if part == "" {
				return nil, false
			}
		}
		parts = append(parts, part)
		value = strings.TrimLeftFunc(value, unicode.IsSpace)
		if value == "" {
			return parts, true
		}
		if value[0] != '.' {
			return nil, false
		}
		value = value[1:]
	}
}

func consumeQuotedKey(value string) (string, string, bool) {
	escaped := false
	for i := 1; i < len(value); i++ {
		switch {
		case escaped:
			escaped = false
		case value[i] == '\\':
			escaped = true
		case value[i] == '"':
			unquoted, err := strconv.Unquote(value[:i+1])
			if err != nil {
				return "", "", false
			}
			return unquoted, value[i+1:], true
		}
	}
	return "", "", false
}

func normalizeMCPBlock(block string) string {
	var normalized []string
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if left, right, ok := strings.Cut(trimmed, "="); ok {
			trimmed = strings.TrimSpace(left) + " = " + strings.TrimSpace(right)
		}
		normalized = append(normalized, trimmed)
	}
	return strings.Join(normalized, "\n")
}

func replaceTOMLBlocks(contents string, replacements []tomlReplacement) string {
	if len(replacements) == 0 {
		return contents
	}
	lines := strings.SplitAfter(contents, "\n")
	sort.SliceStable(replacements, func(i, j int) bool {
		return replacements[i].Start > replacements[j].Start
	})
	for _, replacement := range replacements {
		text := replacement.Text
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		lines = append(lines[:replacement.Start], append([]string{text}, lines[replacement.End:]...)...)
	}
	return strings.Join(lines, "")
}

func appendTOMLBlocks(contents string, blocks []string) string {
	if len(blocks) == 0 {
		return contents
	}
	out := strings.TrimRight(contents, "\n")
	if strings.TrimSpace(out) != "" {
		out += "\n\n"
	}
	for i, block := range blocks {
		if i > 0 {
			out += "\n"
		}
		out += strings.TrimRight(block, "\n") + "\n"
	}
	return out
}
