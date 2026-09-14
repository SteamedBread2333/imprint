package cli

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed templates/body.md
var imprintBody []byte

func (a *App) cmdInit(g globals, args []string) int {
	force, editors, err := parseInitEditors(args)
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
	written, err := writeInitEditors(root, force, editors)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{
			"written": written,
		}))
	}
	for _, p := range written {
		fmt.Fprintf(a.out(), "wrote %s\n", p)
	}
	fmt.Fprintln(a.out(), "Keep imprint on PATH. Agents follow these rules (prefer imprint MCP; fall back to imprint --json).")
	fmt.Fprintln(a.out(), "init does not write MCP config — see docs/mcp.md and docs/editors.md.")
	return 0
}

func (a *App) workdir() (string, error) {
	if a.Getwd != nil {
		return a.Getwd()
	}
	return os.Getwd()
}
