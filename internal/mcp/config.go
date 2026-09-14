package mcp

import (
	"fmt"
	"strings"
)

// Config holds imprint-mcp startup flags.
type Config struct {
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
		case a == "--vault":
			if i+1 >= len(args) {
				return c, fmt.Errorf("--vault needs a path")
			}
			c.Vault = args[i+1]
			i++
		case strings.HasPrefix(a, "--vault="):
			c.Vault = strings.TrimPrefix(a, "--vault=")
		default:
			return c, fmt.Errorf("unknown argument %q (imprint-mcp uses stdio only; pass --vault, --global, or --help)", a)
		}
	}
	return c, nil
}

const usageText = `imprint-mcp — MCP server for an imprint vault (stdio transport)

Usage:
  imprint-mcp [--vault PATH] [--global] [--version]

Vault resolution (same as imprint CLI):
  --vault PATH   explicit directory
  --global       ~/.imprint
  IMPRINT_VAULT  environment override when no --vault/--global
  otherwise      walk up for memory/, else ./memory

Logs go to stderr. Tool results are JSON (same shapes as imprint --json).

See docs/mcp.md for Cursor / Claude Desktop mount examples.
`
