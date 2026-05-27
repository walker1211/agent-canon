package security

import "strings"

const RedactedValue = "<REDACTED>"

var secretKeyMarkers = []string{
	"secret",
	"token",
	"password",
	"auth",
	"credential",
	"api_key",
	"openai_api_key",
	"anthropic_api_key",
	"github_token",
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
