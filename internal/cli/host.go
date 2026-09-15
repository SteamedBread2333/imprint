package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/SteamedBread2333/imprint/internal/host"
	"github.com/SteamedBread2333/imprint/internal/plugin"
)

func (a *App) cmdHost(g globals, rest []string) int {
	if len(rest) == 0 || rest[0] != "serve" {
		fmt.Fprint(a.out(), "Usage: imprint host serve [--listen ADDR]\n")
		return 2
	}
	fs := newFlags()
	listen := fs.String("listen", "")
	if _, err := fs.parse(rest[1:]); err != nil {
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
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
	if pcfg.Vault == "" {
		pcfg.Vault = v.Dir
	}
	addr := *listen
	if addr == "" {
		addr = pcfg.Host.Listen
	}
	if g.json {
		_ = a.writeJSON(map[string]string{"listen": addr, "vault": v.Dir})
	} else {
		fmt.Fprintf(a.out(), "imprint host: listening on http://%s (vault %s)\n", addr, v.Dir)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := host.Serve(ctx, host.Config{Listen: addr, Vault: v}); err != nil && ctx.Err() == nil {
		return a.fail(g.json, err)
	}
	return 0
}
