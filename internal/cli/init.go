package cli

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed templates/imprint-memory.mdc
var cursorRuleTemplate []byte

func (a *App) cmdInit(g globals, args []string) int {
	fs := newFlags()
	force := fs.Bool("force", false)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("init"))
			return 0
		}
		return a.fail(g.json, err)
	}

	root, err := a.workdir()
	if err != nil {
		return a.fail(g.json, err)
	}
	rulePath := filepath.Join(root, ".cursor", "rules", "imprint-memory.mdc")
	if err := writeCursorRule(rulePath, *force); err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{
			"written": []string{rulePath},
		}))
	}
	fmt.Fprintf(a.out(), "wrote %s\n", rulePath)
	fmt.Fprintln(a.out(), "Keep imprint on PATH. Agents follow this alwaysApply rule and run imprint --json.")
	return 0
}

func (a *App) workdir() (string, error) {
	if a.Getwd != nil {
		return a.Getwd()
	}
	return os.Getwd()
}

func writeCursorRule(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("%s already exists; pass --force to overwrite", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := bytes.TrimSpace(cursorRuleTemplate)
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o644)
}
