package cli

import (
	"fmt"

	"github.com/SteamedBread2333/imprint/internal/vault/migrate"
)

func (a *App) cmdMigrateShards(g globals, args []string) int {
	fs := newFlags()
	dryRun := fs.Bool("dry-run", false)
	_, err := fs.parse(args)
	if err != nil {
		if err == errHelp {
			fmt.Fprint(a.out(), commandHelp("migrate-shards"))
			return 0
		}
		return a.fail(g.json, err)
	}
	v, err := a.openVault(g)
	if err != nil {
		return a.fail(g.json, err)
	}
	res, err := migrate.ImportShards(v, *dryRun)
	if err != nil {
		return a.fail(g.json, err)
	}
	if g.json {
		return a.fail(false, a.writeJSON(res))
	}
	c := a.console()
	c.Heading("migrate-shards")
	for _, sh := range res.Shards {
		c.KV(sh.Path, fmt.Sprintf("%d rules", sh.Rules))
	}
	c.Done("markdown %d  ·  kept %d  ·  total %d (active=%d dormant=%d superseded=%d)",
		res.FromMarkdown, res.KeptExisting, res.Total, res.Active, res.Dormant, res.Superseded)
	if res.DryRun {
		c.Note("dry-run: vault.db unchanged")
	} else if res.Imported {
		c.Note("imported into vault.db")
	}
	c.blank()
	return 0
}
