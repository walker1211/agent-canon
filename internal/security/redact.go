package security

import (
	"regexp"
	"strings"
	"unicode"
)

const RedactedValue = "<REDACTED>"

var inlineSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{8,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func IsSecretKey(key string) bool {
	words := keyWords(key)
	for i, word := range words {
		switch word {
		case "secret", "password", "auth", "authorization", "credential", "credentials":
			return true
		case "token":
			if isSecretTokenWord(words, i) {
				return true
			}
		}
	}
	return hasAdjacentWords(words, "private", "key") || hasAdjacentWords(words, "api", "key")
}

func keyWords(key string) []string {
	var out strings.Builder
	var previous rune
	for i, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if i > 0 && unicode.IsUpper(r) && (unicode.IsLower(previous) || unicode.IsDigit(previous)) {
				out.WriteByte(' ')
			}
			out.WriteRune(unicode.ToLower(r))
		} else {
			out.WriteByte(' ')
		}
		previous = r
	}
	return strings.Fields(out.String())
}

func hasAdjacentWords(words []string, first string, second string) bool {
	for i := 0; i+1 < len(words); i++ {
		if words[i] == first && words[i+1] == second {
			return true
		}
	}
	return false
}

func isSecretTokenWord(words []string, index int) bool {
	if len(words) == 1 {
		return true
	}
	if index > 0 && isTokenQualifier(words[index-1]) {
		return true
	}
	if index > 1 && words[index-2] == "git" && (words[index-1] == "hub" || words[index-1] == "lab") {
		return true
	}
	return index+1 < len(words) && words[index+1] == "value"
}

func isTokenQualifier(word string) bool {
	switch word {
	case "access", "auth", "bearer", "github", "gitlab", "id", "refresh", "slack":
		return true
	default:
		return false
	}
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
