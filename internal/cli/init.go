package cli

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed templates/body.md
var imprintBody []byte

//go:embed templates/imprint.yaml
var imprintYAML []byte

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
	written, err := writeInitProject(root, force, editors)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(map[string]any{
			"written": written,
		}))
	}
	c := a.console()
	c.Heading("init")
	for _, p := range written {
		c.Done("wrote %s", p)
	}
	c.Action("mount imprint MCP in your editor (optional)", "see docs/mcp.md and docs/examples/cursor-mcp.json")
	c.Action("start local services when you need desk or shelves", "imprint up")
	c.blank()
	return 0
}

func (a *App) workdir() (string, error) {
	if a.Getwd != nil {
		return a.Getwd()
	}
	return os.Getwd()
}
