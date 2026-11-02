// Package argx parses the string arguments shared by the CLI, MCP tools, and host HTTP API.
package argx

import (
	"fmt"
	"strings"
	"time"
)

// SplitCSV splits a comma-separated list, trimming blanks and dropping empty items.
func SplitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ParseSince accepts YYYY-MM-DD or RFC3339; empty input is the zero time.
// name labels the argument in errors (for example "--since" or "since").
func ParseSince(s, name string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid %s %q (use YYYY-MM-DD or RFC3339)", name, s)
}
