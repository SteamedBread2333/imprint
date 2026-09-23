package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/telemetry"
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

const serverInstructions = `imprint stores durable user policy in vault.db; shelves adds document recall.

Users speak normally: never ask them for commands or rule ids. At most once per user message, call find with scope, query, and query_local prepared together; recall is reference-only and must not block code reads. Prefer rule claims over supplemental documents.

Before writing, classify ADD / REINFORCE / SUPERSEDE / IGNORE and reuse that find result. Ignore one-off work. add uses an English imperative claim plus user-verbatim text; omit confidence for 0.6, use 0.85 for corrections, never add at 0.9. reinforce only when the user explicitly repeats the same policy. supersede when policy changes; successor confidence decays the gap above 0.6 by imprint.yaml supersede.inheritance_alpha (default 0.20). Use sources only when the cited excerpt substantively supports the rule. Server-side privacy and duplicate gates may reject writes.

find/get are compact by default; request full or evidence only for audit. Results are JSON.`

// Run starts the MCP server on stdio using cfg for vault resolution.
func Run(ctx context.Context, cfg Config) error {
	dir, err := resolveVaultDir(cfg)
	if err != nil {
		return err
	}
	opts := imprint.OpenOptions{Dir: dir, Now: Now}
	if cfgPath := resolvePluginConfigPathForRun(cfg, dir); cfgPath != "" {
		pcfg, loadErr := plugin.Load(cfgPath)
		if loadErr != nil {
			return loadErr
		}
		ensureEmbedSidecar(ctx, pcfg)
		if pcfg.Telemetry.Enabled {
			opts.Telemetry = telemetry.New(
				filepath.Join(dir, imprint.StateDirName, "telemetry"),
				pcfg.Telemetry.RetentionDays, Now, os.Stderr,
			)
		}
		pcfg.ApplyToOpenOptions(&opts)
	}
	v, err := imprint.Open(opts)
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

// ensureEmbedSidecar starts the embed plugin if it is enabled and the sidecar
// port is not already healthy. Safe to call when the plugin is already
// running: the probe returns true and we no-op.
//
// When the port is bound by a process we do not own (e.g. another imprint-mcp
// instance, or `imprint plugin start embed` started earlier), we leave it
// alone: whoever owns port 4174 owns the model space (see
// docs/semantic-dedup.md "Single-instance constraint").
//
// Errors are logged and swallowed — the embed path always degrades silently
// to lexical Jaccard when the sidecar never comes up.
func ensureEmbedSidecar(ctx context.Context, pcfg *plugin.Config) {
	if pcfg == nil {
		return
	}
	entry, ok := pcfg.Plugins["embed"]
	if !ok || !entry.Enabled {
		return
	}
	cfg := plugin.EmbedConfigFrom(entry)
	if probeEmbedHealth(cfg.Port) {
		return
	}
	mgr := plugin.NewManager(pcfg)
	if err := mgr.StartPlugin(ctx, "embed"); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"imprint-mcp: embed sidecar not running on port %d and auto-start failed: %v\n"+
				"imprint-mcp: semantic gate will fall back to Jaccard\n"+
				"imprint-mcp: run `imprint plugin start embed` manually if the gate matters\n",
			cfg.Port, err,
		)
		return
	}
	fmt.Fprintf(os.Stderr, "imprint-mcp: embed sidecar started on port %d\n", cfg.Port)
}

// probeEmbedHealth pings GET http://127.0.0.1:<port>/health with a short
// timeout. Returns true only on a 2xx response.
func probeEmbedHealth(port int) bool {
	if port <= 0 {
		port = imprint.DefaultEmbedPort
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
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
