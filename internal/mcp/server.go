package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
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

const serverInstructions = `imprint long-term memory (SQLite vault.db + shelves). MCP find/get include shelves (documents, links, resolved_sources, referenced_rules, chunk get). CLI find/get: vault rules only — for scripts.

Users never maintain the vault — speak normally; you run find/add/reinforce/supersede/forget. Never ask for imprint commands or rule ids. Review-and-prune: show/get or desk, explain, then supersede/forget/sweep after they agree. Before every write classify ADD / REINFORCE / SUPERSEDE / IGNORE; never duplicate; record only what the user said. User negates in plain speech → find then forget or supersede.

Before coding or style answers: find with narrow scope (tags AND) + query when shelves is on → { rules with resolved_sources, documents, links }. When local-language search terms differ from query, pass query_local alongside query (both run BM25 on vault and shelves; merged by max score). Same-turn add/supersede/reinforce may persist query_local on the rule. Link kinds in find.links (session-only except sources field): sources (vault rule→doc), cited_by ([[r-…]] in doc), co_search (same-query BM25; session-only).

Persistent links: Rule↔rule in vault — supersedes, related, conflicts_with; referenced_by on get. Rule→doc — sources on add/supersede [{path, heading?, chunk?}] in vault → resolved_sources on get/find. Doc→rule — referenced_rules (vault reverse scan) and optional cited_rules from markdown on get chunk id.

When find hits a matching document, same-turn add/supersede with sources. Results are JSON.`

// Run starts the MCP server on stdio using cfg for vault resolution.
func Run(ctx context.Context, cfg Config) error {
	dir, err := resolveVaultDir(cfg)
	if err != nil {
		return err
	}
	v, err := imprint.OpenWithNow(dir, Now)
	if err != nil {
		return fmt.Errorf("open vault %s: %w", dir, err)
	}
	fmt.Fprintf(os.Stderr, "imprint-mcp: version %s\n", imprint.Version)
	logProjectRoot(cfg)
	fmt.Fprintf(os.Stderr, "imprint-mcp: vault %s\n", v.Dir)

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "imprint",
		Version: imprint.Version,
	}, &sdkmcp.ServerOptions{
		Instructions: serverInstructions,
	})
	registerTools(server, v, loadShelvesForMCP(cfg, v.Dir))
	if cfgPath := resolvePluginConfigPathForRun(cfg, v.Dir); cfgPath != "" {
		registerPluginTools(server, cfgPath)
	}

	return server.Run(ctx, &sdkmcp.StdioTransport{})
}

// resolvePluginConfigPath pairs plugin config with the vault directory only.
// It never falls back to process cwd — that would bind the wrong imprint.yaml when
// Cursor spawns MCP with an unexpected working directory.
func resolvePluginConfigPath(vaultDir string) string {
	path, ok := plugin.ResolveConfigPathForVault(vaultDir)
	if !ok {
		return ""
	}
	return path
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

func loadShelvesForMCP(cfg Config, vaultDir string) *shelves.Service {
	cfgPath := resolvePluginConfigPathForRun(cfg, vaultDir)
	if cfgPath == "" {
		return nil
	}
	pcfg, err := plugin.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "imprint-mcp: shelves config: %v\n", err)
		return nil
	}
	if err := pcfg.ApplyGlossary(); err != nil {
		fmt.Fprintf(os.Stderr, "imprint-mcp: glossary: %v\n", err)
	}
	svc, err := shelves.New(shelves.ConfigFrom(pcfg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "imprint-mcp: shelves: %v\n", err)
		return nil
	}
	st := svc.Status()
	fmt.Fprintf(os.Stderr, "imprint-mcp: shelves enabled=%v chunks=%d\n", st.Enabled, st.ChunkCount)
	return svc
}
