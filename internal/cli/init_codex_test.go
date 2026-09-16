package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexInstructionPathPrefersOverride(t *testing.T) {
	dir := t.TempDir()
	agents := filepath.Join(dir, "AGENTS.md")
	override := filepath.Join(dir, "AGENTS.override.md")
	if err := os.WriteFile(agents, []byte("# team\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := codexInstructionPath(dir); got != agents {
		t.Fatalf("empty override: got %q want %q", got, agents)
	}
	if err := os.WriteFile(override, []byte("# override\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := codexInstructionPath(dir); got != override {
		t.Fatalf("non-empty override: got %q want %q", got, override)
	}
}

func TestCodexSectionMergeAndUpdate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(path, []byte("## Repository expectations\n\nRun make test\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeCodexInit(dir, true); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "Run make test") || !strings.Contains(text, imprintAgentsSection) {
		t.Fatalf("append failed:\n%s", text)
	}
	if strings.Contains(text, "<!-- imprint-memory") {
		t.Fatal("should not use HTML comment markers")
	}
	if _, err := writeCodexInit(dir, false); err != nil {
		t.Fatal(err)
	}
	raw2, _ := os.ReadFile(path)
	if !strings.Contains(string(raw2), "Users never maintain the vault") {
		t.Fatalf("update section failed:\n%s", raw2)
	}
}

func TestEndOfImprintSection(t *testing.T) {
	text := "## imprint memory\n\nfoo\n\n### Must do\n\n1. x\n\n## Other\n\nbar"
	end := endOfImprintSection(text, len("## imprint memory"))
	if !strings.HasPrefix(text[end:], "\n## Other") {
		t.Fatalf("end=%d text=%q", end, text[end:])
	}
}
