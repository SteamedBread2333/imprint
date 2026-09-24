package cli

import (
	"context"
	"fmt"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
)

// embedPluginID is omitted from up/down: MCP (or `imprint plugin start embed`)
// owns the sidecar. See docs/semantic-dedup.md.
const embedPluginID = "embed"

func shelvesRoots(cfg *plugin.Config) []string {
	if cfg == nil {
		return shelves.DefaultRoots
	}
	return shelves.ConfigFrom(cfg).Roots
}

func withoutPlugin(items []plugin.Status, id string) []plugin.Status {
	out := make([]plugin.Status, 0, len(items))
	for _, st := range items {
		if st.ID == id {
			continue
		}
		out = append(out, st)
	}
	return out
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
	items, err := mgr.StartEnabledExcept(context.Background(), []string{embedPluginID})
	if err != nil {
		return a.fail(g.json, err)
	}
	items = withoutPlugin(items, embedPluginID)
	if g.json {
		out := map[string]any{
			"host":    rt.URL,
			"vault":   rt.Vault,
			"shelves": rt.Health.Shelves,
			"plugins": items,
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
		if st.ID == "desk" && st.Enabled {
			deskEnabled = true
		}
		printPluginBlock(c, st.ID, st.Name, st.Enabled, st.Healthy, st.URL, st.Error)
	}
	printUpNextSteps(c, deskEnabled, rt.Health.Shelves.Indexed, true)
	c.blank()
	return 0
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
	c.blank()
	return 0
}
