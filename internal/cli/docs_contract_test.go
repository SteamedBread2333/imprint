package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/SteamedBread2333/imprint/internal/plugin"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestDocsContractYAMLAndHelp(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"imprint.yaml", "internal/cli/templates/imprint.yaml", "docs/examples/imprint.yaml"} {
		cfg, err := plugin.Load(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if !cfg.Telemetry.Enabled || cfg.Telemetry.RetentionDays != 30 {
			t.Fatalf("%s telemetry = %+v", rel, cfg.Telemetry)
		}
	}
	help := commandHelp("report")
	if !strings.Contains(help, "--days") {
		t.Fatalf("report help missing --days: %s", help)
	}
	if !strings.Contains(commandHelp("reinforce"), "--evidence TEXT") {
		t.Fatalf("reinforce help should require evidence")
	}
	if !strings.Contains(rootHelp, "report") {
		t.Fatal("root help missing report")
	}
}

func TestEditorRuleMatchesBody(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "internal/cli/templates/body.md"))
	if err != nil {
		t.Fatal(err)
	}
	rule, err := os.ReadFile(filepath.Join(root, ".cursor/rules/imprint-memory.mdc"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(rule)
	if i := strings.Index(text, "# imprint memory"); i >= 0 {
		text = text[i:]
	}
	if strings.TrimSpace(string(body)) != strings.TrimSpace(text) {
		t.Fatal("body.md and .cursor/rules/imprint-memory.mdc drifted")
	}
}

func TestDocsHaveNoLegacyTerms(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		"README.md", "README.zh.md",
		"docs/mcp.md", "docs/mcp.zh.md",
		"docs/correction.md", "docs/correction.zh.md",
		"docs/shelves-builtin.md", "docs/shelves-builtin.zh.md",
		"docs/testing.md", "docs/testing.zh.md",
		"internal/cli/templates/body.md",
		".cursor/rules/imprint-memory.mdc",
	}
	legacy := []string{"last_touched_at", "+0.1 (cap", "confidence +0.1", "schema_version', '3'", "scopes.aliases"}
	for _, rel := range paths {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		for _, term := range legacy {
			if strings.Contains(text, term) {
				t.Errorf("%s still contains %q", rel, term)
			}
		}
	}
}
