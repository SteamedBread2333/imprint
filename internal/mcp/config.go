package mcp

import (
	"fmt"
	"strings"
)

// Config holds imprint-mcp startup flags.
type Config struct {
	Project string
	Vault   string
	Global  bool
	Version bool
	Help    bool
}

// ParseArgs reads imprint-mcp flags (no subcommands).
func ParseArgs(args []string) (Config, error) {
	var c Config
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			c.Help = true
		case a == "--version" || a == "-V":
			c.Version = true
		case a == "--global":
			c.Global = true
		case a == "--project", a == "--root":
			if i+1 >= len(args) {
				return c, fmt.Errorf("%s needs a path", a)
			}
			c.Project = args[i+1]
			i++
		case strings.HasPrefix(a, "--project="):
			c.Project = strings.TrimPrefix(a, "--project=")
		case strings.HasPrefix(a, "--root="):
			c.Project = strings.TrimPrefix(a, "--root=")
		case a == "--vault":
			if i+1 >= len(args) {
				return c, fmt.Errorf("--vault needs a path")
			}
			c.Vault = args[i+1]
			i++
		case strings.HasPrefix(a, "--vault="):
			c.Vault = strings.TrimPrefix(a, "--vault=")
		default:
			return c, fmt.Errorf("unknown argument %q (imprint-mcp uses stdio only; pass --project, --vault, --global, or --help)", a)
		}
	}
	return c, nil
}

const usageText = `imprint-mcp — MCP server for an imprint vault (stdio transport)

Usage:
  imprint-mcp [--project PATH] [--vault PATH] [--global] [--version]

Project + vault (recommended for Cursor multi-root workspaces):
  --project PATH   repo root (parent of .imprint/); vault defaults to .imprint
  --vault PATH     vault directory (relative paths join under --project)
  IMPRINT_PROJECT  same as --project when the flag is omitted

Otherwise (same as imprint CLI):
  --global         ~/.imprint
  walk-up / IMPRINT_VAULT / ./.imprint under cwd

Logs go to stderr. Tool results are JSON (same shapes as imprint --json).

See docs/mcp.md for Cursor / Claude Desktop mount examples.
`
