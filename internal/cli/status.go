package cli

import (
	"fmt"

	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func (a *App) cmdStatus(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint status\n")
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

	hostPort := plugin.ParseListenPort(cfg.Host.Listen)
	if hostPort <= 0 {
		hostPort = plugin.ParseListenPort(imprint.DefaultHostListen)
	}
	hostURL := cfg.HostURL()

	report := statusReport{
		Project: cfg.Workspace(),
		Vault:   vaultDir,
		Host:    probeHost(hostPort, hostURL),
	}
	shelvesCfg := shelves.ConfigFrom(cfg)
	if health, ok := fetchHostHealthFull(hostURL); ok {
		report.Shelves = health.Shelves
	} else {
		report.Shelves.Enabled = shelvesCfg.Enabled
	}
	deskEnabled := false
	if desk, ok := cfg.Plugins["desk"]; ok && desk.Enabled {
		deskEnabled = true
		port := plugin.PluginPort(desk, imprint.DefaultDeskPort)
		report.Desk = probeDesk(port, hostURL)
	}
	if why, cmd := statusHints(report, deskEnabled); why != "" {
		report.Hint = why + " — " + cmd
	}

	if g.json {
		return boolExit(a.writeJSON(report))
	}

	c := a.console()
	c.Heading("status")
	c.KV("project", report.Project)
	c.KV("vault", report.Vault)
	if report.Host.Listening {
		printHostBlock(c, true, hostURL, report.Host.Rules)
	} else {
		printHostBlock(c, false, fmt.Sprintf(":%d", report.Host.Port), 0)
	}
	printShelvesBlock(c, report.Shelves, shelvesRoots(cfg), report.Host.Listening)
	if report.Desk.Port > 0 && report.Host.Listening {
		printDeskBlock(c, report.Desk.Port, report.Desk.Listening)
	}
	if why, cmd := statusHints(report, deskEnabled); why != "" {
		c.Action(why, cmd)
	}
	c.blank()
	return 0
}
