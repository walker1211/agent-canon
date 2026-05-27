package apply

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zhangyoujun/agent-canon/internal/security"
)

func RedactedDiff(change FileChange) (string, error) {
	before := "(missing)"
	current, err := os.ReadFile(change.Path)
	if err == nil {
		before, _ = security.RedactContent(string(current))
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read apply target %s: %w", change.Path, err)
	}
	after, _ := security.RedactContent(string(change.Contents))
	return strings.Join([]string{
		fmt.Sprintf("--- %s", change.Path),
		before,
		fmt.Sprintf("+++ %s", change.Path),
		after,
	}, "\n"), nil
}
