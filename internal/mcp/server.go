package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// Env reads process environment variables.
var Env = os.Getenv

// Getwd returns the working directory.
var Getwd = os.Getwd

// Home returns the user home directory.
var Home = os.UserHomeDir

// Now returns the current time for vault operations.
var Now = time.Now

const serverInstructions = `imprint long-term memory tools. Before coding or style questions, call find with a narrow scope (tags are AND). Before every write, classify ADD / REINFORCE / SUPERSEDE / IGNORE; never duplicate. Record only what the user said. User says forget → forget or skip. Results are JSON like imprint --json.`

// Run starts the MCP server on stdio using cfg for vault resolution.
func Run(ctx context.Context, cfg Config) error {
	dir, err := imprint.ResolveDir(cfg.Vault, cfg.Global, Env, Getwd, Home)
	if err != nil {
		return err
	}
	v, err := imprint.OpenWithNow(dir, Now)
	if err != nil {
		return fmt.Errorf("open vault %s: %w", dir, err)
	}
	fmt.Fprintf(os.Stderr, "imprint-mcp: vault %s\n", dir)

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "imprint",
		Version: imprint.Version,
	}, &sdkmcp.ServerOptions{
		Instructions: serverInstructions,
	})
	registerTools(server, v)

	return server.Run(ctx, &sdkmcp.StdioTransport{})
}

// Usage returns help text for -h.
func Usage() string { return usageText }

// PrintVersion writes the version line to w.
func PrintVersion(w io.Writer) {
	fmt.Fprintf(w, "imprint-mcp %s\n", imprint.Version)
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid since %q (use YYYY-MM-DD or RFC3339)", s)
}
