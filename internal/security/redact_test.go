package security

import "testing"

func TestIsSecretKeyMatchesKnownKeysCaseInsensitively(t *testing.T) {
	tests := []string{
		"secret",
		"ClientSecret",
		"token",
		"GitHub_Token",
		"password",
		"AUTH_HEADER",
		"credential_path",
		"api_key",
		"OPENAI_API_KEY",
		"anthropic_api_key",
		"github_token",
	}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			if !IsSecretKey(key) {
				t.Fatalf("IsSecretKey(%q) = false, want true", key)
			}
		})
	}
}

func TestIsSecretKeyIgnoresNonSecretKeys(t *testing.T) {
	tests := []string{
		"command",
		"args",
		"endpoint",
		"model",
		"timeout",
	}

	for _, key := range tests {
		t.Run(key, func(t *testing.T) {
			if IsSecretKey(key) {
				t.Fatalf("IsSecretKey(%q) = true, want false", key)
			}
		})
	}
}

func TestRedactIfSecretReplacesOnlySecretValues(t *testing.T) {
	got, redacted := RedactIfSecret("GITHUB_TOKEN", "fixture-value")
	if !redacted {
		t.Fatalf("RedactIfSecret redacted = false, want true")
	}
	if got != RedactedValue {
		t.Fatalf("RedactIfSecret value = %q, want %q", got, RedactedValue)
	}

	got, redacted = RedactIfSecret("command", "fixture-value")
	if redacted {
		t.Fatalf("RedactIfSecret non-secret redacted = true, want false")
	}
	if got != "fixture-value" {
		t.Fatalf("RedactIfSecret non-secret value = %q, want original", got)
	}
}
