package security

import (
	"strings"
	"testing"
)

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

func TestRedactContentRedactsInlineTokensAndPrivateKeys(t *testing.T) {
	githubPAT := "github_pat_11ABCDEFG0abcdefghijklmnopqrstuvwxyz_1234567890ABCDE"
	input := "github=ghp_1234567890abcdef\nfinegrained=" + githubPAT + "\nopenai=sk-1234567890abcdefghijklmnop\n-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\n"

	got, redacted := RedactContent(input)
	if !redacted {
		t.Fatalf("RedactContent redacted = false, want true")
	}
	for _, leaked := range []string{"ghp_1234567890abcdef", githubPAT, "sk-1234567890abcdefghijklmnop", "PRIVATE KEY", "secret"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("RedactContent leaked %q in %q", leaked, got)
		}
	}
	if count := strings.Count(got, RedactedValue); count != 4 {
		t.Fatalf("RedactContent redaction count = %d, want 4 in %q", count, got)
	}
}

func TestRedactContentRedactsSecretAssignmentLines(t *testing.T) {
	input := strings.Join([]string{
		"token: fixture-token",
		"PRIVATE_KEY=base64-key-material",
		"private_key: base64-key-material",
		"export OPENAI_API_KEY=fixture-key command=kept",
		"FOO=bar GITHUB_TOKEN=fixture-token run",
		"command=kept",
	}, "\n")

	got, redacted := RedactContent(input)
	if !redacted {
		t.Fatalf("RedactContent redacted = false, want true")
	}
	want := strings.Join([]string{
		"token: <REDACTED>",
		"PRIVATE_KEY=<REDACTED>",
		"private_key: <REDACTED>",
		"export OPENAI_API_KEY=<REDACTED> command=kept",
		"FOO=bar GITHUB_TOKEN=<REDACTED> run",
		"command=kept",
	}, "\n")
	if got != want {
		t.Fatalf("RedactContent = %q, want %q", got, want)
	}
}

func TestRedactContentRedactsMultilinePrivateKeyAssignments(t *testing.T) {
	input := strings.Join([]string{
		"PRIVATE_KEY: -----BEGIN PRIVATE KEY-----",
		"fixture-secret-key-body",
		"-----END PRIVATE KEY-----",
	}, "\n")

	got, redacted := RedactContent(input)
	if !redacted {
		t.Fatalf("RedactContent redacted = false, want true")
	}
	for _, leaked := range []string{"fixture-secret-key-body", "BEGIN PRIVATE KEY", "END PRIVATE KEY"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("RedactContent leaked %q in %q", leaked, got)
		}
	}
	if got != "PRIVATE_KEY: <REDACTED>" {
		t.Fatalf("RedactContent = %q, want private key assignment marker redacted", got)
	}
}
