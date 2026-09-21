package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/plugin"
	"github.com/SteamedBread2333/imprint/internal/shelves"
	"github.com/SteamedBread2333/imprint/internal/telemetry"
	"github.com/SteamedBread2333/imprint/pkg/imprint"
)

func (a *App) cmdHost(g globals, rest []string) int {
	if len(rest) == 0 {
		fmt.Fprint(a.out(), hostUsage)
		return 2
	}
	switch rest[0] {
	case "serve":
		return a.cmdHostServe(g, rest[1:])
	case "start":
		return a.cmdHostStart(g, rest[1:])
	case "stop":
		return a.cmdHostStop(g, rest[1:])
	default:
		fmt.Fprintf(a.errw(), "imprint host: unknown subcommand %q\n", rest[0])
		fmt.Fprint(a.errw(), hostUsage)
		return 2
	}
}

const hostUsage = `Usage:
  imprint host serve [--listen ADDR]   Foreground vault + shelves API
  imprint host start                   Background host for this project
  imprint host stop                    Free the host listen port

Prefer imprint up / imprint down for everyday use.
`

func (a *App) cmdHostServe(g globals, rest []string) int {
	fs := newFlags()
	listen := fs.String("listen", "")
	if _, err := fs.parse(rest); err != nil {
		return a.fail(g.json, err)
	}
	cfgPath, err := plugin.ResolveConfigPath(a.Getwd)
	if err != nil {
		return a.fail(g.json, err)
	}
	pcfg, err := plugin.Load(cfgPath)
	if err != nil {
		return a.fail(g.json, err)
	}
	if err := pcfg.ApplyGlossary(); err != nil {
		return a.fail(g.json, err)
	}
	vaultDir, err := a.resolveHostVault(g, pcfg)
	if err != nil {
		return a.fail(g.json, err)
	}
	opts := imprint.OpenOptions{Dir: vaultDir, Now: a.now}
	if pcfg.Telemetry.Enabled {
		opts.Telemetry = telemetry.New(
			filepath.Join(vaultDir, imprint.StateDirName, "telemetry"),
			pcfg.Telemetry.RetentionDays, a.now, a.errw(),
		)
	}
	v, err := imprint.Open(opts)
	if err != nil {
		return a.fail(g.json, err)
	}
	if strings.TrimSpace(pcfg.Vault) == "" {
		pcfg.Vault = v.Dir
	}
	addr := *listen
	if addr == "" {
		addr = pcfg.Host.Listen
	}
	shelvesSvc, err := shelves.New(shelves.ConfigFrom(pcfg))
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		_ = a.writeJSON(map[string]string{"listen": addr, "vault": v.Dir})
	} else {
		c := a.console()
		c.Heading("host serve")
		c.KV("listen", "http://"+addr)
		c.KV("vault", v.Dir)
		c.Action("stop the foreground server", "Ctrl+C")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := host.Serve(ctx, host.Config{Listen: addr, Vault: v, Shelves: shelvesSvc}); err != nil && ctx.Err() == nil {
		return a.fail(g.json, err)
	}
	return 0
}

func (a *App) cmdHostStart(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint host start\n")
		return 2
	}
	rt, err := a.hostStart(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		out := map[string]any{
			"host":    rt.URL,
			"vault":   rt.Vault,
			"shelves": rt.Health.Shelves,
		}
		return boolExit(a.writeJSON(out))
	}
	cfg, _, _ := a.projectConfig(g)
	c := a.console()
	c.Heading("host start")
	if cfg != nil {
		c.KV("project", cfg.Workspace())
	}
	c.KV("vault", rt.Vault)
	printHostBlock(c, true, rt.URL, fetchRuleCount(rt.URL))
	printShelvesBlock(c, rt.Health.Shelves, shelvesRoots(cfg), true)
	printUpNextSteps(c, deskPluginEnabled(cfg), rt.Health.Shelves.Indexed, true)
	c.Action("stop the background host", "imprint down")
	c.blank()
	return 0
}

func (a *App) cmdHostStop(g globals, rest []string) int {
	if len(rest) > 0 {
		fmt.Fprint(a.out(), "Usage: imprint host stop\n")
		return 2
	}
	listen, err := a.hostStop(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return boolExit(a.writeJSON(map[string]string{"listen": listen, "stopped": "true"}))
	}
	c := a.console()
	c.Heading("host stop")
	c.Row("host", stateOff, "stopped", listen)
	c.blank()
	return 0
}
