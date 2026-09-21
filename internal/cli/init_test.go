package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAllEditors(t *testing.T) {
	dir := t.TempDir()
	app, out, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "init"}); code != 0 {
		t.Fatalf("init exit %d stderr=%s", code, errw)
	}
	yamlPath := filepath.Join(dir, "imprint.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("missing imprint.yaml: %v", err)
	}
	want := []string{
		filepath.Join(dir, ".cursor", "rules", "imprint-memory.mdc"),
		filepath.Join(dir, ".claude", "rules", "imprint-memory.md"),
		filepath.Join(dir, ".trae", "rules", "imprint-memory.md"),
		filepath.Join(dir, ".codebuddy", "rules", "imprint-memory", "RULE.mdc"),
		filepath.Join(dir, "AGENTS.md"),
	}
	for _, p := range want {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if !strings.Contains(string(raw), "imprint memory") {
			t.Fatalf("%s missing body: %s", p, raw)
		}
	}
	agents, _ := os.ReadFile(want[4])
	if !strings.Contains(string(agents), imprintAgentsSection) {
		t.Fatalf("AGENTS.md missing imprint section:\n%s", agents)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "mcp.json")); !os.IsNotExist(err) {
		t.Fatal("init must not write mcp.json")
	}
	_ = out
}

func TestInitSingleEditor(t *testing.T) {
	dir := t.TempDir()
	app, _, errw := testApp(t, dir)
	if code := app.Run([]string{"--json", "init", "--trae"}); code != 0 {
		t.Fatalf("init trae %d %s", code, errw)
	}
	if _, err := os.Stat(filepath.Join(dir, ".trae", "rules", "imprint-memory.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "rules", "imprint-memory.mdc")); !os.IsNotExist(err) {
		t.Fatal("cursor rule should not exist")
	}
}

func TestInitCodexMergeExistingAgents(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(existing, []byte("# Team\n\nBuild: make test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, errw := testApp(t, dir)
	if code := app.Run([]string{"init", "--codex", "--force"}); code != 0 {
		t.Fatalf("init codex force %d %s", code, errw)
	}
	raw, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "Build: make test") || !strings.Contains(text, imprintAgentsSection) {
		t.Fatalf("merge failed:\n%s", text)
	}
}

func TestInitRefuseWithoutForce(t *testing.T) {
	dir := t.TempDir()
	app, _, _ := testApp(t, dir)
	if code := app.Run([]string{"--json", "init", "--cursor"}); code != 0 {
		t.Fatalf("first init %d", code)
	}
	if code := app.Run([]string{"init", "--cursor"}); code == 0 {
		t.Fatal("expected refuse second init without force")
	}
}
