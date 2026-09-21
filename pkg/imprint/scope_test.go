package imprint

import "testing"

func TestCanonicalScopeLanguageAliases(t *testing.T) {
	cases := map[string]string{
		"ts":         "typescript",
		"tsx":        "typescript",
		"typescript": "typescript",
		"js":         "javascript",
		"jsx":        "javascript",
		"golang":     "go",
		"go":         "go",
		"py":         "python",
		"python":     "python",
		"frontend":   "frontend",
		"naming":     "naming",
		"docs":       "docs",
		"md":         "markdown",
	}
	for in, want := range cases {
		if got := CanonicalScope(in); got != want {
			t.Errorf("CanonicalScope(%q) = %q, want %q", in, got, want)
		}
	}
}
