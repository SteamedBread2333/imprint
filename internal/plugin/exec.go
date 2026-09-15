package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// execStartCommand runs manifest entry.start. Authors write shell lines (e.g. npm && node);
// simple commands without metacharacters run directly without a shell.
func execStartCommand(ctx context.Context, startLine string) (*exec.Cmd, error) {
	startLine = strings.TrimSpace(startLine)
	if startLine == "" {
		return nil, fmt.Errorf("empty start command")
	}
	if startNeedsShell(startLine) {
		if runtime.GOOS == "windows" {
			return exec.CommandContext(ctx, "cmd", "/c", startLine), nil
		}
		return exec.CommandContext(ctx, "sh", "-c", startLine), nil
	}
	parts := strings.Fields(startLine)
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid start command")
	}
	return exec.CommandContext(ctx, parts[0], parts[1:]...), nil
}

func startNeedsShell(line string) bool {
	if strings.Contains(line, "&&") || strings.Contains(line, "||") {
		return true
	}
	for _, r := range line {
		switch r {
		case '|', ';', '>', '<', '`', '$', '(', ')', '\'', '"':
			return true
		}
	}
	return false
}
