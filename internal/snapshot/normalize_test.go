package snapshot

import (
	"strings"
	"testing"

	"github.com/zhangyoujun/agent-canon/internal/security"
)

func TestNormalizeContentTrimsWhitespaceAndCollapsesBlankLines(t *testing.T) {
	input := []byte("\r\n  first line  \r\n\r\n\r\nsecond line\t\r\n\r\n")

	got := NormalizeContent(input)

	if got.Text != "first line\n\nsecond line" {
		t.Fatalf("NormalizeContent text = %q, want %q", got.Text, "first line\n\nsecond line")
	}
	if got.SecretRedacted {
		t.Fatalf("NormalizeContent SecretRedacted = true, want false")
	}
}

func TestNormalizeContentRedactsSecretsBeforeHashing(t *testing.T) {
	input := []byte(strings.Join([]string{
		"GITHUB_TOKEN=ghp_1234567890abcdef",
		"private_key: -----BEGIN PRIVATE KEY-----fixture-----END PRIVATE KEY-----",
		"body",
	}, "\n"))

	got := NormalizeContent(input)

	if !got.SecretRedacted {
		t.Fatalf("NormalizeContent SecretRedacted = false, want true")
	}
	for _, leaked := range []string{"ghp_1234567890abcdef", "fixture", "PRIVATE KEY"} {
		if strings.Contains(got.Text, leaked) {
			t.Fatalf("NormalizeContent leaked %q in %q", leaked, got.Text)
		}
	}
	if !strings.Contains(got.Text, security.RedactedValue) {
		t.Fatalf("NormalizeContent text = %q, want redacted marker", got.Text)
	}
}

func TestNormalizeContentStableHashesEquivalentText(t *testing.T) {
	first := NormalizeContent([]byte("alpha  \r\n\r\n\r\nbeta\n"))
	second := NormalizeContent([]byte("\nalpha\n\nbeta   \n"))

	if first.Text != second.Text {
		t.Fatalf("normalized texts differ: %q vs %q", first.Text, second.Text)
	}
	if first.ContentHash == "" {
		t.Fatalf("ContentHash is empty")
	}
	if len(first.ContentHash) != 64 {
		t.Fatalf("ContentHash length = %d, want 64", len(first.ContentHash))
	}
	if first.ContentHash != second.ContentHash {
		t.Fatalf("ContentHash = %q, want %q", first.ContentHash, second.ContentHash)
	}
}
