package cli

import (
	"context"
	"fmt"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
)

// embedPluginID is the plugin id whose lifecycle `imprint up` does not own.
// The embed sidecar is a singleton started by either `imprint-mcp` (the
// common case for MCP users) or `imprint plugin start embed` (manual /
// CLI-only users). `up` starts the host, shelves, and the rest of the
// plugin set — embedding is intentionally not in that set because:
//
//   - MCP users do not run `imprint up` at all; treating embed as part of
//     `up` would make MCP silently miss the sidecar.
//   - CLI users who want embed can opt in once with
//     `imprint plugin start embed`, after which the process stays up across
//     `up` / `down` cycles and is reaped by `down` only.
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
			// Surface the embed caveat so JSON consumers do not have to
			// chase the docs.
			"embed_notice": embedUpNotice(cfg),
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
		// pre-launch probe. Skip it here — it has its own notice row
		// below — so the operator is not confused.
		if st.ID == embedPluginID {
			continue
		}
		if st.ID == "desk" && st.Enabled {
			deskEnabled = true
		}
		printPluginBlock(c, st.ID, st.Name, st.Enabled, st.Healthy, st.URL, st.Error)
	}
	if notice := embedUpNotice(cfg); notice != "" {
		c.Row(embedPluginID, stateOff, "not started by up", notice)
	}
	printUpNextSteps(c, deskEnabled, rt.Health.Shelves.Indexed, true)
	c.blank()
	return 0
}

// embedUpNotice returns a one-line human hint when the embed plugin is
// configured but `up` deliberately did not start it. Returns "" when the
// embed entry is absent or disabled.
func embedUpNotice(cfg *plugin.Config) string {
	if cfg == nil {
		return ""
	}
	entry, ok := cfg.Plugins[embedPluginID]
	if !ok || !entry.Enabled {
		return ""
	}
	return "start with: imprint plugin start embed"
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
	stopped := configuredPluginIDs(cfg)
	mgr.StopAll()
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
	c.blank()
	return 0
}
