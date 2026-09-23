package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/SteamedBread2333/imprint/internal/embed"
	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

// embedPluginID is the plugin id whose lifecycle `imprint up` does not own.
// The embed sidecar is a singleton started by either `imprint-mcp` (the
// common case for MCP users) or `imprint plugin start embed` (manual /
// CLI-only users). `up` starts the host, shelves, and the rest of the
// plugin set — embedding is intentionally not in that set because:
//
//   - MCP users do not run `imprint up` at all; treating embed as part of
//     `up` would make MCP silently miss the sidecar.
//   - CLI users who want embed opt in with `imprint plugin start embed`.
//     The process stays up across `up` / `down`. Stop it with
//     `imprint plugin stop embed`.
//
// See docs/semantic-dedup.md "Single-instance constraint" for the rationale
// behind the singleton port.
const embedPluginID = "embed"

func shelvesRoots(cfg *plugin.Config) []string {
	if cfg == nil {
		return shelves.DefaultRoots
	}
	return shelves.ConfigFrom(cfg).Roots
}

func (a *App) cmdUp(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint up\n")
		return 2
	}
	rt, err := a.hostStart(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	cfg, _, _ := a.projectConfig(g)
	mgr := plugin.NewManager(cfg)
	// `up` no longer starts the embed sidecar. The MCP server owns embed
	// when it is running; for CLI-only setups the user opts in via
	// `imprint plugin start embed`. See embedPluginID for the rationale.
	items, err := mgr.StartEnabledExcept(context.Background(), []string{embedPluginID})
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		out := map[string]any{
			"host":    rt.URL,
			"vault":   rt.Vault,
			"shelves": rt.Health.Shelves,
			"plugins": items,
			// Embed's real state — JSON consumers should see whether the
			// sidecar is actually listening, not a static "did not start
			// it" string that suggests a bug when MCP or a previous run
			// has the port.
			"embed": embedLiveJSON(cfg),
		}
		return boolExit(a.writeJSON(out))
	}
	c := a.console()
	c.Heading("up")
	if cfg != nil {
		c.KV("project", cfg.Workspace())
	}
	c.KV("vault", rt.Vault)
	printHostBlock(c, true, rt.URL, fetchRuleCount(rt.URL))
	roots := shelvesRoots(cfg)
	printShelvesBlock(c, rt.Health.Shelves, roots, true)
	deskEnabled := false
	for _, st := range items {
		// embed is intentionally not in `items` because we used
		// StartEnabledExcept to omit it from the start loop, but the
		// subsequent List() still walks every plugin in cfg.Plugins,
		// and statusFor may surface a "no such manifest" error from the
		// pre-launch probe. Skip it here — it has its own status row
		// computed from the live port probe below — so the operator is
		// not confused by a stale "no manifest" error.
		if st.ID == embedPluginID {
			continue
		}
		if st.ID == "desk" && st.Enabled {
			deskEnabled = true
		}
		printPluginBlock(c, st.ID, st.Name, st.Enabled, st.Healthy, st.URL, st.Error)
	}
	// Show embed's real state (probe the port) instead of a static "not
	// started by up" notice — the sidecar may already be up because
	// `imprint-mcp` (in the common case) or a previous `plugin start`
	// started it. Showing "not started" when it actually IS running
	// suggests a bug and prompts users to start a second sidecar on
	// the same port. State reflects the truth; the start hint only
	// appears when nothing is listening.
	if id, st, badge, detail, show := embedLiveRow(cfg); show {
		c.Row(id, st, badge, detail)
	}
	printUpNextSteps(c, deskEnabled, rt.Health.Shelves.Indexed, true)
	c.blank()
	return 0
}

// embedLiveRow probes the configured embed port and returns the row data
// `up` should print. When the sidecar is healthy it reports the live URL
// and model (so a sidecar brought up by `imprint-mcp` or a leftover from
// an earlier session is honestly described as "running"). When nothing is
// listening it returns the start hint so CLI-only users have a clear next
// step. show is false when embed is absent or disabled in the config.
func embedLiveRow(cfg *plugin.Config) (id string, st lineState, badge, detail string, show bool) {
	if cfg == nil {
		return "", 0, "", "", false
	}
	entry, ok := cfg.Plugins[embedPluginID]
	if !ok || !entry.Enabled {
		return "", 0, "", "", false
	}
	port, healthy, model := embedProbe(cfg)
	if healthy {
		url := imprint.LocalURL(port)
		if model != "" {
			return embedPluginID, stateOK, "running", fmt.Sprintf("%s · %s", url, model), true
		}
		return embedPluginID, stateOK, "running", url, true
	}
	return embedPluginID, stateOff, "not started by up", "start with: imprint plugin start embed", true
}

// embedLiveJSON returns the same live state for JSON consumers.
func embedLiveJSON(cfg *plugin.Config) map[string]any {
	out := map[string]any{
		"configured": false,
		"running":    false,
	}
	if cfg == nil {
		return out
	}
	entry, ok := cfg.Plugins[embedPluginID]
	if !ok {
		return out
	}
	out["configured"] = entry.Enabled
	if !entry.Enabled {
		return out
	}
	port, healthy, model := embedProbe(cfg)
	out["port"] = port
	out["url"] = imprint.LocalURL(port)
	out["running"] = healthy
	if healthy && model != "" {
		out["model"] = model
	}
	if !healthy {
		out["start_hint"] = "imprint plugin start embed"
	}
	return out
}

// embedProbe returns the configured embed port, whether the sidecar is
// healthy on it, and the model it advertises. Centralised so the human
// row and the JSON payload stay in sync.
func embedProbe(cfg *plugin.Config) (port int, healthy bool, model string) {
	entry, _ := cfg.Plugins[embedPluginID]
	port = plugin.EmbedConfigFrom(entry).Port
	if port <= 0 {
		port = imprint.DefaultEmbedPort
	}
	client := embed.NewClient(port, "", 500*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	healthy, model, _ = client.Health(ctx)
	return port, healthy, model
}

func (a *App) cmdDown(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint down\n")
		return 2
	}
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	mgr := plugin.NewManager(cfg)
	// `down` does not own the embed sidecar — MCP / `imprint plugin start
	// embed` do. Stop it explicitly with `imprint plugin stop embed`
	// (listen-port sweep, same as start).
	stopped := configuredPluginIDsExcept(cfg, embedPluginID)
	mgr.StopAllExcept([]string{embedPluginID})
	listen, err := a.hostStop(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return boolExit(a.writeJSON(map[string]any{
			"host":    listen,
			"plugins": stopped,
		}))
	}
	c := a.console()
	c.Heading("down")
	c.Row("host", stateOff, "stopped", listen)
	for _, id := range stopped {
		c.Row(id, stateOff, "stopped", "")
	}
	c.Action("start again when needed", "imprint up")
	if notice := embedDownNotice(cfg); notice != "" {
		c.Row(embedPluginID, stateOff, "left running", notice)
	}
	c.blank()
	return 0
}

// embedDownNotice returns a one-line hint when embed is configured but
// `down` deliberately left it running. Returns "" when the embed entry is
// absent or disabled.
func embedDownNotice(cfg *plugin.Config) string {
	if cfg == nil {
		return ""
	}
	entry, ok := cfg.Plugins[embedPluginID]
	if !ok || !entry.Enabled {
		return ""
	}
	return "stop with: imprint plugin stop embed"
}
