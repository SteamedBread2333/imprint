package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
)

func (a *App) cmdPlugin(g globals, rest []string) int {
	if len(rest) == 0 {
		fmt.Fprint(a.out(), pluginUsage)
		return 2
	}
	sub := rest[0]
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	mgr := plugin.NewManager(cfg)
	switch sub {
	case "list":
		return a.pluginList(g, mgr)
	case "enable":
		return a.pluginSetEnabled(g, cfg, true, rest[1:])
	case "disable":
		return a.pluginSetEnabled(g, cfg, false, rest[1:])
	case "start":
		return a.pluginStart(g, mgr, rest[1:])
	case "stop":
		return a.pluginStop(g, mgr, rest[1:])
	default:
		fmt.Fprintf(a.errw(), "imprint plugin: unknown subcommand %q\n", sub)
		fmt.Fprint(a.errw(), pluginUsage)
		return 2
	}
}

const pluginUsage = `Usage:
  imprint plugin list
  imprint plugin enable ID
  imprint plugin disable ID
  imprint plugin start [ID...]   Start enabled external plugins (default: all)
  imprint plugin stop            Stop running plugins

Shelves is host config (shelves: in imprint.yaml) — use imprint up/down.
`

func (a *App) loadPluginConfig() (*plugin.Config, error) {
	path, err := plugin.ResolveConfigPath(a.Getwd)
	if err != nil {
		return nil, err
	}
	return plugin.Load(path)
}

func (a *App) pluginList(g globals, mgr *plugin.Manager) int {
	items, err := mgr.List()
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return boolExit(a.writeJSON(items))
	}
	cfg := mgr.Config()
	c := a.console()
	c.Heading("plugins")
	shelvesStatus := shelves.Status{Enabled: shelves.ConfigFrom(cfg).Enabled}
	hostUp := false
	if cfg != nil {
		if health, ok := fetchHostHealthFull(cfg.HostURL()); ok {
			shelvesStatus = health.Shelves
			hostUp = true
		}
	}
	printShelvesBlock(c, shelvesStatus, shelvesRoots(cfg), hostUp)
	for _, st := range items {
		printPluginBlock(c, st.ID, st.Name, st.Enabled, st.Healthy, st.URL, st.Error)
	}
	c.blank()
	return 0
}

func (a *App) pluginSetEnabled(g globals, cfg *plugin.Config, on bool, args []string) int {
	if len(args) == 0 {
		return a.fail(g.json, fmt.Errorf("plugin id required"))
	}
	id := strings.TrimSpace(args[0])
	if id == "shelves" {
		return a.fail(g.json, fmt.Errorf("shelves is host config — edit shelves.enabled in imprint.yaml, then run: imprint up"))
	}
	entry, ok := cfg.Plugins[id]
	if !ok {
		entry = plugin.PluginEntry{Config: map[string]any{}}
	}
	entry.Enabled = on
	cfg.Plugins[id] = entry
	if err := cfg.Save(); err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return boolExit(a.writeJSON(map[string]any{"id": id, "enabled": on}))
	}
	c := a.console()
	c.Heading("plugin " + map[bool]string{true: "enable", false: "disable"}[on])
	c.KV("id", id)
	c.Action("apply the config change", "imprint up")
	c.blank()
	return 0
}

func (a *App) pluginStart(g globals, mgr *plugin.Manager, args []string) int {
	cfg := mgr.Config()
	if cfg == nil {
		return a.fail(g.json, fmt.Errorf("plugin config missing"))
	}
	if len(args) > 0 {
		for _, id := range args {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if id == "shelves" {
				return a.fail(g.json, fmt.Errorf("shelves is host config — run: imprint up"))
			}
			entry, ok := cfg.Plugins[id]
			if !ok || !entry.Enabled {
				return a.fail(g.json, fmt.Errorf("plugin %q is not enabled in imprint.yaml", id))
			}
			if err := mgr.StartPlugin(context.Background(), id); err != nil {
				return a.fail(g.json, err)
			}
		}
	} else {
		if _, err := mgr.Reload(context.Background()); err != nil {
			return a.fail(g.json, err)
		}
	}
	items, err := mgr.List()
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return boolExit(a.writeJSON(items))
	}
	c := a.console()
	c.Heading("plugin start")
	for _, st := range items {
		if !st.Enabled {
			continue
		}
		if len(args) > 0 && !containsID(args, st.ID) {
			continue
		}
		printPluginBlock(c, st.ID, st.Name, st.Enabled, st.Healthy, st.URL, st.Error)
	}
	c.blank()
	return 0
}

func (a *App) pluginStop(g globals, mgr *plugin.Manager, args []string) int {
	if len(args) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint plugin stop\n")
		return 2
	}
	mgr.StopAll()
	if g.json {
		return boolExit(a.writeJSON(map[string]any{"stopped": configuredPluginIDs(mgr.Config())}))
	}
	c := a.console()
	c.Heading("plugin stop")
	for _, id := range configuredPluginIDs(mgr.Config()) {
		c.Row(id, stateOff, "stopped", "")
	}
	c.blank()
	return 0
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if strings.TrimSpace(id) == want {
			return true
		}
	}
	return false
}

func (a *App) cmdDesk(g globals, rest []string) int {
	if len(rest) == 0 || rest[0] != "open" {
		fmt.Fprint(a.out(), "Usage: imprint desk open\n")
		return 2
	}
	cfg, err := a.loadPluginConfig()
	if err != nil {
		return a.fail(g.json, err)
	}
	vaultDir, err := a.resolveHostVault(g, cfg)
	if err != nil {
		return a.fail(g.json, err)
	}
	hostURL := cfg.HostURL()
	if v, ok := fetchHostHealth(hostURL); !ok {
		return a.fail(g.json, fmt.Errorf("local stack is not running — run: imprint up"))
	} else if !vaultPathsEqual(v, vaultDir) {
		return a.fail(g.json, fmt.Errorf("host serves %s, not this project (%s)", v, vaultDir))
	}
	mgr := plugin.NewManager(cfg)
	url, err := mgr.DeskURL()
	if err != nil {
		return a.fail(g.json, fmt.Errorf("desk is not running — run: imprint up (%w)", err))
	}
	if g.json {
		return boolExit(a.writeJSON(map[string]string{"url": url}))
	}
	if err := openBrowser(url); err != nil {
		return a.fail(false, fmt.Errorf("open %s: %w", url, err))
	}
	c := a.console()
	c.Heading("desk open")
	c.Row("desk", stateOK, "running", url)
	c.blank()
	return 0
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func boolExit(err error) int {
	if err != nil {
		return 1
	}
	return 0
}
