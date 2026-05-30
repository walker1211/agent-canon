package configmerge

import (
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

type CodexMCPInput struct {
	Scan       model.ScanReport
	TargetPath string
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
	Name string
	Text string
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

	appended := make([]string, 0, len(servers))
	for _, server := range servers {
		block := formatMCPServerBlock(server.Name, server.Config)
		if existingBlock, ok := blocks[server.Name]; ok {
			if normalizeMCPBlock(existingBlock.Text) == normalizeMCPBlock(block) {
				continue
			}
			return CodexMCPResult{}, fmt.Errorf("config-merge-conflict: Codex MCP server %q already exists with different configuration", server.Name)
		}
		appended = append(appended, block)
	}

	contents := appendTOMLBlocks(string(current), appended)
	return CodexMCPResult{
		Contents:         []byte(contents),
		Warnings:         warnings,
		Existing:         existing,
		MergeableServers: len(servers),
	}, nil
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
	for i, line := range lines {
		if !isTOMLTableHeader(line) {
			continue
		}
		if currentName != "" {
			blocks[currentName] = tomlBlock{Name: currentName, Text: strings.Join(lines[currentStart:i], "")}
			currentName = ""
		}
		name, ok, err := parseMCPServerHeader(line)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if _, exists := blocks[name]; exists {
			return nil, fmt.Errorf("config-merge-conflict: Codex MCP server %q is defined more than once", name)
		}
		currentName = name
		currentStart = i
	}
	if currentName != "" {
		blocks[currentName] = tomlBlock{Name: currentName, Text: strings.Join(lines[currentStart:], "")}
	}
	return blocks, nil
}

func isTOMLTableHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[[") && strings.Contains(trimmed, "]")
}

func parseMCPServerHeader(line string) (string, bool, error) {
	trimmed := strings.TrimSpace(line)
	end := strings.IndexByte(trimmed, ']')
	if end < 0 || !strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "[[") {
		return "", false, nil
	}
	parts, ok := splitTOMLDottedKey(trimmed[1:end])
	if !ok {
		return "", false, fmt.Errorf("parse TOML table header %q", strings.TrimSpace(line))
	}
	if len(parts) != 2 || parts[0] != "mcp_servers" {
		return "", false, nil
	}
	return parts[1], true, nil
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
