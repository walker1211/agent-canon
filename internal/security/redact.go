package security

import (
	"regexp"
	"strings"
)

const RedactedValue = "<REDACTED>"

var secretKeyMarkers = []string{
	"secret",
	"token",
	"password",
	"auth",
	"credential",
	"private_key",
	"api_key",
	"openai_api_key",
	"anthropic_api_key",
	"github_token",
}

var inlineSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{8,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func IsSecretKey(key string) bool {
	normalized := strings.ToLower(key)
	for _, marker := range secretKeyMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func RedactIfSecret(key string, value string) (string, bool) {
	if !IsSecretKey(key) {
		return value, false
	}
	return RedactedValue, true
}

func RedactContent(contents string) (string, bool) {
	result := contents
	redacted := false
	for _, pattern := range inlineSecretPatterns {
		next := pattern.ReplaceAllString(result, RedactedValue)
		if next != result {
			redacted = true
		}
		result = next
	}

	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if next, ok := redactContentLine(line); ok {
			lines[i] = next
			redacted = true
		}
	}
	return strings.Join(lines, "\n"), redacted
}

func redactContentLine(line string) (string, bool) {
	if key, _, ok := strings.Cut(line, ":"); ok && IsSecretKey(strings.TrimSpace(key)) {
		return key + ": " + RedactedValue, true
	}

	trimmed := strings.TrimLeft(line, " \t")
	leading := line[:len(line)-len(trimmed)]
	fields := strings.Fields(trimmed)
	changed := false
	for i, field := range fields {
		key, _, ok := strings.Cut(field, "=")
		if !ok || !IsSecretKey(strings.TrimSpace(key)) {
			continue
		}
		fields[i] = key + "=" + RedactedValue
		changed = true
	}
	if !changed {
		return "", false
	}
	return leading + strings.Join(fields, " "), true
}
