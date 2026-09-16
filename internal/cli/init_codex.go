package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const imprintAgentsSection = "## imprint memory"

// codexInstructionPath picks where Codex loads project guidance from the repo root.
// Codex uses at most one file per directory: non-empty AGENTS.override.md wins over AGENTS.md.
func codexInstructionPath(root string) string {
	override := filepath.Join(root, "AGENTS.override.md")
	if fileNonEmpty(override) {
		return override
	}
	return filepath.Join(root, "AGENTS.md")
}

func fileNonEmpty(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Size() > 0
}

// codexImprintSection is loaded by Codex as part of the whole AGENTS file.
// Must do / Must not use ### so the section boundary is the next peer ## heading.
func codexImprintSection() []byte {
	text := string(imprintBody)
	text = strings.TrimPrefix(text, "# imprint memory\n\n")
	text = strings.ReplaceAll(text, "\n## Must do", "\n### Must do")
	text = strings.ReplaceAll(text, "\n## Must not", "\n### Must not")
	var buf bytes.Buffer
	buf.WriteString(imprintAgentsSection)
	buf.WriteString("\n\n")
	buf.WriteString(strings.TrimSpace(text))
	buf.WriteByte('\n')
	return buf.Bytes()
}

func writeCodexInit(root string, force bool) (string, error) {
	path := codexInstructionPath(root)
	if err := mergeCodexImprintSection(path, force); err != nil {
		return "", err
	}
	return path, nil
}

func mergeCodexImprintSection(path string, force bool) error {
	section := codexImprintSection()
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return os.WriteFile(path, section, 0o644)
	}
	text := string(existing)
	if idx := strings.Index(text, imprintAgentsSection); idx >= 0 {
		end := endOfImprintSection(text, idx+len(imprintAgentsSection))
		var buf bytes.Buffer
		buf.WriteString(strings.TrimRight(text[:idx], " \t\n"))
		if buf.Len() > 0 {
			buf.WriteString("\n\n")
		}
		buf.Write(section)
		rest := strings.TrimLeft(text[end:], " \t\n")
		if rest != "" {
			buf.WriteString("\n\n")
			buf.WriteString(rest)
			if !strings.HasSuffix(rest, "\n") {
				buf.WriteByte('\n')
			}
		}
		return os.WriteFile(path, buf.Bytes(), 0o644)
	}
	if !force {
		return fmt.Errorf("%s already exists without a %q section; pass --force to append it", path, imprintAgentsSection)
	}
	var buf bytes.Buffer
	buf.WriteString(strings.TrimRight(text, " \t\n"))
	buf.WriteString("\n\n")
	buf.Write(section)
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// endOfImprintSection is the byte index where the imprint section ends (next peer ##, not ###).
func endOfImprintSection(text string, from int) int {
	if from < 0 || from > len(text) {
		return len(text)
	}
	rest := text[from:]
	for {
		i := strings.Index(rest, "\n## ")
		if i < 0 {
			return len(text)
		}
		lineEnd := strings.Index(rest[i+1:], "\n")
		if lineEnd < 0 {
			lineEnd = len(rest[i+1:])
		}
		line := rest[i+1 : i+1+lineEnd]
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			return from + i
		}
		rest = rest[i+1:]
	}
}
