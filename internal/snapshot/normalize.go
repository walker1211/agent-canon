package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/security"
)

type NormalizedContent struct {
	Text           string
	ContentHash    string
	SecretRedacted bool
}

func NormalizeContent(contents []byte) NormalizedContent {
	redacted, secretRedacted := security.RedactContent(string(contents))
	text := NormalizeText(redacted)
	hash := sha256.Sum256([]byte(text))
	return NormalizedContent{
		Text:           text,
		ContentHash:    hex.EncodeToString(hash[:]),
		SecretRedacted: secretRedacted,
	}
}

func NormalizeText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	text = strings.TrimSpace(strings.Join(lines, "\n"))
	if text == "" {
		return ""
	}

	lines = strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	lastBlank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if !lastBlank {
				out = append(out, "")
			}
			lastBlank = true
			continue
		}
		out = append(out, line)
		lastBlank = false
	}
	return strings.Join(out, "\n")
}
