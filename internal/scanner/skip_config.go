package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/model"
)

type skipConfig struct {
	Path      string
	Resources map[string]bool
	Paths     map[string]bool
}

func loadSkipConfig(project string) (skipConfig, error) {
	path := filepath.Join(project, ".agent-canon", "config.toml")
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return skipConfig{}, nil
	}
	if err != nil {
		return skipConfig{}, fmt.Errorf("read agent-canon config %q: %w", path, err)
	}

	resources, paths, err := parseSkipConfig(string(payload))
	if err != nil {
		return skipConfig{}, fmt.Errorf("parse agent-canon config %q: %w", path, err)
	}
	config := skipConfig{
		Path:      path,
		Resources: map[string]bool{},
		Paths:     map[string]bool{},
	}
	for _, resource := range resources {
		resource = strings.TrimSpace(resource)
		if resource != "" {
			config.Resources[resource] = true
		}
	}
	for _, pathValue := range paths {
		pathValue = strings.TrimSpace(pathValue)
		if pathValue == "" {
			continue
		}
		clean, err := normalizeSkipPath(project, pathValue)
		if err != nil {
			return skipConfig{}, err
		}
		config.Paths[clean] = true
	}
	return config, nil
}

func parseSkipConfig(contents string) ([]string, []string, error) {
	var resources []string
	var paths []string
	section := ""
	lines := strings.Split(contents, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripLineComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "skip" {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, nil, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "[") && !strings.Contains(value, "]") {
			var parts []string
			parts = append(parts, value)
			for {
				i++
				if i >= len(lines) {
					return nil, nil, fmt.Errorf("line %d: unterminated array for %s", i, key)
				}
				next := strings.TrimSpace(stripLineComment(lines[i]))
				parts = append(parts, next)
				if strings.Contains(next, "]") {
					break
				}
			}
			value = strings.Join(parts, "\n")
		}
		values, err := parseStringArray(value)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %s: %w", i+1, key, err)
		}
		switch key {
		case "resources":
			resources = append(resources, values...)
		case "paths":
			paths = append(paths, values...)
		default:
			return nil, nil, fmt.Errorf("line %d: unsupported [skip] key %q", i+1, key)
		}
	}
	return resources, paths, nil
}

func stripLineComment(line string) string {
	inQuote := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == '#' && !inQuote {
			return line[:i]
		}
	}
	return line
}

func parseStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") {
		return nil, fmt.Errorf("expected string array")
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	if inner == "" {
		return nil, nil
	}

	var values []string
	for _, raw := range splitArrayValues(inner) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.HasPrefix(raw, "\"") || !strings.HasSuffix(raw, "\"") {
			return nil, fmt.Errorf("array values must be double-quoted strings")
		}
		value, err := strconv.Unquote(raw)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func splitArrayValues(value string) []string {
	var values []string
	start := 0
	inQuote := false
	escaped := false
	for i, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inQuote {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			values = append(values, value[start:i])
			start = i + 1
		}
	}
	values = append(values, value[start:])
	return values
}

func normalizeSkipPath(project string, pathValue string) (string, error) {
	if pathValue == "~" || strings.HasPrefix(pathValue, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand skip path %q: %w", pathValue, err)
		}
		if pathValue == "~" {
			pathValue = home
		} else {
			pathValue = filepath.Join(home, strings.TrimPrefix(pathValue, "~/"))
		}
	}
	pathValue = filepath.FromSlash(pathValue)
	if !filepath.IsAbs(pathValue) {
		pathValue = filepath.Join(project, pathValue)
	}
	abs, err := filepath.Abs(pathValue)
	if err != nil {
		return "", fmt.Errorf("resolve skip path %q: %w", pathValue, err)
	}
	return filepath.Clean(abs), nil
}

func applySkipConfig(report *model.ScanReport, config skipConfig) {
	if len(config.Resources) == 0 && len(config.Paths) == 0 {
		return
	}

	filtered := make([]model.Resource, 0, len(report.Resources))
	skipped := 0
	for _, resource := range report.Resources {
		if config.skips(resource) {
			skipped++
			continue
		}
		filtered = append(filtered, resource)
	}
	report.Resources = filtered
	if skipped == 0 {
		return
	}
	report.Warnings = append(report.Warnings, model.Warning{
		Code:    "skip-config-applied",
		Message: fmt.Sprintf("skipped %d resources using %s", skipped, config.Path),
	})
}

func (c skipConfig) skips(resource model.Resource) bool {
	if c.Resources[resource.ID] {
		return true
	}
	for _, path := range []string{resource.SourcePath, resource.TargetPathHint} {
		if path == "" {
			continue
		}
		if c.Paths[filepath.Clean(path)] {
			return true
		}
	}
	return false
}
