package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type initEditor string

const (
	editorCursor    initEditor = "cursor"
	editorClaude    initEditor = "claude"
	editorCodex     initEditor = "codex"
	editorTrae      initEditor = "trae"
	editorWorkbuddy initEditor = "workbuddy"
)

var allInitEditors = []initEditor{
	editorCursor,
	editorClaude,
	editorCodex,
	editorTrae,
	editorWorkbuddy,
}

func parseInitEditors(args []string) (force bool, editors []initEditor, err error) {
	fs := newFlags()
	forcePtr := fs.Bool("force", false)
	cursor := fs.Bool("cursor", false)
	claude := fs.Bool("claude", false)
	codex := fs.Bool("codex", false)
	trae := fs.Bool("trae", false)
	workbuddy := fs.Bool("workbuddy", false)
	pos, err := fs.parse(args)
	if err != nil {
		return false, nil, err
	}
	if len(pos) > 0 {
		return false, nil, fmt.Errorf("unknown init argument %q", pos[0])
	}
	force = *forcePtr
	if *cursor {
		editors = append(editors, editorCursor)
	}
	if *claude {
		editors = append(editors, editorClaude)
	}
	if *codex {
		editors = append(editors, editorCodex)
	}
	if *trae {
		editors = append(editors, editorTrae)
	}
	if *workbuddy {
		editors = append(editors, editorWorkbuddy)
	}
	if len(editors) == 0 {
		editors = append(editors, allInitEditors...)
	}
	return force, editors, nil
}

func writeInitEditors(root string, force bool, editors []initEditor) ([]string, error) {
	var written []string
	for _, ed := range editors {
		paths, err := writeInitEditor(root, ed, force)
		if err != nil {
			return written, err
		}
		written = append(written, paths...)
	}
	return written, nil
}

func writeInitEditor(root string, ed initEditor, force bool) ([]string, error) {
	switch ed {
	case editorCursor:
		p := filepath.Join(root, ".cursor", "rules", "imprint-memory.mdc")
		if err := writeInitFile(p, cursorRuleContent(), force); err != nil {
			return nil, err
		}
		return []string{p}, nil
	case editorClaude:
		p := filepath.Join(root, ".claude", "rules", "imprint-memory.md")
		if err := writeInitFile(p, claudeRuleContent(), force); err != nil {
			return nil, err
		}
		return []string{p}, nil
	case editorTrae:
		p := filepath.Join(root, ".trae", "rules", "imprint-memory.md")
		if err := writeInitFile(p, traeRuleContent(), force); err != nil {
			return nil, err
		}
		return []string{p}, nil
	case editorWorkbuddy:
		p := filepath.Join(root, ".codebuddy", "rules", "imprint-memory", "RULE.mdc")
		if err := writeInitFile(p, cursorRuleContent(), force); err != nil {
			return nil, err
		}
		return []string{p}, nil
	case editorCodex:
		p, err := writeCodexInit(root, force)
		if err != nil {
			return nil, err
		}
		return []string{p}, nil
	default:
		return nil, fmt.Errorf("unknown editor %q", ed)
	}
}

func cursorRuleContent() []byte {
	return joinFrontmatter(`---
description: Use imprint as long-term memory. Prefer imprint MCP tools; fall back to imprint --json CLI. Write only explicit user preferences.
alwaysApply: true
---
`)
}

func claudeRuleContent() []byte {
	// No paths frontmatter → loads every session (Claude Code project rules).
	return joinFrontmatter("---\n---\n")
}

func traeRuleContent() []byte {
	return joinFrontmatter(`---
alwaysApply: true
description: Use imprint as long-term memory. Prefer imprint MCP tools; fall back to imprint --json CLI.
---
`)
}

func joinFrontmatter(frontmatter string) []byte {
	var buf bytes.Buffer
	buf.WriteString(strings.TrimSpace(frontmatter))
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	buf.Write(imprintBody)
	if len(imprintBody) == 0 || imprintBody[len(imprintBody)-1] != '\n' {
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func writeInitFile(path string, body []byte, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists; pass --force to overwrite", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, bytes.TrimSpace(body), 0o644)
}
