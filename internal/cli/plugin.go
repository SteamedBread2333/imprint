package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/SteamedBread2333/imprint/internal/plugin"
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
	case "reload":
		return a.pluginReload(g, mgr)
	default:
		fmt.Fprintf(a.errw(), "imprint plugin: unknown subcommand %q\n", sub)
		fmt.Fprint(a.errw(), pluginUsage)
		return 2
	}
}

const pluginUsage = `Usage:
  imprint plugin list
  imprint plugin enable desk|shelves
  imprint plugin disable desk|shelves
  imprint plugin reload
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
	for _, st := range items {
		state := "disabled"
		if st.Enabled {
			state = "enabled"
		}
		if st.Running {
			state += ", running"
		}
		if st.Healthy {
			state += ", healthy"
		}
		name := st.Name
		if name == "" {
			name = st.ID
		}
		line := fmt.Sprintf("%s (%s) — %s", st.ID, name, state)
		if st.URL != "" && st.Enabled {
			line += fmt.Sprintf(" @ %s", st.URL)
		}
		if st.Error != "" {
			line += " — " + st.Error
		}
		fmt.Fprintln(a.out(), line)
	}
	return 0
}

func (a *App) pluginSetEnabled(g globals, cfg *plugin.Config, on bool, args []string) int {
	if len(args) == 0 {
		return a.fail(g.json, fmt.Errorf("plugin id required"))
	}
	id := strings.TrimSpace(args[0])
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
	verb := "disabled"
	if on {
		verb = "enabled"
	}
	fmt.Fprintf(a.out(), "plugin %s %s (run imprint plugin reload)\n", id, verb)
	return 0
}

func (a *App) pluginReload(g globals, mgr *plugin.Manager) int {
	items, err := mgr.Reload(context.Background())
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return boolExit(a.writeJSON(items))
	}
	fmt.Fprintln(a.out(), "plugins reloaded")
	for _, st := range items {
		if !st.Enabled {
			continue
		}
		line := fmt.Sprintf("  %s: ", st.ID)
		if st.Healthy {
			line += "healthy"
		} else if st.Error != "" {
			line += st.Error
		} else {
			line += "not healthy"
		}
		fmt.Fprintln(a.out(), line)
	}
	return 0
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
		return a.fail(g.json, fmt.Errorf("host is not running — run: imprint up (starts host + desk together)"))
	} else if !vaultPathsEqual(v, vaultDir) {
		return a.fail(g.json, fmt.Errorf("host on %s serves %s, not this project (%s) — run: imprint up", hostURL, v, vaultDir))
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
		fmt.Fprintf(a.out(), "open %s\n", url)
		return a.fail(false, err)
	}
	fmt.Fprintf(a.out(), "opened %s\n", url)
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
